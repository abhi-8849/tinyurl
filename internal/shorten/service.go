package shorten

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// CodeGenerator issues unique codes; idgen.Generator satisfies it.
type CodeGenerator interface {
	NextCode(ctx context.Context) (string, error)
}

// Clock supplies the current time, so expiry can be controlled where time matters.
type Clock interface {
	Now() time.Time
}

// CreateRequest is the input to Create.
// ExpiresAt is optional; the zero value means "use the default lifetime".
type CreateRequest struct {
	LongURL   string
	Alias     string // optional; replaces the generated code
	OwnerID   string // who may delete the link
	ExpiresAt time.Time
}

// Service creates short links.
type Service struct {
	repo       store.Repository
	gen        CodeGenerator
	clock      Clock
	ownHost    string
	defaultTTL time.Duration
}

// NewService returns a Service. ownHost is the host (with port, if any) this service is served from; URLs pointing at it are refused.
func NewService(repo store.Repository, gen CodeGenerator, clock Clock, ownHost string, defaultTTL time.Duration) *Service {
	return &Service{repo: repo, gen: gen, clock: clock, ownHost: ownHost, defaultTTL: defaultTTL}
}

// Create stores a new mapping under the alias, or under a generated code when there is none.
// It returns ErrInvalidURL, ErrInvalidAlias or ErrAliasTaken.
func (s *Service) Create(ctx context.Context, req CreateRequest) (store.Mapping, error) {
	if err := validateURL(req.LongURL, s.ownHost); err != nil {
		return store.Mapping{}, err
	}
	code, err := s.code(ctx, req.Alias)
	if err != nil {
		return store.Mapping{}, err
	}
	now := s.clock.Now()
	expires := req.ExpiresAt
	if expires.IsZero() {
		expires = now.Add(s.defaultTTL)
	}
	m := store.Mapping{Code: code, LongURL: req.LongURL, OwnerID: req.OwnerID, CreatedAt: now, ExpiresAt: expires}
	if err := s.repo.Insert(ctx, m); errors.Is(err, store.ErrConflict) {
		// The conditional insert is what decides uniqueness, so two requests racing for one alias cannot both win.
		return store.Mapping{}, ErrAliasTaken
	} else if err != nil {
		return store.Mapping{}, fmt.Errorf("create %s: %w", code, err)
	}
	return m, nil
}

// code returns the validated alias, which skips the generator, or the next generated code.
func (s *Service) code(ctx context.Context, alias string) (string, error) {
	if alias != "" {
		return alias, validateAlias(alias)
	}
	code, err := s.gen.NextCode(ctx)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	return code, nil
}

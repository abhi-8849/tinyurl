package shorten

import (
	"context"
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
type CreateRequest struct {
	LongURL string
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

// Create stores a new mapping under a generated code. It returns ErrInvalidURL for an unacceptable URL.
func (s *Service) Create(ctx context.Context, req CreateRequest) (store.Mapping, error) {
	if err := validateURL(req.LongURL, s.ownHost); err != nil {
		return store.Mapping{}, err
	}
	code, err := s.gen.NextCode(ctx)
	if err != nil {
		return store.Mapping{}, fmt.Errorf("create: %w", err)
	}
	now := s.clock.Now()
	m := store.Mapping{Code: code, LongURL: req.LongURL, CreatedAt: now, ExpiresAt: now.Add(s.defaultTTL)}
	if err := s.repo.Insert(ctx, m); err != nil {
		return store.Mapping{}, fmt.Errorf("create %s: %w", code, err)
	}
	return m, nil
}

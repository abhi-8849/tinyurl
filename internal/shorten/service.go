package shorten

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// Kinds of created link, as counted by the Observer.
const (
	KindGenerated = "generated"
	KindAlias     = "alias"
)

// CodeGenerator issues unique codes; idgen.Generator satisfies it.
type CodeGenerator interface {
	NextCode(ctx context.Context) (string, error)
}

// Clock supplies the current time, so expiry can be controlled where time matters.
type Clock interface {
	Now() time.Time
}

// Cache is the part of the cache that Service writes: new links go in, deleted links come out.
type Cache interface {
	Set(ctx context.Context, m store.Mapping) error
	Delete(ctx context.Context, code string) error
}

// Observer counts what Service does; metrics.Metrics satisfies it.
type Observer interface {
	Created(kind string)
	AliasConflict()
	Deleted()
	CacheError()
}

// Deps are the collaborators and settings of a Service. OwnHost is the host (with port, if any) this service is served from;
// URLs pointing at it are refused.
type Deps struct {
	Repo       store.Repository
	Gen        CodeGenerator
	Cache      Cache
	Clock      Clock
	Observer   Observer
	Log        *slog.Logger
	OwnHost    string
	DefaultTTL time.Duration
}

// CreateRequest is the input to Create. ExpiresAt is optional; the zero value means "use the default lifetime".
type CreateRequest struct {
	LongURL   string
	Alias     string // optional; replaces the generated code
	OwnerID   string // who may delete the link
	ExpiresAt time.Time
}

// Service creates and deletes short links.
type Service struct {
	d Deps
}

// NewService returns a Service using d.
func NewService(d Deps) *Service {
	return &Service{d: d}
}

// Create stores a new mapping under the alias, or under a generated code when there is none.
// It returns ErrInvalidURL, ErrInvalidAlias or ErrAliasTaken.
func (s *Service) Create(ctx context.Context, req CreateRequest) (store.Mapping, error) {
	if err := validateURL(req.LongURL, s.d.OwnHost); err != nil {
		return store.Mapping{}, err
	}
	code, err := s.code(ctx, req.Alias)
	if err != nil {
		return store.Mapping{}, err
	}
	now := s.d.Clock.Now()
	expires := req.ExpiresAt
	if expires.IsZero() {
		expires = now.Add(s.d.DefaultTTL)
	}
	m := store.Mapping{Code: code, LongURL: req.LongURL, OwnerID: req.OwnerID, CreatedAt: now, ExpiresAt: expires}
	if err := s.d.Repo.Insert(ctx, m); errors.Is(err, store.ErrConflict) {
		// The conditional insert is what decides uniqueness, so two requests racing for one alias cannot both win.
		s.d.Observer.AliasConflict()
		return store.Mapping{}, ErrAliasTaken
	} else if err != nil {
		return store.Mapping{}, fmt.Errorf("create %s: %w", code, err)
	}
	s.writeThrough(ctx, m)
	s.d.Observer.Created(kindOf(req.Alias))
	return m, nil
}

// writeThrough puts a new link in the cache so its first redirect is a hit. A cache failure is logged, never returned:
// the link is already stored, and a miss just falls back to the store.
func (s *Service) writeThrough(ctx context.Context, m store.Mapping) {
	if err := s.d.Cache.Set(ctx, m); err != nil {
		s.d.Observer.CacheError()
		s.d.Log.Warn("cache set failed", "code", m.Code, "error", err)
	}
}

func kindOf(alias string) string {
	if alias != "" {
		return KindAlias
	}
	return KindGenerated
}

// code returns the validated alias, which skips the generator, or the next generated code.
func (s *Service) code(ctx context.Context, alias string) (string, error) {
	if alias != "" {
		return alias, validateAlias(alias)
	}
	code, err := s.d.Gen.NextCode(ctx)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	return code, nil
}

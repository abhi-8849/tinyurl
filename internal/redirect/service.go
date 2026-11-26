package redirect

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// ErrNotFound means no link exists for the code.
var ErrNotFound = errors.New("redirect: code not found")

// ErrExpired means the link existed but its expiry time has passed.
var ErrExpired = errors.New("redirect: link expired")

// Clock supplies the current time, so expiry can be controlled where time matters.
type Clock interface {
	Now() time.Time
}

// Service resolves codes. It needs only the repository, never the id allocator, so redirect-only servers stay independent of it.
type Service struct {
	repo  store.Repository
	clock Clock
}

// NewService returns a Service reading from repo.
func NewService(repo store.Repository, clock Clock) *Service {
	return &Service{repo: repo, clock: clock}
}

// Resolve returns the long URL for code, or ErrNotFound or ErrExpired.
func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	m, err := s.repo.Get(ctx, code)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", code, err)
	}
	if !s.clock.Now().Before(m.ExpiresAt) {
		return "", ErrExpired
	}
	return m.LongURL, nil
}

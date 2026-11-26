package redirect

import (
	"context"
	"errors"
	"fmt"

	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// ErrNotFound means no link exists for the code.
var ErrNotFound = errors.New("redirect: code not found")

// Service resolves codes. It needs only the repository, never the id allocator, so redirect-only servers stay independent of it.
type Service struct {
	repo store.Repository
}

// NewService returns a Service reading from repo.
func NewService(repo store.Repository) *Service {
	return &Service{repo: repo}
}

// Resolve returns the long URL for code, or ErrNotFound.
func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	m, err := s.repo.Get(ctx, code)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", code, err)
	}
	return m.LongURL, nil
}

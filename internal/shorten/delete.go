package shorten

import (
	"context"
	"errors"
	"fmt"

	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

var (
	// ErrNotFound means no link exists for the code.
	ErrNotFound = errors.New("shorten: code not found")
	// ErrForbidden means the link belongs to someone else.
	ErrForbidden = errors.New("shorten: not the owner")
)

// Delete marks the owner's link as deleted. Deleting an already deleted link succeeds, so a retry is safe.
// The row stays, so the code can never be issued again.
func (s *Service) Delete(ctx context.Context, ownerID, code string) error {
	m, err := s.repo.Get(ctx, code)
	if errors.Is(err, store.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete %s: %w", code, err)
	}
	if m.OwnerID != ownerID {
		return ErrForbidden
	}
	if m.DeletedAt != nil {
		return nil
	}
	if err := s.repo.MarkDeleted(ctx, code, s.clock.Now()); err != nil && !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("delete %s: %w", code, err)
	}
	return nil
}

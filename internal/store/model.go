package store

import (
	"context"
	"errors"
	"time"
)

// ErrConflict means a mapping with this code already exists, so codes are never overwritten or reused.
var ErrConflict = errors.New("store: code already exists")

// ErrNotFound means no mapping exists for the code.
var ErrNotFound = errors.New("store: code not found")

// Mapping is one short link. Links are never updated after they are created.
type Mapping struct {
	Code      string
	LongURL   string
	OwnerID   string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Repository persists mappings. Implementations must make Insert fail with ErrConflict instead of overwriting,
// because that check is what keeps codes and aliases unique.
type Repository interface {
	Insert(ctx context.Context, m Mapping) error
	Get(ctx context.Context, code string) (Mapping, error)
}

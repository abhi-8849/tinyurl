package idgen

import (
	"context"
	"sync/atomic"
)

// Allocator hands out ranges of ids. NextRange returns N, and the caller then owns ids N+1 through N+size.
// Ranges from one allocator never overlap, so several servers can share one allocator without clashing.
type Allocator interface {
	NextRange(ctx context.Context, size uint64) (start uint64, err error)
}

// MemoryAllocator is an Allocator backed by a counter in this process. It serves a single instance and loses its position on restart.
type MemoryAllocator struct {
	counter atomic.Uint64
}

// NewMemoryAllocator returns an allocator whose first id is 1.
func NewMemoryAllocator() *MemoryAllocator {
	return &MemoryAllocator{}
}

// NextRange claims size ids and returns the counter value before the claim.
func (m *MemoryAllocator) NextRange(_ context.Context, size uint64) (uint64, error) {
	return m.counter.Add(size) - size, nil
}

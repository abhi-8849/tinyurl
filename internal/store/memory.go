package store

import (
	"context"
	"sync"
	"time"
)

// MemoryRepository is a Repository held in process memory. It serves a single instance and loses its data on restart.
type MemoryRepository struct {
	mu sync.RWMutex // guards m; readers share it, Insert takes it exclusively
	m  map[string]Mapping
}

// NewMemoryRepository returns an empty MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{m: make(map[string]Mapping)}
}

// Insert stores m, or returns ErrConflict if its code is taken.
func (r *MemoryRepository) Insert(_ context.Context, m Mapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[m.Code]; ok {
		return ErrConflict
	}
	r.m[m.Code] = m
	return nil
}

// Get returns the mapping for code, or ErrNotFound.
func (r *MemoryRepository) Get(_ context.Context, code string) (Mapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.m[code]
	if !ok {
		return Mapping{}, ErrNotFound
	}
	return m, nil
}

// MarkDeleted records the deletion time of code, or returns ErrNotFound.
func (r *MemoryRepository) MarkDeleted(_ context.Context, code string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.m[code]
	if !ok {
		return ErrNotFound
	}
	m.DeletedAt = &at
	r.m[code] = m
	return nil
}

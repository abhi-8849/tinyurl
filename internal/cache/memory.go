package cache

import (
	"context"
	"sync"
	"time"

	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// Clock supplies the current time, so entry lifetimes can be controlled where time matters.
type Clock interface {
	Now() time.Time
}

type entry struct {
	m     store.Mapping
	until time.Time
}

// MemoryCache is a Cache held in process memory, for a single instance.
// TODO: expired entries are only dropped when read; add a size limit or periodic sweep before long-running use.
type MemoryCache struct {
	clock Clock
	mu    sync.RWMutex // guards m; readers share it, Set and Delete take it exclusively
	m     map[string]entry
}

// NewMemoryCache returns an empty MemoryCache.
func NewMemoryCache(clock Clock) *MemoryCache {
	return &MemoryCache{clock: clock, m: make(map[string]entry)}
}

// Get returns the cached mapping, or ErrMiss if it is absent or its lifetime has passed.
func (c *MemoryCache) Get(_ context.Context, code string) (store.Mapping, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[code]
	if !ok || !c.clock.Now().Before(e.until) {
		return store.Mapping{}, ErrMiss
	}
	return e.m, nil
}

// Set caches m for TTL(m). A mapping that is already expired is not stored.
func (c *MemoryCache) Set(_ context.Context, m store.Mapping) error {
	now := c.clock.Now()
	ttl := TTL(m, now)
	if ttl <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[m.Code] = entry{m: m, until: now.Add(ttl)}
	return nil
}

// Delete removes code from the cache.
func (c *MemoryCache) Delete(_ context.Context, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, code)
	return nil
}

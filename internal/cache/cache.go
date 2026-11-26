package cache

import (
	"context"
	"errors"
	"time"

	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// ErrMiss means the cache holds no usable entry for the code. It is the normal way to say "ask the store", not a failure.
var ErrMiss = errors.New("cache: miss")

// maxTTL caps how long an entry may live, so a changed or deleted link cannot be served from a stale cache for long.
const maxTTL = 24 * time.Hour

// Cache holds mappings in front of the store. Set picks the lifetime itself; see TTL.
type Cache interface {
	Get(ctx context.Context, code string) (store.Mapping, error)
	Set(ctx context.Context, m store.Mapping) error
	Delete(ctx context.Context, code string) error
}

// TTL is how long m may stay cached: the time until it expires, at most 24 hours. Zero or negative means do not cache it.
func TTL(m store.Mapping, now time.Time) time.Duration {
	return min(maxTTL, m.ExpiresAt.Sub(now))
}

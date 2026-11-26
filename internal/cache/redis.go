package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// redisTimeout bounds every Redis call, so a slow or unreachable Redis costs a redirect at most this much before it falls back to the store.
const redisTimeout = 250 * time.Millisecond

const keyPrefix = "link:"

// RedisCache is a Cache shared by every instance, so a link cached or deleted by one is seen by all.
type RedisCache struct {
	client *redis.Client
	clock  Clock
}

// NewRedisCache connects lazily to the Redis at addr; an unreachable Redis shows up as errors on use, not at startup.
func NewRedisCache(addr string, clock Clock) *RedisCache {
	// MaxRetries -1 turns off the client's own retries: the store is the fallback, and retrying a dead Redis would slow every redirect.
	client := redis.NewClient(&redis.Options{Addr: addr, DialTimeout: redisTimeout, ReadTimeout: redisTimeout, WriteTimeout: redisTimeout, MaxRetries: -1})
	return &RedisCache{client: client, clock: clock}
}

// Get returns the cached mapping, or ErrMiss if Redis has no entry.
func (c *RedisCache) Get(ctx context.Context, code string) (store.Mapping, error) {
	b, err := c.client.Get(ctx, keyPrefix+code).Bytes()
	if errors.Is(err, redis.Nil) {
		return store.Mapping{}, ErrMiss
	}
	if err != nil {
		return store.Mapping{}, fmt.Errorf("redis get %s: %w", code, err)
	}
	var m store.Mapping
	if err := json.Unmarshal(b, &m); err != nil {
		return store.Mapping{}, fmt.Errorf("decode %s: %w", code, err)
	}
	return m, nil
}

// Set caches m for TTL(m). A mapping that is already expired is not stored.
func (c *RedisCache) Set(ctx context.Context, m store.Mapping) error {
	ttl := TTL(m, c.clock.Now())
	if ttl <= 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode %s: %w", m.Code, err)
	}
	if err := c.client.Set(ctx, keyPrefix+m.Code, b, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %s: %w", m.Code, err)
	}
	return nil
}

// Delete removes code from Redis.
func (c *RedisCache) Delete(ctx context.Context, code string) error {
	if err := c.client.Del(ctx, keyPrefix+code).Err(); err != nil {
		return fmt.Errorf("redis del %s: %w", code, err)
	}
	return nil
}

// Close releases the connection pool.
func (c *RedisCache) Close() error {
	return c.client.Close()
}

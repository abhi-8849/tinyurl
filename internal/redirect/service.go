package redirect

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/abhi-8849/system-design/tinyurl/internal/cache"
	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

var (
	// ErrNotFound means no link exists for the code.
	ErrNotFound = errors.New("redirect: code not found")
	// ErrDeleted means the owner deleted the link.
	ErrDeleted = errors.New("redirect: link deleted")
	// ErrExpired means the link existed but its expiry time has passed.
	ErrExpired = errors.New("redirect: link expired")
)

// Results of a lookup, as counted by the Observer.
const (
	ResultCacheHit = "cache_hit"
	ResultStoreHit = "store_hit"
	ResultNotFound = "not_found"
	ResultExpired  = "expired"
	ResultDeleted  = "deleted"
)

// Clock supplies the current time, so expiry can be controlled where time matters.
type Clock interface {
	Now() time.Time
}

// Observer counts what Service does; metrics.Metrics satisfies it.
type Observer interface {
	Resolved(result string)
	CacheError()
}

// Deps are the collaborators of a Service.
type Deps struct {
	Repo     store.Repository
	Cache    cache.Cache
	Clock    Clock
	Observer Observer
	Log      *slog.Logger
}

// Service resolves codes. It needs only the repository and cache, never the id allocator, so redirect-only servers stay independent of it.
type Service struct {
	d     Deps
	group singleflight.Group // merges concurrent store reads for one code into a single read
}

// NewService returns a Service using d.
func NewService(d Deps) *Service {
	return &Service{d: d}
}

// Resolve returns the long URL for code, or ErrNotFound, ErrDeleted or ErrExpired.
func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	m, result, err := s.load(ctx, code)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.d.Observer.Resolved(ResultNotFound)
		return "", ErrNotFound
	case err != nil:
		return "", fmt.Errorf("resolve %s: %w", code, err)
	case m.DeletedAt != nil:
		s.d.Observer.Resolved(ResultDeleted)
		return "", ErrDeleted
	case !s.d.Clock.Now().Before(m.ExpiresAt):
		s.d.Observer.Resolved(ResultExpired)
		return "", ErrExpired
	}
	s.d.Observer.Resolved(result)
	return m.LongURL, nil
}

// load reads the cache first and the store on a miss. A cache error is logged and treated as a miss, so a broken cache never fails a redirect.
func (s *Service) load(ctx context.Context, code string) (store.Mapping, string, error) {
	m, err := s.d.Cache.Get(ctx, code)
	if err == nil {
		return m, ResultCacheHit, nil
	}
	if !errors.Is(err, cache.ErrMiss) {
		s.cacheFailed("get", code, err)
	}
	m, err = s.fromStore(ctx, code)
	return m, ResultStoreHit, err
}

// fromStore reads one code from the store and fills the cache. Concurrent callers for the same code share one read, so a popular
// code that just fell out of the cache does not stampede the store. The first caller's ctx is used for all of them. No goroutine is
// started here: singleflight runs the read in the calling goroutine and makes the others wait for its result.
func (s *Service) fromStore(ctx context.Context, code string) (store.Mapping, error) {
	v, err, _ := s.group.Do(code, func() (any, error) {
		m, err := s.d.Repo.Get(ctx, code)
		if err != nil {
			return store.Mapping{}, err
		}
		if err := s.d.Cache.Set(ctx, m); err != nil {
			s.cacheFailed("set", code, err)
		}
		return m, nil
	})
	if err != nil {
		return store.Mapping{}, err
	}
	return v.(store.Mapping), nil
}

func (s *Service) cacheFailed(op, code string, err error) {
	s.d.Observer.CacheError()
	s.d.Log.Warn("cache "+op+" failed", "code", code, "error", err)
}

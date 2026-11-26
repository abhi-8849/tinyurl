// Command tinyurl runs the URL shortener: it parses flags, wires the services and shuts down gracefully.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhi-8849/system-design/tinyurl/internal/auth"
	"github.com/abhi-8849/system-design/tinyurl/internal/cache"
	"github.com/abhi-8849/system-design/tinyurl/internal/httpapi"
	"github.com/abhi-8849/system-design/tinyurl/internal/idgen"
	"github.com/abhi-8849/system-design/tinyurl/internal/metrics"
	"github.com/abhi-8849/system-design/tinyurl/internal/redirect"
	"github.com/abhi-8849/system-design/tinyurl/internal/shorten"
	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// shutdownTimeout is how long in-flight requests get to finish after SIGINT or SIGTERM.
const shutdownTimeout = 10 * time.Second

type config struct {
	addr       string
	baseURL    *url.URL
	defaultTTL time.Duration
	rangeSize  uint64
	store      string // memory or dynamodb
	endpoint   string // DynamoDB endpoint; empty means real AWS
	table      string
	counterTbl string
	createTbls bool
	redisAddr  string // empty means an in-memory cache
	apiKeys    string // from TINYURL_API_KEYS, "key:owner,..."; never logged
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	// ctx is cancelled by SIGINT or SIGTERM; run treats that as the request to stop accepting and drain.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], log); err != nil {
		log.Error("tinyurl stopped", "error", err)
		os.Exit(1)
	}
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("tinyurl", flag.ContinueOnError)
	var c config
	base := fs.String("base-url", "http://localhost:8080", "public address that short URLs start with")
	fs.StringVar(&c.addr, "addr", ":8080", "listen address")
	fs.DurationVar(&c.defaultTTL, "default-ttl", 43800*time.Hour, "lifetime of a link with no expiry of its own")
	fs.StringVar(&c.store, "store", "memory", "link storage: memory or dynamodb")
	fs.StringVar(&c.endpoint, "dynamodb-endpoint", "", "DynamoDB endpoint, set for DynamoDB Local")
	fs.StringVar(&c.table, "table", "url_mapping", "DynamoDB table for links")
	fs.StringVar(&c.counterTbl, "counter-table", "id_counter", "DynamoDB table for the id counter")
	fs.BoolVar(&c.createTbls, "create-tables", false, "create the DynamoDB table if it is missing")
	fs.StringVar(&c.redisAddr, "redis-addr", "", "Redis address for the cache; empty uses an in-memory cache")
	fs.Uint64Var(&c.rangeSize, "range-size", 1_000_000, "ids claimed from the allocator at a time")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	u, err := url.Parse(*base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return config{}, fmt.Errorf("invalid -base-url %q", *base)
	}
	if c.store != "memory" && c.store != "dynamodb" {
		return config{}, fmt.Errorf("invalid -store %q: want memory or dynamodb", c.store)
	}
	if c.rangeSize == 0 {
		return config{}, errors.New("-range-size must be above zero")
	}
	c.baseURL = u
	c.apiKeys = os.Getenv("TINYURL_API_KEYS")
	return c, nil
}

func newHandler(c config, repo store.Repository, alloc idgen.Allocator, cch cache.Cache, log *slog.Logger) (http.Handler, error) {
	keys, err := auth.NewAPIKeys(c.apiKeys)
	if err != nil {
		return nil, fmt.Errorf("TINYURL_API_KEYS: %w", err)
	}
	m := metrics.New()
	clock := systemClock{}
	alloc = idgen.WithLogging(idgen.WithObserver(alloc, m), log)
	creator := shorten.NewService(shorten.Deps{
		Repo: repo, Gen: idgen.NewGenerator(alloc, c.rangeSize), Cache: cch, Clock: clock, Observer: m, Log: log,
		OwnHost: c.baseURL.Host, DefaultTTL: c.defaultTTL,
	})
	resolver := redirect.NewService(redirect.Deps{Repo: repo, Cache: cch, Clock: clock, Observer: m, Log: log})
	return httpapi.NewServer(keys, creator, resolver, m, c.baseURL.String(), log).Handler(), nil
}

func run(ctx context.Context, args []string, log *slog.Logger) error {
	c, err := parseConfig(args)
	if err != nil {
		return err
	}
	repo, alloc, err := newBackend(ctx, c)
	if err != nil {
		return err
	}
	cch := newCache(c)
	if closer, ok := cch.(io.Closer); ok {
		defer closer.Close()
	}
	h, err := newHandler(c, repo, alloc, cch, log)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: c.addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}

	// Goroutine owner: run. It stops when srv.Shutdown (or a listen failure) makes ListenAndServe return. It is the only sender and
	// sends exactly once; run is the only receiver. The channel is never closed. Capacity 1 lets the send finish even if run has
	// already returned on a startup error.
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("tinyurl started", "addr", c.addr, "base_url", c.baseURL.String())

	select {
	case err := <-errc: // The server failed on its own, for example because the port is taken.
		return err
	case <-ctx.Done(): // SIGINT or SIGTERM: stop accepting and drain.
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	<-errc // Waits for the serving goroutine to exit; it reports http.ErrServerClosed, which is the normal result.
	log.Info("tinyurl stopped")
	return nil
}

// newBackend returns the repository and id allocator chosen by -store. With DynamoDB both tables are created first when -create-tables is set.
func newBackend(ctx context.Context, c config) (store.Repository, idgen.Allocator, error) {
	if c.store == "memory" {
		return store.NewMemoryRepository(), idgen.NewMemoryAllocator(), nil
	}
	client, err := store.NewDynamoDBClient(ctx, c.endpoint)
	if err != nil {
		return nil, nil, err
	}
	if c.createTbls {
		for _, t := range []struct{ name, key string }{{c.table, "code"}, {c.counterTbl, "name"}} {
			if err := store.EnsureTable(ctx, client, t.name, t.key); err != nil {
				return nil, nil, err
			}
		}
	}
	return store.NewDynamoDBRepository(client, c.table), idgen.NewDynamoDBAllocator(client, c.counterTbl), nil
}

// newCache returns Redis when -redis-addr is set, otherwise a cache local to this process.
func newCache(c config) cache.Cache {
	if c.redisAddr != "" {
		return cache.NewRedisCache(c.redisAddr, systemClock{})
	}
	return cache.NewMemoryCache(systemClock{})
}

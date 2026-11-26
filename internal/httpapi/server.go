package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/abhi-8849/system-design/tinyurl/internal/shorten"
	"github.com/abhi-8849/system-design/tinyurl/internal/store"
)

// Shortener creates links; shorten.Service satisfies it.
type Shortener interface {
	Create(ctx context.Context, req shorten.CreateRequest) (store.Mapping, error)
	Delete(ctx context.Context, ownerID, code string) error
}

// Authenticator names the owner of a request; auth.APIKeys satisfies it. It returns auth.ErrUnauthenticated for an unknown caller.
type Authenticator interface {
	Owner(r *http.Request) (string, error)
}

// Resolver looks up a code; redirect.Service satisfies it.
type Resolver interface {
	Resolve(ctx context.Context, code string) (string, error)
}

// Server maps HTTP requests onto the services and their errors onto status codes.
type Server struct {
	auth      Authenticator
	shortener Shortener
	resolver  Resolver
	baseURL   string
	log       *slog.Logger
}

// NewServer returns a Server. baseURL is the public address that short URLs in responses start with.
func NewServer(a Authenticator, s Shortener, r Resolver, baseURL string, log *slog.Logger) *Server {
	return &Server{auth: a, shortener: s, resolver: r, baseURL: strings.TrimRight(baseURL, "/"), log: log}
}

// Handler returns the routes. A GET pattern also answers HEAD, so `curl -I` works.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/urls", s.create)
	mux.HandleFunc("DELETE /api/v1/urls/{code}", s.delete)
	mux.HandleFunc("GET /{code}", s.redirect)
	return mux
}

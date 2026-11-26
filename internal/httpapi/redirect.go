package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/abhi-8849/system-design/tinyurl/internal/redirect"
)

func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() { s.metrics.RedirectDuration(time.Since(start)) }()
	url, err := s.resolver.Resolve(r.Context(), r.PathValue("code"))
	if errors.Is(err, redirect.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if errors.Is(err, redirect.ErrDeleted) {
		http.Error(w, "link deleted", http.StatusGone)
		return
	}
	if errors.Is(err, redirect.ErrExpired) {
		http.Error(w, "link expired", http.StatusGone)
		return
	}
	if err != nil {
		s.log.Error("resolve failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Always 302, never 301: a permanent redirect would be cached by browsers and bypass expiry and deletion.
	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusFound)
}

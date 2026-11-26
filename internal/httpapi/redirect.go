package httpapi

import (
	"errors"
	"net/http"

	"github.com/abhi-8849/system-design/tinyurl/internal/redirect"
)

func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	url, err := s.resolver.Resolve(r.Context(), r.PathValue("code"))
	if errors.Is(err, redirect.ErrNotFound) {
		http.NotFound(w, r)
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

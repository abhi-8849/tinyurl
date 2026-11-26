package httpapi

import (
	"errors"
	"net/http"

	"github.com/abhi-8849/system-design/tinyurl/internal/shorten"
)

func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	owner, err := s.auth.Owner(r)
	if err != nil {
		http.Error(w, "missing or unknown API key", http.StatusUnauthorized)
		return
	}
	err = s.shortener.Delete(r.Context(), owner, r.PathValue("code"))
	switch {
	case errors.Is(err, shorten.ErrNotFound):
		http.NotFound(w, r)
	case errors.Is(err, shorten.ErrForbidden):
		http.Error(w, "not the owner of this link", http.StatusForbidden)
	case err != nil:
		s.log.Error("delete failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

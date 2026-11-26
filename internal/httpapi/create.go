package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/abhi-8849/system-design/tinyurl/internal/shorten"
)

// maxBodyBytes bounds the request body; the URL itself is capped lower, in shorten.
const maxBodyBytes = 8 << 10

type createRequest struct {
	LongURL   string     `json:"long_url"`
	Alias     string     `json:"custom_alias"` // optional
	ExpiresAt *time.Time `json:"expires_at"`   // optional; RFC 3339
}

type createResponse struct {
	Code      string    `json:"code"`
	ShortURL  string    `json:"short_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	sr := shorten.CreateRequest{LongURL: req.LongURL, Alias: req.Alias}
	if req.ExpiresAt != nil {
		sr.ExpiresAt = *req.ExpiresAt
	}
	m, err := s.shortener.Create(r.Context(), sr)
	if errors.Is(err, shorten.ErrInvalidURL) || errors.Is(err, shorten.ErrInvalidAlias) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if errors.Is(err, shorten.ErrAliasTaken) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err != nil {
		s.log.Error("create failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createResponse{Code: m.Code, ShortURL: s.baseURL + "/" + m.Code, ExpiresAt: m.ExpiresAt}); err != nil {
		s.log.Error("write create response", "error", err)
	}
}

package httpapi

import (
	"net/http"
	"time"
)

// statusWriter remembers the status code so it can be logged after the handler has run.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// logRequests writes one line per request. It logs the route pattern, not the raw path, so codes do not flood the log fields,
// and it never reads headers, so API keys cannot leak into it.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		// ServeMux sets r.Pattern on this same request while routing, so it is known by now.
		s.log.Info("request", "method", r.Method, "route", r.Pattern, "status", sw.status, "duration_ms", float64(time.Since(start).Microseconds())/1000)
	})
}

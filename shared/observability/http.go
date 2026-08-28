package observability

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// AccessLog returns an HTTP middleware that ensures every request has a
// request ID (read from the X-Request-ID header when present, otherwise
// generated), exposes it to downstream handlers and responses, and logs an
// access record with method, path, request ID, status code, and duration.
//
// Health-check probes are completely skipped from logging to avoid noise 
// from frequent ECS/Render target group monitoring.
func AccessLog(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqID := r.Header.Get(Header)
			ctx, reqID := GetOrCreateWithValue(r.Context(), reqID)
			r = r.WithContext(ctx)

			ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			if ww.Header().Get(Header) == "" {
				ww.Header().Set(Header, reqID)
			}

			next.ServeHTTP(ww, r)

			// Completely skip logging for health check endpoints
			if r.URL.Path == "/v1/public/transactions/healthcheck" ||
			r.URL.Path == "/v1/public/clients/healthcheck" ||
			r.URL.Path == "/v1/public/admindashboard/healthcheck" {
				return
			}

			logger.Info().
				Str("request_id", reqID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.status).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("http request")
		})
	}
}

// statusWriter records the response status code while delegating to the
// wrapped ResponseWriter.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

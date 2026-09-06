package observability

import (
	"net/http"
	"strings"
)

// ParseAllowedOrigins splits a configured comma-separated CORS origin
// allowlist, discarding empty entries.
func ParseAllowedOrigins(raw string) []string {
	origins := []string{}
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

// CORS applies a configured origin allowlist to the wrapped handler.
//
// Only origins on the allowlist receive CORS headers (no wildcard, no blind
// reflection). Browser preflight OPTIONS requests are answered here — the
// grpc-gateway mux has no OPTIONS route and would otherwise terminate the
// preflight with 404/405 and no CORS headers. Preflights never reach the
// gateway, services, repositories, or payment providers.
//
// Requests without an Origin header (server-to-server, curl) pass through
// unchanged: CORS is a browser-only concern. Requests from non-allowlisted
// origins also pass through unchanged but receive no CORS headers, so the
// browser blocks the response from being read by that origin's JavaScript.
//
// No credentials are granted: the Dashboard has no session/auth architecture
// and none is introduced here.
func CORS(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")

				// Preflight: answer before the wrapped handler sees the
				// request. Methods cover every Dashboard call (GET reads,
				// POST /v1/public/deposits); the Dashboard sends only
				// Content-Type on its POSTs.
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

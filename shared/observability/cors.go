package observability

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"
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

// DefaultAllowedOrigins returns the documented baseline Dashboard origins:
// the deployed admin dashboard and the local development dashboard.
func DefaultAllowedOrigins() []string {
	return []string{"https://admindashboard.rvpay.xyz", "http://localhost:3000"}
}

// LocalDevOrigin is the local development Dashboard origin. It is always
// ensured in the effective allowlist so a stale deployment configuration
// (e.g. an env file written before localhost support was added) cannot
// silently break local Dashboard development.
const LocalDevOrigin = "http://localhost:3000"

// ResolveAllowedOrigins turns the configured raw value into the effective
// allowlist:
//   - the parsed configured entries are kept as-is (order preserved);
//   - LocalDevOrigin is appended when missing;
//   - an empty/missing configuration falls back to DefaultAllowedOrigins.
//
// This keeps the runtime working even when HTTP_CORS_ALLOWED_ORIGINS is set
// to an old or empty value; deployment-specific origins remain configurable.
func ResolveAllowedOrigins(raw string) []string {
	origins := ParseAllowedOrigins(raw)
	if len(origins) == 0 {
		// Empty or missing configuration: use the documented defaults so a
		// blank env var cannot silently disable CORS.
		return DefaultAllowedOrigins()
	}
	hasLocal := false
	for _, origin := range origins {
		if origin == LocalDevOrigin {
			hasLocal = true
			break
		}
	}
	if !hasLocal {
		origins = append(origins, LocalDevOrigin)
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
//
// Every request carrying an Origin header is logged (method, path, origin,
// allowed) so a running service can prove why a browser did or did not
// receive CORS headers. No credentials or request payload data are logged.
func CORS(logger zerolog.Logger, allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			_, originAllowed := allowed[origin]
			logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("origin", origin).
				Bool("allowed", originAllowed).
				Msg("cors middleware processed request")

			if originAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")

				// Preflight: answer before the wrapped handler sees the
				// request. Methods cover every Dashboard call (GET reads,
				// POST /v1/public/deposits). Headers must permit the
				// Authorization bearer header that authenticated Dashboard
				// requests carry, alongside Content-Type on its POSTs.
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

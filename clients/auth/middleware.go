package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/metadata"
)

// MetadataBearerKey is the gRPC metadata key carrying the raw bearer token.
// The grpc-gateway forwards the Authorization header as lowercase metadata.
const MetadataBearerKey = "grpcgateway-authorization"

// TokenFromContext extracts the raw bearer token from gRPC metadata (as
// forwarded by the gateway). It never logs the token.
func TokenFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, key := range []string{MetadataBearerKey, "authorization"} {
		if values := md.Get(key); len(values) > 0 {
			return bearerToken(values[0])
		}
	}
	return ""
}

// bearerToken parses an Authorization header value of the form
// "Bearer <token>" (case-insensitive scheme) and returns the token part.
func bearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// AdminRoute is one protected method+path pair. Exact matching keeps the
// payment-page and health endpoints public with zero ambiguity.
type AdminRoute struct {
	Method string
	Path   string
}

// isAdminRoute reports whether the request targets a protected admin route.
func isAdminRoute(routes []AdminRoute, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

// AdminAuthMiddleware returns HTTP middleware that protects the given routes
// with the access-token authorization flow. Public endpoints (payment page,
// InitiateDeposit, healthchecks, provider webhooks) are never intercepted.
// The Authorization header is never logged.
func AdminAuthMiddleware(
	validate func(ctx context.Context, accessToken string) (bool, error),
	routes []AdminRoute,
	logger zerolog.Logger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isAdminRoute(routes, r.Method, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="rvpay-admin"`)
				http.Error(w, `{"code":16,"message":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			valid, err := validate(r.Context(), token)
			if err != nil || !valid {
				logger.Warn().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Bool("validation_error", err != nil).
					Msg("admin authorization rejected")
				w.Header().Set("WWW-Authenticate", `Bearer realm="rvpay-admin"`)
				http.Error(w, `{"code":16,"message":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

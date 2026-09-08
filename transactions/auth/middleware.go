package auth

import (
	"context"
	"net/http"

	clientsauth "github.com/I-Frostbyte/rvpay-go/clients/auth"
	clientsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/clientsgrpc"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AdminRoute mirrors clients/auth.AdminRoute for Transactions-local route
// declarations.
type AdminRoute = clientsauth.AdminRoute

// NewValidator connects to the Clients service and returns a validate
// function that delegates access-token validation over the internal
// ValidateAccessToken RPC. The Clients service owns the users/access_tokens
// state; Transactions never duplicates it. The returned closer releases the
// gRPC connection at shutdown.
func NewValidator(clientsAddr string, logger zerolog.Logger) (func(ctx context.Context, accessToken string) (bool, error), func(), error) {
	conn, err := grpc.NewClient(clientsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	client := clientsgrpc.NewAuthServiceClient(conn)
	validate := func(ctx context.Context, accessToken string) (bool, error) {
		resp, err := client.ValidateAccessToken(ctx, &clientsgrpc.ValidateAccessTokenRequest{AccessToken: accessToken})
		if err != nil {
			logger.Error().Err(err).Msg("access-token validation RPC failed")
			return false, err
		}
		return resp.GetValid(), nil
	}
	return validate, func() { _ = conn.Close() }, nil
}

// AdminAuthMiddleware protects the given routes with delegated access-token
// validation, reusing the shared middleware semantics from clients/auth
// (Bearer scheme, 401 with WWW-Authenticate, Authorization header never
// logged).
func AdminAuthMiddleware(
	validate func(ctx context.Context, accessToken string) (bool, error),
	routes []AdminRoute,
	logger zerolog.Logger,
) func(http.Handler) http.Handler {
	return clientsauth.AdminAuthMiddleware(validate, routes, logger)
}

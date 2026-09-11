package ghlsync

import (
	"context"
	"strings"

	"github.com/MountainHubTech/rvpay-go/clients/oauth"
	clientsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/clientsgrpc"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Impl implements clientsgrpc.PaymentSyncServiceServer. It owns the
// gRPC-only PaymentSyncService surface used by the Transactions service's
// durable synchronization worker. GHL-specific HTTP details stay in the
// provider client layer; this adapter maps/validates the request and
// delegates token+provider work to the existing OAuth sync entry point.
type Impl struct {
	oauthService *oauth.Service
	logger       zerolog.Logger

	clientsgrpc.UnimplementedPaymentSyncServiceServer
}

// NewGHLSyncService creates the PaymentSyncService adapter. oauthService is
// the existing shared OAuth service (tokens, refresh, provider registry,
// payment provider client, structured logging).
func NewGHLSyncService(oauthService *oauth.Service, logger zerolog.Logger) *Impl {
	return &Impl{
		oauthService: oauthService,
		logger:       logger,
	}
}

// UpdateGhlOrderStatus receives a server-to-server synchronization request
// from the Transactions worker and delegates to the OAuth sync entry point.
// It is the only operation of this service; no HTTP binding is registered,
// so it is reachable on the internal gRPC listener only.
func (s *Impl) UpdateGhlOrderStatus(ctx context.Context, req *clientsgrpc.UpdateGhlOrderStatusRequest) (*clientsgrpc.UpdateGhlOrderStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "order status request is required")
	}

	locationID := strings.TrimSpace(req.GetLocationId())
	orderID := strings.TrimSpace(req.GetOrderId())
	orderStatus := strings.ToLower(strings.TrimSpace(req.GetStatus()))
	if locationID == "" || orderID == "" {
		return nil, status.Error(codes.InvalidArgument, "location_id and order_id are required")
	}

	if err := s.oauthService.SyncGhlOrderStatus(ctx, locationID, orderID, orderStatus, req.GetAmount()); err != nil {
		// Surface the outcome to the worker without leaking credentials or
		// tokens. A gRPC error here counts as one failed GHL attempt and is
		// re-queued/failed by the worker per the two-try rule; it must never
		// mutate the PawaPay-authoritative deposit status.
		s.logger.Error().Err(err).
			Str("location_id", locationID).
			Str("order_id", orderID).
			Str("status", orderStatus).
			Msg("server-side GHL order status update failed")
		return nil, err
	}

	return &clientsgrpc.UpdateGhlOrderStatusResponse{Success: true}, nil
}

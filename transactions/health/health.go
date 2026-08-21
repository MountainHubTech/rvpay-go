package health

import (
	"context"

	"github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/rs/zerolog"
)

type Impl struct {
	logger zerolog.Logger

	transactionsgrpc.UnimplementedHealthServiceServer
}

func NewHealthService(
	logger zerolog.Logger,
) *Impl {
	return &Impl{
		logger: logger,
	}
}

func (h *Impl) HealthCheck(ctx context.Context, req *transactionsgrpc.HealthCheckRequest) (*transactionsgrpc.HealthCheckResponse, error) {
	return &transactionsgrpc.HealthCheckResponse{
		Status: "ok",
	}, nil
}
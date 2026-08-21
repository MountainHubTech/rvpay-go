package health

import (
	"context"

	"github.com/I-Frostbyte/rvpay-go/grpc/go/clientsgrpc"
	"github.com/rs/zerolog"
)

type Impl struct {
	logger       zerolog.Logger

	clientsgrpc.UnimplementedHealthServiceServer
}

func NewHealthService(
	logger zerolog.Logger,
) *Impl {
	return &Impl{
		logger: logger,
	}
}

func (h *Impl) HealthCheck(ctx context.Context, req *clientsgrpc.HealthCheckRequest) (*clientsgrpc.HealthCheckResponse, error) {
	return &clientsgrpc.HealthCheckResponse{
		Status: "ok",
	}, nil
}
package overview

import (
	"context"
	"strings"
	"sync"

	clientsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/clientsgrpc"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// ClientResolver maps an RVPay client/sub-account primary key (clients.id,
// owned by the Clients service) to the canonical RVPay client name persisted
// in clients.client_name ("highlevel-<locationId>"). The deposits tenant
// boundary is exactly that name (deposits.client_name), so a client_id
// supplied to ListTransactions is resolved through this interface before the
// database filter runs. The Clients service stays the only owner of the
// clients table; Transactions never queries it directly.
//
// It is satisfied by ClientsClientNameResolver in production; tests inject
// fakes.
type ClientResolver interface {
	ClientNameByID(ctx context.Context, clientID string) (string, error)
}

// ClientsClientNameResolver resolves client ids through the Clients service
// GetClient RPC — the documented cross-service validation path. It reuses the
// existing CLIENTS_GRPC_ADDR configuration (the same address the admin
// auth-token validator and the ghlsync worker already dial) and connects
// lazily on first use, mirroring ghlsync.Worker.connect. The connection lives
// for the process lifetime, exactly like the ghlsync worker's connection.
type ClientsClientNameResolver struct {
	clientsAddr string
	logger      zerolog.Logger

	mu     sync.Mutex
	client clientsgrpc.ClientsServiceClient
}

// NewClientsClientNameResolver creates a resolver that calls the Clients
// service at clientsAddr (CLIENTS_GRPC_ADDR configuration; never hard-coded).
func NewClientsClientNameResolver(clientsAddr string, logger zerolog.Logger) *ClientsClientNameResolver {
	return &ClientsClientNameResolver{
		clientsAddr: clientsAddr,
		logger:      logger,
	}
}

// connect lazily dials the Clients service and builds the GetClient client.
func (r *ClientsClientNameResolver) connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		return nil
	}
	if r.clientsAddr == "" {
		return status.Error(codes.FailedPrecondition, "clients grpc address is not configured")
	}
	conn, err := grpc.NewClient(r.clientsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return status.Errorf(codes.Internal, "connect to clients service: %v", err)
	}
	r.client = clientsgrpc.NewClientsServiceClient(conn)
	return nil
}

// ClientNameByID returns the canonical RVPay client name for the given
// clients.id. It surfaces codes.NotFound when no such client exists and
// preserves the Clients service status codes otherwise. The returned name is
// never fabricated: an empty stored name is treated as not found so the
// caller can never filter on a fabricated client.
func (r *ClientsClientNameResolver) ClientNameByID(ctx context.Context, clientID string) (string, error) {
	if err := r.connect(); err != nil {
		return "", err
	}
	resp, err := r.client.GetClient(ctx, &clientsgrpc.GetClientRequest{Id: clientID})
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(resp.GetClient().GetName())
	if name == "" {
		return "", status.Error(codes.NotFound, "client not found")
	}
	return name, nil
}

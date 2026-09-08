package service

import (
	"context"
	"errors"
	"time"

	"github.com/MountainHubTech/rvpay-go/clients/db/repo"
	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	clientsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/clientsgrpc"
	"github.com/MountainHubTech/rvpay-go/shared/observability"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ClientsServiceImpl struct {
	clientsRepo repo.ClientRepo
	logger      zerolog.Logger
	clientsgrpc.UnimplementedClientsServiceServer
}

func NewClientsServiceImpl(clientsRepo repo.ClientRepo, logger zerolog.Logger) *ClientsServiceImpl {
	return &ClientsServiceImpl{
		clientsRepo: clientsRepo,
		logger:      logger,
	}
}

func (s *ClientsServiceImpl) CreateClient(ctx context.Context, req *clientsgrpc.CreateClientRequest) (*clientsgrpc.CreateClientResponse, error) {
	if req == nil || req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "client name is required")
	}

	_, err := s.clientsRepo.GetByName(ctx, req.GetName())
	if err == nil {
		return nil, ErrClientAlreadyExists
	}
	if !errors.Is(err, repo.ErrNotFound) {
		return nil, translateRepoError(err)
	}

	client, err := s.clientsRepo.Create(ctx, req.GetName(), sqlc.ClientStatusREGISTERED)
	if err != nil {
		return nil, translateRepoError(err)
	}

	s.logger.Info().Str("client_id", client.ID.String()).Str("name", client.ClientName).Msg("client created")

	return &clientsgrpc.CreateClientResponse{
		Client: sqlcClientToProto(client),
	}, nil
}

func (s *ClientsServiceImpl) GetClient(ctx context.Context, req *clientsgrpc.GetClientRequest) (*clientsgrpc.GetClientResponse, error) {
	id, err := parseUUID(req.GetId())
	if err != nil {
		return nil, err
	}

	client, err := s.clientsRepo.GetByID(ctx, id)
	if err == repo.ErrNotFound {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, translateRepoError(err)
	}

	return &clientsgrpc.GetClientResponse{
		Client: sqlcClientToProto(client),
	}, nil
}

func (s *ClientsServiceImpl) ListClients(ctx context.Context, req *clientsgrpc.ListClientsRequest) (*clientsgrpc.ListClientsResponse, error) {
	pageSize := int32(20)
	offset := int32(0)
	if req.GetPagination() != nil {
		if req.GetPagination().PageSize > 0 {
			pageSize = req.GetPagination().PageSize
		}
		if req.GetPagination().PageToken != "" {
			offset = pageSize
		}
	}

	clients, err := s.clientsRepo.List(ctx, pageSize, offset)
	if err != nil {
		return nil, translateRepoError(err)
	}

	protoClients := make([]*clientsgrpc.Client, 0, len(clients))
	for _, c := range clients {
		protoClients = append(protoClients, sqlcClientToProto(c))
	}

	return &clientsgrpc.ListClientsResponse{
		Clients: protoClients,
	}, nil
}

func (s *ClientsServiceImpl) UpdateClient(ctx context.Context, req *clientsgrpc.UpdateClientRequest) (*clientsgrpc.UpdateClientResponse, error) {
	if req == nil || req.GetId() == "" || req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "client id and name are required")
	}

	id, err := parseUUID(req.GetId())
	if err != nil {
		return nil, err
	}

	client, err := s.clientsRepo.GetByID(ctx, id)
	if err == repo.ErrNotFound {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, translateRepoError(err)
	}

	updated, err := s.clientsRepo.UpdateStatus(ctx, id, client.Status)
	if err != nil {
		return nil, translateRepoError(err)
	}

	s.logger.Info().Str("client_id", updated.ID.String()).Str("name", updated.ClientName).Msg("client updated")

	return &clientsgrpc.UpdateClientResponse{
		Client: sqlcClientToProto(updated),
	}, nil
}

func (s *ClientsServiceImpl) DeleteClient(ctx context.Context, req *clientsgrpc.DeleteClientRequest) (*clientsgrpc.DeleteClientResponse, error) {
	id, err := parseUUID(req.GetId())
	if err != nil {
		return nil, err
	}

	client, err := s.clientsRepo.GetByID(ctx, id)
	if err == repo.ErrNotFound {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, translateRepoError(err)
	}

	if client.Status == sqlc.ClientStatusACTIVE {
		return nil, ErrClientHasIntegrations
	}

	err = s.clientsRepo.Delete(ctx, id)
	if err != nil {
		return nil, translateRepoError(err)
	}

	s.logger.Info().Str("client_id", id.String()).Msg("client deleted")

	return &clientsgrpc.DeleteClientResponse{
		Id: id.String(),
	}, nil
}

func (s *ClientsServiceImpl) ActivateClient(ctx context.Context, req *clientsgrpc.ActivateClientRequest) (*clientsgrpc.ActivateClientResponse, error) {
	id, err := parseUUID(req.GetId())
	if err != nil {
		return nil, err
	}

	client, err := s.clientsRepo.GetByID(ctx, id)
	if err == repo.ErrNotFound {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, translateRepoError(err)
	}

	if client.Status == sqlc.ClientStatusACTIVE {
		return &clientsgrpc.ActivateClientResponse{
			Client: sqlcClientToProto(client),
		}, nil
	}

	updated, err := s.clientsRepo.UpdateStatus(ctx, id, sqlc.ClientStatusACTIVE)
	if err != nil {
		return nil, translateRepoError(err)
	}

	s.logger.Info().Str("client_id", updated.ID.String()).Msg("client activated")

	return &clientsgrpc.ActivateClientResponse{
		Client: sqlcClientToProto(updated),
	}, nil
}

func (s *ClientsServiceImpl) DeactivateClient(ctx context.Context, req *clientsgrpc.DeactivateClientRequest) (*clientsgrpc.DeactivateClientResponse, error) {
	id, err := parseUUID(req.GetId())
	if err != nil {
		return nil, err
	}

	client, err := s.clientsRepo.GetByID(ctx, id)
	if err == repo.ErrNotFound {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, translateRepoError(err)
	}

	if client.Status == sqlc.ClientStatusCLOSED {
		return &clientsgrpc.DeactivateClientResponse{
			Client: sqlcClientToProto(client),
		}, nil
	}

	updated, err := s.clientsRepo.UpdateStatus(ctx, id, sqlc.ClientStatusCLOSED)
	if err != nil {
		return nil, translateRepoError(err)
	}

	s.logger.Info().Str("client_id", updated.ID.String()).Msg("client deactivated")

	return &clientsgrpc.DeactivateClientResponse{
		Client: sqlcClientToProto(updated),
	}, nil
}

// ListSubAccounts lists client/sub-account records for the Admin Dashboard
// sub-accounts page. balance/last_payout_date/total_processed are not
// populated (they require Transactions-owned data); the dashboard renders
// placeholders for those cells. See dashboard-setup.md for the documented
// cross-service gaps.
func (s *ClientsServiceImpl) ListSubAccounts(ctx context.Context, req *clientsgrpc.ListSubAccountsRequest) (resp *clientsgrpc.ListSubAccountsResponse, err error) {
	start := time.Now()
	if req == nil {
		req = &clientsgrpc.ListSubAccountsRequest{}
	}
	s.logger.Info().
		Str("request_id", observability.RequestIDFromContext(ctx)).
		Str("endpoint", "/v1/public/clients/sub-accounts").
		Str("method", "GET").
		Str("operation", "ListSubAccounts").
		Str("search", req.GetSearch()).
		Str("status", req.GetStatus()).
		Str("sort", req.GetSort()).
		Str("order", req.GetOrder()).
		Int("page", int(req.GetPage())).
		Int("page_size", int(req.GetPageSize())).
		Msg("dashboard API request received")

	defer func() {
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("request_id", observability.RequestIDFromContext(ctx)).
				Str("endpoint", "/v1/public/clients/sub-accounts").
				Str("operation", "ListSubAccounts").
				Str("search", req.GetSearch()).
				Str("status", req.GetStatus()).
				Int("page", int(req.GetPage())).
				Str("grpc_code", status.Code(err).String()).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("dashboard API request failed")
			return
		}
		s.logger.Info().
			Str("request_id", observability.RequestIDFromContext(ctx)).
			Str("endpoint", "/v1/public/clients/sub-accounts").
			Str("operation", "ListSubAccounts").
			Str("grpc_code", "OK").
			Int("rows_returned", len(resp.GetRows())).
			Int64("total", resp.GetTotal()).
			Int("page", int(resp.GetPage())).
			Int("page_size", int(resp.GetPageSize())).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("dashboard API request completed")
	}()

	page := req.GetPage()
	if page < 1 {
		page = 1
	}
	pageSize := req.GetPageSize()
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	rows, err := s.clientsRepo.ListSubAccounts(ctx, req.GetSearch(), req.GetStatus(), req.GetSort(), req.GetOrder(), pageSize, offset)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "ListSubAccounts").
			Str("repository", "ClientRepo.ListSubAccounts").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not list sub-accounts")
		return nil, translateRepoError(err)
	}
	if len(rows) == 0 {
		s.logger.Info().
			Str("operation", "ListSubAccounts").
			Str("repository", "ClientRepo.ListSubAccounts").
			Str("search", req.GetSearch()).
			Str("status", req.GetStatus()).
			Int("rows_returned", 0).
			Msg("dashboard sub-account query returned no rows")
	} else {
		s.logger.Debug().
			Str("operation", "ListSubAccounts").
			Str("repository", "ClientRepo.ListSubAccounts").
			Int("rows_returned", len(rows)).
			Msg("repository query completed")
	}
	total, err := s.clientsRepo.CountSubAccounts(ctx, req.GetSearch(), req.GetStatus())
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "ListSubAccounts").
			Str("repository", "ClientRepo.CountSubAccounts").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not count sub-accounts")
		return nil, translateRepoError(err)
	}

	protoRows := make([]*clientsgrpc.SubAccountRow, 0, len(rows))
	for _, row := range rows {
		protoRows = append(protoRows, subAccountRowToProto(row))
	}

	return &clientsgrpc.ListSubAccountsResponse{
		Rows:     protoRows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

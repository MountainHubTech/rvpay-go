package service

import (
	"context"
	"testing"

	"github.com/MountainHubTech/rvpay-go/clients/db/repo"
	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	clientsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/clientsgrpc"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockClientRepo is a test double for ClientRepo
type mockClientRepo struct {
	clients       map[string]sqlc.Client
	subAccounts   []sqlc.ListSubAccountsFilteredRow
	subAccountErr error
	total         int64
	countErr      error
	listStatus    string
	countStatus   string
}

func newMockClientRepo() *mockClientRepo {
	return &mockClientRepo{
		clients: make(map[string]sqlc.Client),
	}
}

func (m *mockClientRepo) Create(ctx context.Context, name string, status sqlc.ClientStatus) (sqlc.Client, error) {
	client := sqlc.Client{
		ID:         uuid.New(),
		ClientName: name,
		Status:     status,
	}
	m.clients[client.ID.String()] = client
	return client, nil
}

func (m *mockClientRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.Client, error) {
	client, ok := m.clients[id.String()]
	if !ok {
		return sqlc.Client{}, repo.ErrNotFound
	}
	return client, nil
}

func (m *mockClientRepo) GetByName(ctx context.Context, name string) (sqlc.Client, error) {
	for _, client := range m.clients {
		if client.ClientName == name {
			return client, nil
		}
	}
	return sqlc.Client{}, repo.ErrNotFound
}

func (m *mockClientRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.ClientStatus) (sqlc.Client, error) {
	client, ok := m.clients[id.String()]
	if !ok {
		return sqlc.Client{}, repo.ErrNotFound
	}
	client.Status = status
	m.clients[id.String()] = client
	return client, nil
}

// UpdateDisplayName mirrors the guarded SQL update: only a missing
// display name is filled; an existing name is never overwritten.
func (m *mockClientRepo) UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) (sqlc.Client, error) {
	client, ok := m.clients[id.String()]
	if !ok {
		return sqlc.Client{}, repo.ErrNotFound
	}
	if client.DisplayName == "" {
		client.DisplayName = displayName
		m.clients[id.String()] = client
	}
	return client, nil
}

// ListNeedingDisplayName serves the client-name backfill CLI; these tests
// do not exercise it.
func (m *mockClientRepo) ListNeedingDisplayName(ctx context.Context, limit, offset int32) ([]sqlc.ListClientsNeedingDisplayNameRow, error) {
	return nil, nil
}
func (m *mockClientRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.clients[id.String()]; !ok {
		return repo.ErrNotFound
	}
	delete(m.clients, id.String())
	return nil
}
func (m *mockClientRepo) ListSubAccounts(ctx context.Context, search, status, sort, order string, limit, offset int32) ([]sqlc.ListSubAccountsFilteredRow, error) {
	m.listStatus = status
	return m.subAccounts, m.subAccountErr
}

func (m *mockClientRepo) CountSubAccounts(ctx context.Context, search, status string) (int64, error) {
	m.countStatus = status
	return m.total, m.countErr
}
func (m *mockClientRepo) List(ctx context.Context, limit, offset int32) ([]sqlc.Client, error) {
	clients := make([]sqlc.Client, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	return clients, nil
}

func (m *mockClientRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.clients)), nil
}

func (m *mockClientRepo) ExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	_, ok := m.clients[id.String()]
	return ok, nil
}

func (m *mockClientRepo) ListActive(ctx context.Context, limit, offset int32) ([]sqlc.Client, error) {
	clients := make([]sqlc.Client, 0)
	for _, client := range m.clients {
		if client.Status == sqlc.ClientStatusACTIVE {
			clients = append(clients, client)
		}
	}
	return clients, nil
}

func TestCreateClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *clientsgrpc.CreateClientRequest
		code codes.Code
	}{
		{name: "missing request", code: codes.InvalidArgument},
		{name: "empty name", req: &clientsgrpc.CreateClientRequest{Name: ""}, code: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clientRepo := newMockClientRepo()
			svc := NewClientsServiceImpl(clientRepo, zerolog.Nop())

			_, err := svc.CreateClient(context.Background(), tt.req)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("status code = %s, want %s", got, tt.code)
			}
		})
	}
}

func TestGetClient(t *testing.T) {
	t.Parallel()

	clientRepo := newMockClientRepo()
	service := NewClientsServiceImpl(clientRepo, zerolog.Nop())

	client, err := clientRepo.Create(context.Background(), "Test Client", sqlc.ClientStatusACTIVE)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	resp, err := service.GetClient(context.Background(), &clientsgrpc.GetClientRequest{
		Id: client.ID.String(),
	})
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	if resp.Client.Name != "Test Client" {
		t.Fatalf("client name = %s, want Test Client", resp.Client.Name)
	}
}

func TestDeleteClient(t *testing.T) {
	t.Parallel()

	clientRepo := newMockClientRepo()
	service := NewClientsServiceImpl(clientRepo, zerolog.Nop())

	client, err := clientRepo.Create(context.Background(), "Test Client", sqlc.ClientStatusCLOSED)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	_, err = service.DeleteClient(context.Background(), &clientsgrpc.DeleteClientRequest{
		Id: client.ID.String(),
	})
	if err != nil {
		t.Fatalf("DeleteClient failed: %v", err)
	}

	_, err = clientRepo.GetByID(context.Background(), client.ID)
	if err != repo.ErrNotFound {
		t.Fatal("deleted client should not be found")
	}
}

func TestListSubAccountsMapsDashboardStatusToDatabaseEnum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     string
		wantStatus string
	}{
		{name: "all", status: "", wantStatus: ""},
		{name: "active", status: clientsgrpc.SubAccountStatus_SUB_ACCOUNT_STATUS_ACTIVE.String(), wantStatus: "ACTIVE"},
		{name: "restricted", status: clientsgrpc.SubAccountStatus_SUB_ACCOUNT_STATUS_RESTRICTED.String(), wantStatus: "SUSPENDED"},
		{name: "inactive", status: clientsgrpc.SubAccountStatus_SUB_ACCOUNT_STATUS_INACTIVE.String(), wantStatus: "CLOSED"},
		{name: "unknown", status: "unexpected", wantStatus: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newMockClientRepo()
			svc := NewClientsServiceImpl(repo, zerolog.Nop())

			if _, err := svc.ListSubAccounts(context.Background(), &clientsgrpc.ListSubAccountsRequest{
				Status:   tt.status,
				Page:     1,
				PageSize: 20,
			}); err != nil {
				t.Fatalf("ListSubAccounts failed: %v", err)
			}
			if repo.listStatus != tt.wantStatus {
				t.Errorf("list status = %q, want %q", repo.listStatus, tt.wantStatus)
			}
			if repo.countStatus != tt.wantStatus {
				t.Errorf("count status = %q, want %q", repo.countStatus, tt.wantStatus)
			}
		})
	}
}

func TestListSubAccountsReturnsAllRowsWhenOneDisplayNameIsUnavailable(t *testing.T) {
	t.Parallel()
	repo := newMockClientRepo()
	repo.total = 3
	repo.subAccounts = []sqlc.ListSubAccountsFilteredRow{
		{ID: uuid.New(), ClientName: "highlevel-location-a", DisplayName: "Account A", Status: sqlc.ClientStatusACTIVE, ExternalAccountID: "location-a"},
		{ID: uuid.New(), ClientName: "highlevel-location-b", DisplayName: "", Status: sqlc.ClientStatusACTIVE, ExternalAccountID: "location-b"},
		{ID: uuid.New(), ClientName: "highlevel-location-c", DisplayName: "Account C", Status: sqlc.ClientStatusACTIVE, ExternalAccountID: "location-c"},
	}
	svc := NewClientsServiceImpl(repo, zerolog.Nop())

	resp, err := svc.ListSubAccounts(context.Background(), &clientsgrpc.ListSubAccountsRequest{
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListSubAccounts failed: %v", err)
	}
	if len(resp.GetRows()) != 3 {
		t.Fatalf("rows = %d, want 3", len(resp.GetRows()))
	}
	if got := resp.GetRows()[0].GetName(); got != "Account A" {
		t.Errorf("row A name = %q, want Account A", got)
	}
	if got := resp.GetRows()[1].GetName(); got != "highlevel-location-b" {
		t.Errorf("row B fallback = %q, want highlevel-location-b", got)
	}
	if got := resp.GetRows()[2].GetName(); got != "Account C" {
		t.Errorf("row C name = %q, want Account C", got)
	}
}

package overview

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	transactionsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/MountainHubTech/rvpay-go/transactions/db/repo/mocks"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeClientResolver resolves client ids from a fixed map. Unknown ids and
// the injected err surface unchanged, mirroring what ClientsClientNameResolver
// returns (Clients GetClient emits codes.NotFound for an unknown clients.id).
type fakeClientResolver struct {
	names map[string]string
	err   error
}

func (f *fakeClientResolver) ClientNameByID(_ context.Context, clientID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	name, ok := f.names[clientID]
	if !ok {
		return "", status.Error(codes.NotFound, "client not found")
	}
	return name, nil
}

// newDepositRow builds the minimal deposit row ListTransactions maps into a
// response row.
func newDepositRow(clientName string) sqlc.Deposit {
	return sqlc.Deposit{
		ID:          uuid.New(),
		ClientName:  clientName,
		Amount:      pgtype.Numeric{Int: big.NewInt(12345), Exp: -2, Valid: true},
		Currency:    "XAF",
		Status:      sqlc.DepositStatusCOMPLETED,
		Provider:    sqlc.PaymentProviderMTNMOMO,
		InitiatedAt: time.Now().UTC(),
	}
}

// newTestOverview wires an Impl with strict repository mocks: any unexpected
// repository call fails the test, which is what proves filtering decisions are
// made before the database layer.
func newTestOverview(t *testing.T, resolver ClientResolver) (*Impl, *mocks.MockDepositRepo, *mocks.MockCustomerRepo) {
	t.Helper()
	ctrl := gomock.NewController(t)
	depositRepo := mocks.NewMockDepositRepo(ctrl)
	customerRepo := mocks.NewMockCustomerRepo(ctrl)
	svc := NewOverviewService(depositRepo, nil, nil, customerRepo, resolver, zerolog.Nop())
	return svc, depositRepo, customerRepo
}

// TestListTransactionsResolvesClientID proves a RVPay client id is resolved
// to the canonical client name and that exact name reaches the repository as
// the server-side filter (both list and count).
func TestListTransactionsResolvesClientID(t *testing.T) {
	t.Parallel()

	clientID := uuid.New().String()
	row := newDepositRow("highlevel-loc-a")
	svc, depositRepo, customerRepo := newTestOverview(t, &fakeClientResolver{
		names: map[string]string{clientID: "highlevel-loc-a"},
	})

	depositRepo.EXPECT().
		ListFiltered(gomock.Any(), "", "", "highlevel-loc-a", int32(20), int32(0)).
		Return([]sqlc.Deposit{row}, nil)
	depositRepo.EXPECT().
		CountFiltered(gomock.Any(), "", "", "highlevel-loc-a").
		Return(int64(1), nil)
	customerRepo.EXPECT().
		GetNameByClientAndPhone(gomock.Any(), "highlevel-loc-a", gomock.Any()).
		Return(nil, nil)

	resp, err := svc.ListTransactions(context.Background(), &transactionsgrpc.ListTransactionsRequest{
		ClientId: clientID,
	})
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}
	if resp.GetTotal() != 1 {
		t.Errorf("total = %d, want 1", resp.GetTotal())
	}
	if got := resp.GetRows()[0].GetSubAccount(); got != "highlevel-loc-a" {
		t.Errorf("row sub_account = %q, want highlevel-loc-a", got)
	}
}

// TestListTransactionsMapsLocationID proves the GHL locationId maps through
// the canonical highlevel-<locationId> naming convention into the same
// server-side client-name filter, without any Clients lookup.
func TestListTransactionsMapsLocationID(t *testing.T) {
	t.Parallel()

	svc, depositRepo, _ := newTestOverview(t, nil)

	depositRepo.EXPECT().
		ListFiltered(gomock.Any(), "", "", "highlevel-loc-b", int32(20), int32(0)).
		Return(nil, nil)
	depositRepo.EXPECT().
		CountFiltered(gomock.Any(), "", "", "highlevel-loc-b").
		Return(int64(0), nil)

	if _, err := svc.ListTransactions(context.Background(), &transactionsgrpc.ListTransactionsRequest{
		LocationId: "loc-b",
	}); err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}
}

// TestListTransactionsTenantIsolationServerSide is the mandatory
// cross-sub-account isolation proof at the service layer: requesting Client A
// must pass exactly A's canonical name to the repository (both list and
// count) and must surface only A's rows — never B's or C's. The SQL-level
// counterpart lives in transactions/db/repo/deposit_repo_filtered_test.go.
func TestListTransactionsTenantIsolationServerSide(t *testing.T) {
	t.Parallel()

	clientA := uuid.New().String()
	clientB := uuid.New().String()
	clientC := uuid.New().String()
	resolver := &fakeClientResolver{names: map[string]string{
		clientA: "highlevel-loc-a",
		clientB: "highlevel-loc-b",
		clientC: "highlevel-loc-c",
	}}

	rowsForA := []sqlc.Deposit{newDepositRow("highlevel-loc-a"), newDepositRow("highlevel-loc-a")}

	svc, depositRepo, customerRepo := newTestOverview(t, resolver)

	// Strict expectation: only the A filter may reach the repository. A call
	// with B's or C's name (or no name) fails the test.
	depositRepo.EXPECT().
		ListFiltered(gomock.Any(), "", "", "highlevel-loc-a", int32(20), int32(0)).
		Return(rowsForA, nil)
	depositRepo.EXPECT().
		CountFiltered(gomock.Any(), "", "", "highlevel-loc-a").
		Return(int64(2), nil)
	customerRepo.EXPECT().
		GetNameByClientAndPhone(gomock.Any(), "highlevel-loc-a", gomock.Any()).
		Return(nil, nil).Times(2)

	resp, err := svc.ListTransactions(context.Background(), &transactionsgrpc.ListTransactionsRequest{ClientId: clientA})
	if err != nil {
		t.Fatalf("ListTransactions for client A failed: %v", err)
	}
	if resp.GetTotal() != 2 {
		t.Errorf("total = %d, want 2", resp.GetTotal())
	}
	for i, row := range resp.GetRows() {
		if row.GetSubAccount() != "highlevel-loc-a" {
			t.Errorf("row %d sub_account = %q, want only highlevel-loc-a rows (client B/C leaked)", i, row.GetSubAccount())
		}
	}
}

// TestListTransactionsConflictingIdentifiersRejected proves conflicting
// identifiers are rejected rather than silently resolved to one of them, and
// that the repository is never reached.
func TestListTransactionsConflictingIdentifiersRejected(t *testing.T) {
	t.Parallel()

	otherClient := uuid.New().String()

	tests := []struct {
		name string
		req  *transactionsgrpc.ListTransactionsRequest
	}{
		{
			name: "client_id disagrees with location_id",
			req: &transactionsgrpc.ListTransactionsRequest{
				ClientId:   otherClient,
				LocationId: "loc-b",
			},
		},
		{
			name: "sub_account disagrees with location_id",
			req: &transactionsgrpc.ListTransactionsRequest{
				SubAccount: "highlevel-loc-a",
				LocationId: "loc-b",
			},
		},
		{
			name: "sub_account disagrees with resolved client_id",
			req: &transactionsgrpc.ListTransactionsRequest{
				SubAccount: "highlevel-loc-a",
				ClientId:   otherClient,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The resolver maps the client id to a name that conflicts with
			// the other identifier in the request.
			resolver := &fakeClientResolver{names: map[string]string{
				otherClient: "highlevel-loc-zz",
			}}
			// No repository expectations: any repository call fails the test.
			svc, _, _ := newTestOverview(t, resolver)
			_, err := svc.ListTransactions(context.Background(), tt.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument (err=%v)", status.Code(err), err)
			}
		})
	}
}

// TestListTransactionsAgreeingIdentifiersAccepted proves identifiers that
// refer to the same client are accepted.
func TestListTransactionsAgreeingIdentifiersAccepted(t *testing.T) {
	t.Parallel()

	clientID := uuid.New().String()
	svc, depositRepo, _ := newTestOverview(t, &fakeClientResolver{
		names: map[string]string{clientID: "highlevel-loc-a"},
	})

	depositRepo.EXPECT().
		ListFiltered(gomock.Any(), "", "", "highlevel-loc-a", int32(20), int32(0)).
		Return(nil, nil)
	depositRepo.EXPECT().
		CountFiltered(gomock.Any(), "", "", "highlevel-loc-a").
		Return(int64(0), nil)

	if _, err := svc.ListTransactions(context.Background(), &transactionsgrpc.ListTransactionsRequest{
		ClientId:   clientID,
		LocationId: "loc-a",
		SubAccount: "highlevel-loc-a",
	}); err != nil {
		t.Fatalf("agreeing identifiers must be accepted: %v", err)
	}
}

// TestListTransactionsIdentifierErrors covers malformed, unknown, resolver
// failure and unconfigured-resolver outcomes. In every case the repository is
// never reached (no expectations set).
func TestListTransactionsIdentifierErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resolver ClientResolver
		req      *transactionsgrpc.ListTransactionsRequest
		want     codes.Code
	}{
		{
			name:     "malformed client_id",
			resolver: &fakeClientResolver{},
			req:      &transactionsgrpc.ListTransactionsRequest{ClientId: "not-a-uuid"},
			want:     codes.InvalidArgument,
		},
		{
			name:     "malformed location_id contains whitespace",
			resolver: &fakeClientResolver{},
			req:      &transactionsgrpc.ListTransactionsRequest{LocationId: "loc bad"},
			want:     codes.InvalidArgument,
		},
		{
			name:     "unknown client_id",
			resolver: &fakeClientResolver{names: map[string]string{}},
			req:      &transactionsgrpc.ListTransactionsRequest{ClientId: uuid.New().String()},
			want:     codes.NotFound,
		},
		{
			name:     "resolver outage",
			resolver: &fakeClientResolver{err: status.Error(codes.Unavailable, "clients unavailable")},
			req:      &transactionsgrpc.ListTransactionsRequest{ClientId: uuid.New().String()},
			want:     codes.Internal,
		},
		{
			name:     "resolver not configured fails closed",
			resolver: nil,
			req:      &transactionsgrpc.ListTransactionsRequest{ClientId: uuid.New().String()},
			want:     codes.FailedPrecondition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, _, _ := newTestOverview(t, tt.resolver)
			_, err := svc.ListTransactions(context.Background(), tt.req)
			if status.Code(err) != tt.want {
				t.Fatalf("code = %v, want %v (err=%v)", status.Code(err), tt.want, err)
			}
		})
	}
}

// TestListTransactionsLegacySubAccountPassThrough proves existing behavior
// is intact: without the new identifiers the raw sub_account value reaches
// the repository byte-for-byte (no trimming, no rewriting).
func TestListTransactionsLegacySubAccountPassThrough(t *testing.T) {
	t.Parallel()

	svc, depositRepo, _ := newTestOverview(t, nil)

	depositRepo.EXPECT().
		ListFiltered(gomock.Any(), "pay", "COMPLETED", " highlevel-legacy ", int32(20), int32(0)).
		Return(nil, nil)
	depositRepo.EXPECT().
		CountFiltered(gomock.Any(), "pay", "COMPLETED", " highlevel-legacy ").
		Return(int64(0), nil)

	if _, err := svc.ListTransactions(context.Background(), &transactionsgrpc.ListTransactionsRequest{
		Search:     "pay",
		Status:     "COMPLETED",
		SubAccount: " highlevel-legacy ",
	}); err != nil {
		t.Fatalf("legacy path failed: %v", err)
	}
}

// TestListTransactionsPagination covers defaults, explicit pages and the
// page-size cap — the resolved identifier must not disturb pagination.
func TestListTransactionsPagination(t *testing.T) {
	t.Parallel()

	locationID := "loc-a"
	tests := []struct {
		name       string
		page       int32
		pageSize   int32
		wantPage   int32
		wantSize   int32
		wantOffset int32
	}{
		{name: "defaults", page: 0, pageSize: 0, wantPage: 1, wantSize: 20, wantOffset: 0},
		{name: "page 2 size 5", page: 2, pageSize: 5, wantPage: 2, wantSize: 5, wantOffset: 5},
		{name: "page 3 size 10", page: 3, pageSize: 10, wantPage: 3, wantSize: 10, wantOffset: 20},
		{name: "size capped at 100", page: 1, pageSize: 500, wantPage: 1, wantSize: 100, wantOffset: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, depositRepo, _ := newTestOverview(t, nil)
			depositRepo.EXPECT().
				ListFiltered(gomock.Any(), "", "", "highlevel-"+locationID, tt.wantSize, tt.wantOffset).
				Return(nil, nil)
			depositRepo.EXPECT().
				CountFiltered(gomock.Any(), "", "", "highlevel-"+locationID).
				Return(int64(0), nil)

			resp, err := svc.ListTransactions(context.Background(), &transactionsgrpc.ListTransactionsRequest{
				LocationId: locationID,
				Page:       tt.page,
				PageSize:   tt.pageSize,
			})
			if err != nil {
				t.Fatalf("ListTransactions failed: %v", err)
			}
			if resp.GetPage() != tt.wantPage || resp.GetPageSize() != tt.wantSize {
				t.Errorf("page/page_size = %d/%d, want %d/%d", resp.GetPage(), resp.GetPageSize(), tt.wantPage, tt.wantSize)
			}
		})
	}
}

// TestListTransactionsEmptyClientReturnsEmptyResult proves a client with no
// transactions gets an empty page (total 0), not an error.
func TestListTransactionsEmptyClientReturnsEmptyResult(t *testing.T) {
	t.Parallel()

	clientID := uuid.New().String()
	svc, depositRepo, _ := newTestOverview(t, &fakeClientResolver{
		names: map[string]string{clientID: "highlevel-new-client"},
	})

	depositRepo.EXPECT().ListFiltered(gomock.Any(), "", "", "highlevel-new-client", int32(20), int32(0)).Return(nil, nil)
	depositRepo.EXPECT().CountFiltered(gomock.Any(), "", "", "highlevel-new-client").Return(int64(0), nil)

	resp, err := svc.ListTransactions(context.Background(), &transactionsgrpc.ListTransactionsRequest{ClientId: clientID})
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}
	if len(resp.GetRows()) != 0 || resp.GetTotal() != 0 {
		t.Errorf("rows/total = %d/%d, want 0/0", len(resp.GetRows()), resp.GetTotal())
	}
}

// TestListTransactionsRepositoryError proves repository failures keep the
// existing Internal translation.
func TestListTransactionsRepositoryError(t *testing.T) {
	t.Parallel()

	svc, depositRepo, _ := newTestOverview(t, nil)
	depositRepo.EXPECT().
		ListFiltered(gomock.Any(), "", "", "", int32(20), int32(0)).
		Return(nil, errors.New("db down"))

	if _, err := svc.ListTransactions(context.Background(), &transactionsgrpc.ListTransactionsRequest{}); status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal (err=%v)", status.Code(err), err)
	}
}

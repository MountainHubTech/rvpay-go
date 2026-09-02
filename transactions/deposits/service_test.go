package deposits

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/I-Frostbyte/pawapay_client"
	commongrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/commongrpc"
	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo"
	repomocks "github.com/I-Frostbyte/rvpay-go/transactions/db/repo/mocks"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	sqlcmocks "github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc/mocks"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeTx is a minimal pgx.Tx recording Commit/Rollback calls so tests can
// assert the transaction boundary behaviour of InitiateDeposit.
type fakeTx struct {
	pgx.Tx
	committed  bool
	rolledBack bool
	commitErr  error
}

func (f *fakeTx) Commit(context.Context) error {
	if f.commitErr != nil {
		return f.commitErr
	}
	f.committed = true
	return nil
}

func (f *fakeTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

// beginTx wires a TransactionsRepo mock to a fake pgx.Tx and a mock Querier,
// mirroring the production Begin(ctx) -> (Querier, Tx) contract.
func beginTx(ctrl *gomock.Controller, tx *fakeTx) (*sqlcmocks.MockQuerier, *repomocks.MockTransactionsRepo) {
	txRepo := repomocks.NewMockTransactionsRepo(ctrl)
	querier := sqlcmocks.NewMockQuerier(ctrl)
	txRepo.EXPECT().Begin(gomock.Any()).Return(querier, tx, nil).AnyTimes()
	return querier, txRepo
}

func newTestService(depositRepo repo.DepositRepo, txRepo repo.TransactionsRepo, client pawapay_client.Client) *Impl {
	return NewDepositService(depositRepo, txRepo, zerolog.Nop(), client)
}

func validCreateRequest() *transactionsgrpc.CreateDepositRequest {
	return &transactionsgrpc.CreateDepositRequest{
		ClientName:       "highlevel-abc123",
		CustomerId:       "ghl-contact-123",
		MerchantId:       "+237654131027", // payer phone number (temporary mapping)
		GhlTransactionId: "6a981bf9111e4879c418ffee",
		Amount:           &commongrpc.Money{Amount: "1000.00", Currency: "XAF"},
		PaymentType:      commongrpc.PaymentType_PAYMENT_TYPE_MMO,
		PayerPhoneNumber: "+237600000000",
		Provider:         commongrpc.Provider_PROVIDER_MTN_MOMO,
	}
}

func TestInitiateDepositValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *transactionsgrpc.CreateDepositRequest
		code codes.Code
	}{
		{name: "missing request", code: codes.InvalidArgument},
		{name: "missing client name", req: &transactionsgrpc.CreateDepositRequest{}, code: codes.InvalidArgument},
		{name: "blank client name", req: &transactionsgrpc.CreateDepositRequest{ClientName: "   "}, code: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			service := newTestService(repomocks.NewMockDepositRepo(ctrl), repomocks.NewMockTransactionsRepo(ctrl), pawapay_client.Client{})

			_, err := service.InitiateDeposit(context.Background(), tt.req)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("status code = %s, want %s", got, tt.code)
			}
		})
	}
}

func TestInitiateDepositZeroAmount(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	service := newTestService(repomocks.NewMockDepositRepo(ctrl), repomocks.NewMockTransactionsRepo(ctrl), pawapay_client.Client{})

	req := validCreateRequest()
	req.Amount = &commongrpc.Money{Amount: "0", Currency: "XAF"}

	_, err := service.InitiateDeposit(context.Background(), req)
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s", got, codes.InvalidArgument)
	}
}

// TestInitiateDepositBeginFailure verifies that a failed transaction begin
// results in an Internal error with no database mutation attempted.
func TestInitiateDepositBeginFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	txRepo := repomocks.NewMockTransactionsRepo(ctrl)
	txRepo.EXPECT().Begin(gomock.Any()).Return(nil, nil, errors.New("pool exhausted"))

	service := newTestService(repomocks.NewMockDepositRepo(ctrl), txRepo, pawapay_client.Client{})

	_, err := service.InitiateDeposit(context.Background(), validCreateRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
}

// TestInitiateDepositExternalIdentifiersPersisted verifies that valid
// HighLevel-style external identifiers (NOT UUIDs) pass validation and are
// forwarded unchanged to persistence; a create failure rolls back.
func TestInitiateDepositExternalIdentifiersPersisted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg sqlc.CreateDepositParams) (sqlc.Deposit, error) {
			if arg.ClientName != "highlevel-abc123" {
				t.Errorf("client_name = %q, want %q", arg.ClientName, "highlevel-abc123")
			}
			if arg.CustomerID == nil || *arg.CustomerID != "ghl-contact-123" {
				t.Errorf("customer_id = %v, want exact external value", arg.CustomerID)
			}
			if arg.MerchantID == nil || *arg.MerchantID != "+237654131027" {
				t.Errorf("merchant_id = %v, want the payer phone number", arg.MerchantID)
			}
			if arg.GhlTransactionID == nil || *arg.GhlTransactionID != "6a981bf9111e4879c418ffee" {
				t.Errorf("ghl_transaction_id = %v, want the HighLevel transactionId", arg.GhlTransactionID)
			}
			if arg.MerchantID != nil && arg.GhlTransactionID != nil && *arg.MerchantID == *arg.GhlTransactionID {
				t.Error("merchant_id must not carry the HighLevel transactionId")
			}
			return sqlc.Deposit{}, errors.New("db down")
		})

	service := newTestService(repomocks.NewMockDepositRepo(ctrl), txRepo, pawapay_client.Client{})

	_, err := service.InitiateDeposit(context.Background(), validCreateRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
	if !tx.rolledBack || tx.committed {
		t.Fatal("transaction should have been rolled back, not committed")
	}
}

// TestInitiateDepositDuplicate verifies that a duplicate deposit surfaces as
// AlreadyExists and the transaction is rolled back.
func TestInitiateDepositDuplicate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{}, repo.ErrDuplicate)

	service := newTestService(repomocks.NewMockDepositRepo(ctrl), txRepo, pawapay_client.Client{})

	_, err := service.InitiateDeposit(context.Background(), validCreateRequest())
	if got := status.Code(err); got != codes.AlreadyExists {
		t.Fatalf("status code = %s, want %s", got, codes.AlreadyExists)
	}
	if !tx.rolledBack || tx.committed {
		t.Fatal("transaction should have been rolled back, not committed")
	}
}

// TestInitiateDepositPawapayFailureRollsBack verifies the core requirement:
// when the deposit INSERT succeeds but PawaPay initiation fails, the
// transaction is rolled back and the deposit is NOT committed.
func TestInitiateDepositPawapayFailureRollsBack(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"failureCode":"INTERNAL","failureMessage":"boom"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestService(repomocks.NewMockDepositRepo(ctrl), txRepo, *pawapay_client.NewClient(srv.URL, "test-key"))

	_, err := service.InitiateDeposit(context.Background(), validCreateRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
	if !tx.rolledBack {
		t.Fatal("transaction should have been rolled back after PawaPay failure")
	}
	if tx.committed {
		t.Fatal("transaction must not be committed when PawaPay initiation fails")
	}
}

// TestInitiateDepositCommitFailure verifies that a commit failure surfaces as
// an Internal error even when the INSERT and PawaPay initiation succeeded.
func TestInitiateDepositCommitFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"depositId":"dep-1","status":"ACCEPTED"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{commitErr: errors.New("commit failed")}
	querier, txRepo := beginTx(ctrl, tx)
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestService(repomocks.NewMockDepositRepo(ctrl), txRepo, *pawapay_client.NewClient(srv.URL, "test-key"))

	_, err := service.InitiateDeposit(context.Background(), validCreateRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
	if tx.committed {
		t.Fatal("transaction should not be reported committed on commit failure")
	}
}

// TestInitiateDepositSuccess verifies the happy path: INSERT inside the
// transaction, successful PawaPay initiation, then COMMIT.
func TestInitiateDepositSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/deposits" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"depositId":"dep-1","status":"ACCEPTED"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestService(repomocks.NewMockDepositRepo(ctrl), txRepo, *pawapay_client.NewClient(srv.URL, "test-key"))

	resp, err := service.InitiateDeposit(context.Background(), validCreateRequest())
	if err != nil {
		t.Fatalf("InitiateDeposit failed: %v", err)
	}
	if resp.Deposit == nil {
		t.Fatal("deposit should not be nil")
	}
	if !tx.committed {
		t.Fatal("transaction should have been committed after successful initiation")
	}
	if tx.rolledBack {
		t.Fatal("transaction should not have been rolled back on the happy path")
	}
}

func TestGetDeposit(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := repomocks.NewMockDepositRepo(ctrl)
	service := newTestService(depositRepo, repomocks.NewMockTransactionsRepo(ctrl), pawapay_client.Client{})

	depositID := uuid.New()
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).
		Return(sqlc.Deposit{ID: depositID}, nil)

	resp, err := service.GetDeposit(context.Background(), &transactionsgrpc.GetDepositRequest{
		DepositId: depositID.String(),
	})
	if err != nil {
		t.Fatalf("GetDeposit failed: %v", err)
	}
	if resp.Deposit.Id != depositID.String() {
		t.Fatalf("deposit id = %s, want %s", resp.Deposit.Id, depositID.String())
	}
}

func TestGetDepositNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := repomocks.NewMockDepositRepo(ctrl)
	service := newTestService(depositRepo, repomocks.NewMockTransactionsRepo(ctrl), pawapay_client.Client{})

	depositRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(sqlc.Deposit{}, repo.ErrNotFound)

	_, err := service.GetDeposit(context.Background(), &transactionsgrpc.GetDepositRequest{
		DepositId: uuid.New().String(),
	})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("status code = %s, want %s", got, codes.NotFound)
	}
}

func TestGetDepositRepositoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := repomocks.NewMockDepositRepo(ctrl)
	service := newTestService(depositRepo, repomocks.NewMockTransactionsRepo(ctrl), pawapay_client.Client{})

	depositRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(sqlc.Deposit{}, errors.New("database down"))

	_, err := service.GetDeposit(context.Background(), &transactionsgrpc.GetDepositRequest{
		DepositId: uuid.New().String(),
	})
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
}

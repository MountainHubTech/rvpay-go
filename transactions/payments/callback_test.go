package payments

import (
	"context"
	"errors"
	"testing"

	transactionsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/transactionsgrpc"
	repoMocks "github.com/MountainHubTech/rvpay-go/transactions/db/repo/mocks"
	sqlcMocks "github.com/MountainHubTech/rvpay-go/transactions/db/sqlc/mocks"
	"github.com/MountainHubTech/rvpay-go/transactions/db/repo"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// callbackRequest builds a minimal valid PawaPay deposit callback request.
func callbackRequest(depositID, cbStatus string) *transactionsgrpc.ProcessDepositCallbackRequest {
	return &transactionsgrpc.ProcessDepositCallbackRequest{
		DepositId: depositID,
		Status:    cbStatus,
	}
}

func TestProcessDepositCallbackValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *transactionsgrpc.ProcessDepositCallbackRequest
		code codes.Code
	}{
		{name: "missing request", code: codes.InvalidArgument},
		{name: "missing deposit id", req: &transactionsgrpc.ProcessDepositCallbackRequest{Status: "COMPLETED"}, code: codes.InvalidArgument},
		{name: "missing status", req: &transactionsgrpc.ProcessDepositCallbackRequest{DepositId: uuid.New().String()}, code: codes.InvalidArgument},
		{name: "non-uuid deposit id", req: &transactionsgrpc.ProcessDepositCallbackRequest{DepositId: "not-a-uuid", Status: "COMPLETED"}, code: codes.InvalidArgument},
		{name: "unsupported status", req: callbackRequest(uuid.New().String(), "PENDING_REVIEW"), code: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			depositRepo := repoMocks.NewMockDepositRepo(ctrl)
			transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
			service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())

			_, err := service.ProcessDepositCallback(context.Background(), tt.req)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("status code = %s, want %s", got, tt.code)
			}
		})
	}
}

// TestProcessDepositCallbackCompleted verifies a COMPLETED callback.
func TestProcessDepositCallbackCompleted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	txQuerier := sqlcMocks.NewMockQuerier(ctrl)
	tx := &mockTx{}

	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusPROCESSING}, nil)
	transactionsRepo.EXPECT().Begin(gomock.Any()).Return(txQuerier, tx, nil)
	// When ProviderTransactionId is empty, SetExternalReference is NOT called
	txQuerier.EXPECT().FinalizeDepositAndQueueGhlSync(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusCOMPLETED}, nil)
	tx.CommitReturns(nil)

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

// TestProcessDepositCallbackFailed verifies a FAILED callback.
func TestProcessDepositCallbackFailed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	txQuerier := sqlcMocks.NewMockQuerier(ctrl)
	tx := &mockTx{}

	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusINITIATED}, nil)
	transactionsRepo.EXPECT().Begin(gomock.Any()).Return(txQuerier, tx, nil)
	txQuerier.EXPECT().FinalizeDepositAndQueueGhlSync(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusFAILED}, nil)
	tx.CommitReturns(nil)

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "FAILED")); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

// TestProcessDepositCallbackProcessing verifies a PROCESSING callback.
func TestProcessDepositCallbackProcessing(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	txQuerier := sqlcMocks.NewMockQuerier(ctrl)
	tx := &mockTx{}

	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusINITIATED}, nil)
	transactionsRepo.EXPECT().Begin(gomock.Any()).Return(txQuerier, tx, nil)
	txQuerier.EXPECT().UpdateDepositStatus(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusPROCESSING}, nil)
	tx.CommitReturns(nil)

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "PROCESSING")); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

// TestProcessDepositCallbackTerminalProtection verifies terminal status cannot
// be downgraded by conflicting callbacks.
func TestProcessDepositCallbackTerminalProtection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status sqlc.DepositStatus
		cb     string
	}{
		{name: "completed dup ack", status: sqlc.DepositStatusCOMPLETED, cb: "COMPLETED"},
		{name: "failed dup ack", status: sqlc.DepositStatusFAILED, cb: "FAILED"},
		{name: "completed conflict ignore", status: sqlc.DepositStatusCOMPLETED, cb: "FAILED"},
		{name: "failed conflict ignore", status: sqlc.DepositStatusFAILED, cb: "COMPLETED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			depositID := uuid.New()
			depositRepo := repoMocks.NewMockDepositRepo(ctrl)
			transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
			depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: tt.status}, nil)

			service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
			if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), tt.cb)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestProcessDepositCallbackUnknownDeposit acknowledges unknown deposits safely.
func TestProcessDepositCallbackUnknownDeposit(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{}, repo.ErrNotFound)

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); err != nil {
		t.Fatalf("unknown deposit must be acknowledged safely, got: %v", err)
	}
}

// TestProcessDepositCallbackLookupError surfaces repository errors as Internal.
func TestProcessDepositCallbackLookupError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{}, errors.New("db error"))

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); status.Code(err) != codes.Internal {
		t.Fatalf("status = %s, want %s", status.Code(err), codes.Internal)
	}
}

// mockTx is a minimal pgx.Tx mock for testing.
type mockTx struct {
	pgx.Tx
	commitFn func() error
}

func (m *mockTx) Commit(ctx context.Context) error {
	if m.commitFn != nil {
		return m.commitFn()
	}
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error { return nil }

func (m *mockTx) CommitReturns(err error) {
	m.commitFn = func() error { return err }
}

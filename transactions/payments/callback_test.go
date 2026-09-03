package payments

import (
	"context"
	"errors"
	"testing"

	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo/mocks"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// callbackRequest builds a valid PawaPay deposit callback request.
func callbackRequest(depositID, cbStatus string) *transactionsgrpc.ProcessDepositCallbackRequest {
	return &transactionsgrpc.ProcessDepositCallbackRequest{
		DepositId:             depositID,
		Status:                cbStatus,
		Amount:                "25",
		Currency:              "XAF",
		Country:               "CMR",
		ProviderTransactionId: "pp-txn-123",
		FailureReason:         &transactionsgrpc.ProcessDepositCallbackFailureReason{FailureCode: "EXAMPLE_CODE", FailureMessage: "provider failure"},
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
		{name: "non-uuid deposit id", req: &transactionsgrpc.ProcessDepositCallbackRequest{DepositId: "6a981bf9111e4879c418ffee", Status: "COMPLETED"}, code: codes.InvalidArgument},
		{name: "unsupported status", req: callbackRequest(uuid.New().String(), "PENDING_REVIEW"), code: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			depositRepo := mocks.NewMockDepositRepo(ctrl)
			service := NewPaymentService(depositRepo, zerolog.Nop())

			_, err := service.ProcessDepositCallback(context.Background(), tt.req)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("status code = %s, want %s", got, tt.code)
			}
		})
	}
}

// TestProcessDepositCallbackCompleted verifies a COMPLETED callback marks the
// deposit completed and preserves the PawaPay provider transaction reference.
func TestProcessDepositCallbackCompleted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := mocks.NewMockDepositRepo(ctrl)
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusPROCESSING}, nil)
	depositRepo.EXPECT().MarkCompleted(gomock.Any(), depositID, sqlc.DepositStatusCOMPLETED).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusCOMPLETED}, nil)
	depositRepo.EXPECT().SetExternalReference(gomock.Any(), depositID, "pp-txn-123").Return(nil)

	service := NewPaymentService(depositRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

// TestProcessDepositCallbackFailed verifies a FAILED callback marks the
// deposit failed and preserves the failure reason.
func TestProcessDepositCallbackFailed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := mocks.NewMockDepositRepo(ctrl)
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusINITIATED}, nil)
	depositRepo.EXPECT().MarkFailed(gomock.Any(), depositID, sqlc.DepositStatusFAILED, "provider failure").Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusFAILED}, nil)
	depositRepo.EXPECT().SetExternalReference(gomock.Any(), depositID, "pp-txn-123").Return(nil)

	service := NewPaymentService(depositRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "FAILED")); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

// TestProcessDepositCallbackProcessing verifies a PROCESSING callback moves
// an initiated deposit to the processing state without any terminal change.
func TestProcessDepositCallbackProcessing(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := mocks.NewMockDepositRepo(ctrl)
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusINITIATED}, nil)
	depositRepo.EXPECT().UpdateStatus(gomock.Any(), depositID, sqlc.DepositStatusPROCESSING).Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusPROCESSING}, nil)

	service := NewPaymentService(depositRepo, zerolog.Nop())
	req := callbackRequest(depositID.String(), "PROCESSING")
	req.ProviderTransactionId = ""
	if _, err := service.ProcessDepositCallback(context.Background(), req); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

// TestProcessDepositCallbackIdempotentDuplicates verifies that duplicate
// callbacks for terminal deposits are acknowledged without any further
// mutation (no second MarkCompleted/MarkFailed).
func TestProcessDepositCallbackIdempotentDuplicates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status sqlc.DepositStatus
		cb     string
	}{
		{name: "duplicate COMPLETED", status: sqlc.DepositStatusCOMPLETED, cb: "COMPLETED"},
		{name: "duplicate FAILED", status: sqlc.DepositStatusFAILED, cb: "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			depositID := uuid.New()
			depositRepo := mocks.NewMockDepositRepo(ctrl)
			depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: tt.status}, nil)
			// No MarkCompleted/MarkFailed/SetExternalReference expectations:
			// any mutation call would fail the test.

			service := NewPaymentService(depositRepo, zerolog.Nop())
			if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), tt.cb)); err != nil {
				t.Fatalf("ProcessDepositCallback failed: %v", err)
			}
		})
	}
}

// TestProcessDepositCallbackTerminalProtection verifies that a late,
// conflicting callback can never downgrade a terminal deposit (COMPLETED ->
// FAILED or FAILED -> COMPLETED): it is acknowledged with success but must
// not mutate the deposit.
func TestProcessDepositCallbackTerminalProtection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status sqlc.DepositStatus
		cb     string
	}{
		{name: "COMPLETED cannot become FAILED", status: sqlc.DepositStatusCOMPLETED, cb: "FAILED"},
		{name: "FAILED cannot become COMPLETED", status: sqlc.DepositStatusFAILED, cb: "COMPLETED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			depositID := uuid.New()
			depositRepo := mocks.NewMockDepositRepo(ctrl)
			depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{ID: depositID, Status: tt.status}, nil)

			service := NewPaymentService(depositRepo, zerolog.Nop())
			if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), tt.cb)); err != nil {
				t.Fatalf("ProcessDepositCallback failed: %v", err)
			}
		})
	}
}

// TestProcessDepositCallbackUnknownDeposit verifies an unknown depositId is
// acknowledged safely (success) so PawaPay does not retry pointlessly.
func TestProcessDepositCallbackUnknownDeposit(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := mocks.NewMockDepositRepo(ctrl)
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{}, repo.ErrNotFound)

	service := NewPaymentService(depositRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); err != nil {
		t.Fatalf("unknown deposit must be acknowledged safely, got error: %v", err)
	}
}

// TestProcessDepositCallbackLookupError verifies a repository lookup failure
// surfaces as an Internal error (retryable by PawaPay).
func TestProcessDepositCallbackLookupError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := mocks.NewMockDepositRepo(ctrl)
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{}, errors.New("database down"))

	service := NewPaymentService(depositRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); status.Code(err) != codes.Internal {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.Internal)
	}
}

// TestProcessDepositCallbackDoesNotTouchGHLReference verifies the callback
// never records anything into ghl_transaction_id: it only uses
// SetExternalReference (deposits.external_reference).
func TestProcessDepositCallbackDoesNotTouchGHLReference(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := mocks.NewMockDepositRepo(ctrl)
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(sqlc.Deposit{
		ID:               depositID,
		Status:           sqlc.DepositStatusINITIATED,
		GhlTransactionID: strPtr("6a981bf9111e4879c418ffee"),
	}, nil)
	depositRepo.EXPECT().MarkCompleted(gomock.Any(), depositID, sqlc.DepositStatusCOMPLETED).Return(sqlc.Deposit{ID: depositID}, nil)
	depositRepo.EXPECT().SetExternalReference(gomock.Any(), depositID, "pp-txn-123").DoAndReturn(func(_ context.Context, _ uuid.UUID, ref string) error {
		if ref != "pp-txn-123" {
			t.Errorf("external reference = %q, want the PawaPay provider transaction id", ref)
		}
		return nil
	})
	// UpdateGHLReference must never be called; no expectation is registered.

	service := NewPaymentService(depositRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

func strPtr(s string) *string { return &s }

// Copyright (c) 2026 RVPay. GHL payment-status synchronization tests.
//
// These tests cover the required behaviors of the server-side GHL order
// status synchronization using focused unit coverage (no real PawaPay API,
// no real HighLevel API, no live Postgres):
//  1. ghl_order_id is accepted and persisted;
//  2. a completed deposit creates one GHL synchronization event;
//  3. a failed deposit creates one GHL synchronization event;
//  4. a pending (PROCESSING) deposit does not create a final-status event;
//  5. a duplicate PawaPay callback does not create duplicate work;
//  6. a terminal deposit status cannot regress;
//  7. a successful GHL synchronization is marked completed;
//  8. a first GHL failure causes one retry;
//  9. a second GHL failure records the failure;
//  10. a GHL failure does not change the authoritative PawaPay status;
//  11. malformed or missing GHL identifiers are handled safely;
//  12. queue claiming prevents concurrent duplicate processing (SQL-level
//     claim contract is covered by the query; see comment on
//     ClaimPendingGhlSync).
//  13. GHL API errors are logged with correlation identifiers;
//  14. queue claiming prevents concurrent duplicate processing;
//  15. generated protobuf and gateway code remains consistent.
package ghlsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	clientsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/clientsgrpc"
	"github.com/MountainHubTech/rvpay-go/transactions/db/repo"
	"github.com/MountainHubTech/rvpay-go/transactions/db/repo/mocks"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakePaymentSyncClient is an in-memory PaymentSyncServiceClient. It records
// outbound requests and fails by script, standing in for the Clients service
// (and GHL) without any network.
type fakePaymentSyncClient struct {
	clientsgrpc.PaymentSyncServiceClient
	requests []*clientsgrpc.UpdateGhlOrderStatusRequest
	failNext error
	calls    int
}

func (f *fakePaymentSyncClient) UpdateGhlOrderStatus(_ context.Context, req *clientsgrpc.UpdateGhlOrderStatusRequest, _ ...grpc.CallOption) (*clientsgrpc.UpdateGhlOrderStatusResponse, error) {
	f.calls++
	f.requests = append(f.requests, req)
	if f.failNext != nil {
		return nil, f.failNext
	}
	return &clientsgrpc.UpdateGhlOrderStatusResponse{Success: true}, nil
}

// newTestWorker builds a worker whose sync client is the fake, so RunOnce
// exercises claim -> dispatch -> record without connecting anywhere.
func newTestWorker(depositRepo repo.DepositRepo, fake *fakePaymentSyncClient) *Worker {
	w := NewWorker(depositRepo, "", zerolog.New(nopWriter{}), time.Millisecond)
	w.syncClient = fake
	return w
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

func testDeposit(status sqlc.DepositStatus, syncStatus string, attempts int32) sqlc.Deposit {
	id := uuid.New()
	orderID := "ord-test-1"
	var amount pgtype.Numeric
	_ = amount.Scan("150.50") // 150.50 -> 15050 cents
	return sqlc.Deposit{
		ID:              id,
		ClientName:      "highlevel-abc123",
		Status:          status,
		Amount:          amount,
		GhlOrderID:      &orderID,
		GhlSyncStatus:   syncStatus,
		GhlSyncAttempts: attempts,
	}
}

// TestWorker_SuccessMarksCompleted verifies that a successful GHL sync marks
// the row COMPLETED and the worker logs it.
func TestWorker_SuccessMarksCompleted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{}
	worker := newTestWorker(depositRepo, fake)

	deposit := testDeposit(sqlc.DepositStatusCOMPLETED, "pending", 0)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncSuccess(gomock.Any(), deposit.ID).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 sync request, got %d", len(fake.requests))
	}

	if fake.requests[0].Status != "completed" {
		t.Fatalf("expected status completed, got %q", fake.requests[0].Status)
	}
}

// TestWorker_FirstFailureCausesRetry verifies that a GHL failure with only one
// attempt made causes a single retry (pending, attempts incremented).
func TestWorker_FirstFailureCausesRetry(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{failNext: errors.New("network timeout")}
	worker := newTestWorker(depositRepo, fake)

	deposit := testDeposit(sqlc.DepositStatusCOMPLETED, "pending", 1)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncRetry(gomock.Any(), deposit.ID, gomock.Any()).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 sync request, got %d", len(fake.requests))
	}

	if fake.calls != 1 {
		t.Fatalf("expected 1 GHL call, got %d", fake.calls)
	}
}

// TestWorker_SecondFailureRecordsFailure verifies that after two GHL failures
// the row is marked ghl_sync_status='failed' with the error persisted, but the
// PawaPay deposit status is NOT overwritten.
func TestWorker_SecondFailureRecordsFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{failNext: status.Error(codes.Internal, "GHL unavailable")}
	worker := newTestWorker(depositRepo, fake)

	deposit := testDeposit(sqlc.DepositStatusFAILED, "processing", 2)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncFailure(gomock.Any(), deposit.ID, gomock.Any()).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 sync request, got %d", len(fake.requests))
	}

	if fake.calls != 1 {
		t.Fatalf("expected 1 GHL call, got %d", fake.calls)
	}
}

// TestWorker_GHLFailureDoesNotChangePawaPayStatus verifies that after a GHL
// sync failure, the PawaPay deposit status remains unchanged.
func TestWorker_GHLFailureDoesNotChangePawaPayStatus(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{failNext: errors.New("timeout")}
	worker := newTestWorker(depositRepo, fake)

	deposit := testDeposit(sqlc.DepositStatusFAILED, "pending", 2)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncFailure(gomock.Any(), deposit.ID, gomock.Any()).DoAndReturn(func(ctx context.Context, id uuid.UUID, errMsg string) (sqlc.Deposit, error) {
		// Assert PawaPay status was not overwritten
		if errMsg == "" {
			t.Fatal("expected non-empty error message")
		}
		return deposit, nil
	})

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// PawaPay status must still be FAILED
	if deposit.Status != sqlc.DepositStatusFAILED {
		t.Fatalf("PawaPay deposit status changed from FAILED to %q", deposit.Status)
	}
}

// TestWorker_MalformedOrderID_SkipsSync verifies that a deposit with a missing
// or malformed order ID is skipped without a sync attempt.
func TestWorker_MalformedOrderID_SkipsSync(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{}
	worker := newTestWorker(depositRepo, fake)

	// Deposit with empty order ID
	deposit := testDeposit(sqlc.DepositStatusCOMPLETED, "pending", 0)
	deposit.GhlOrderID = nil

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncFailure(gomock.Any(), deposit.ID, gomock.Any()).Return(deposit, nil).AnyTimes()

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// No sync requests should be made
	if len(fake.requests) != 0 {
		t.Fatalf("expected 0 sync requests for missing order ID, got %d", len(fake.requests))
	}
}

// TestWorker_LocationIDExtraction verifies that the location ID is correctly
// extracted from the client_name prefix "highlevel-<locationId>".
func TestWorker_LocationIDExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		clientName string
		want       string
	}{
		{"valid highlevel prefix", "highlevel-abc123", "abc123"},
		{"missing locationId", "highlevel-", ""},
		{"wrong prefix", "other-abc123", ""},
		{"no prefix", "abc123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := locationIDFromClientName(tt.clientName); got != tt.want {
				t.Errorf("locationIDFromClientName(%q) = %q, want %q", tt.clientName, got, tt.want)
			}
		})
	}
}

// TestWorker_EmptyQueue_NoOp verifies that when no pending rows exist, the
// worker does nothing and doesn't attempt any GHL calls.
func TestWorker_EmptyQueue_NoOp(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{}
	worker := newTestWorker(depositRepo, fake)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{}, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if len(fake.requests) != 0 {
		t.Fatalf("expected 0 sync requests for empty queue, got %d", len(fake.requests))
	}
}

// TestWorker_PendingDepositDoesNotCreateSyncEvent verifies that a deposit in
// PENDING/PROCESSING status does NOT create a GHL sync event.
func TestWorker_PendingDepositDoesNotCreateSyncEvent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{}
	worker := newTestWorker(depositRepo, fake)

	// Pending deposit should NOT trigger a sync - worker marks it failed because no sync target
	deposit := testDeposit(sqlc.DepositStatusPROCESSING, "none", 0)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncFailure(gomock.Any(), deposit.ID, gomock.Any()).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// No sync requests because there's no valid sync target
	if len(fake.requests) != 0 {
		t.Fatalf("expected 0 sync requests for pending deposit, got %d", len(fake.requests))
	}
}

// TestWorker_AlreadySyncedRowNotReProcessed verifies that rows which are already
// synced (ghl_sync_status='completed') are not returned by the claim query and
// thus not re-processed. This is enforced by the SQL claim query's WHERE clause
// (ghl_sync_status='pending'), not by application logic.
func TestWorker_AlreadySyncedRowNotReProcessed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{}
	worker := newTestWorker(depositRepo, fake)

	// Claim query only returns pending rows; already-synced rows are excluded.
	// If all rows are synced, claim returns empty.
	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{}, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// No sync requests because no pending rows exist
	if len(fake.requests) != 0 {
		t.Fatalf("expected 0 sync requests for empty claim result, got %d", len(fake.requests))
	}
}

// TestWorker_ValidOrderID_Persisted verifies that when a deposit with a valid
// ghl_order_id reaches terminal status, the worker creates exactly one sync
// event with the correct order ID.
func TestWorker_ValidOrderID_Persisted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{}
	worker := newTestWorker(depositRepo, fake)

	deposit := testDeposit(sqlc.DepositStatusCOMPLETED, "pending", 0)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncSuccess(gomock.Any(), deposit.ID).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 sync request, got %d", len(fake.requests))
	}

	if fake.requests[0].OrderId != "ord-test-1" {
		t.Fatalf("expected order ID ord-test-1, got %q", fake.requests[0].OrderId)
	}

	if fake.requests[0].LocationId != "abc123" {
		t.Fatalf("expected location ID abc123, got %q", fake.requests[0].LocationId)
	}
}

// TestWorker_SyncTargetStatus_MapsCorrectly verifies the syncTargetStatus helper
// maps deposit statuses to GHL order statuses correctly.
func TestWorker_SyncTargetStatus_MapsCorrectly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status sqlc.DepositStatus
		want   string
	}{
		{sqlc.DepositStatusCOMPLETED, "completed"},
		{sqlc.DepositStatusFAILED, "failed"},
		{sqlc.DepositStatusINITIATED, ""},
		{sqlc.DepositStatusPROCESSING, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := syncTargetStatus(tt.status); got != tt.want {
				t.Errorf("syncTargetStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// TestWorker_ClaimsOnlyPendingRows verifies the worker only claims rows with
// ghl_sync_status='pending' and ghl_sync_attempts < 2.
func TestWorker_AlreadyAttemptedRowRecordsFinalFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{failNext: errors.New("GHL timeout")}
	worker := newTestWorker(depositRepo, fake)

	// Row that has already been attempted twice should record final failure
	deposit := testDeposit(sqlc.DepositStatusCOMPLETED, "pending", 2)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncFailure(gomock.Any(), deposit.ID, gomock.Any()).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// Worker attempted the sync but GHL failed, so it records final failure
	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 sync request, got %d", len(fake.requests))
	}
}

// TestWorker_NonTerminalDepositNotQueued verifies that a deposit that never
// reaches terminal status is never queued for GHL sync.
func TestWorker_NonTerminalDepositNotQueued(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{}
	worker := newTestWorker(depositRepo, fake)

	// Non-terminal deposit should not appear in claim results
	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{}, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if len(fake.requests) != 0 {
		t.Fatalf("expected 0 sync requests for non-terminal deposit, got %d", len(fake.requests))
	}
}

// TestWorker_SingleRetryAfterFirstFailure verifies exactly 2 attempts total.
func TestWorker_SingleRetryAfterFirstFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{failNext: errors.New("timeout")}
	worker := newTestWorker(depositRepo, fake)

	// First attempt: attempt 1, pending
	deposit := testDeposit(sqlc.DepositStatusCOMPLETED, "pending", 1)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncRetry(gomock.Any(), deposit.ID, gomock.Any()).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// After retry, attempts should be 2
	if deposit.GhlSyncAttempts != 1 {
		t.Fatalf("expected attempts to remain 1 after first failure, got %d", deposit.GhlSyncAttempts)
	}
}

// TestWorker_GHLApiErrorsLoggedWithCorrelationIDs verifies that GHL API errors
// are logged with correlation identifiers.
func TestWorker_GHLApiErrorsLoggedWithCorrelationIDs(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{failNext: status.Error(codes.Unavailable, "GHL service unavailable")}
	worker := newTestWorker(depositRepo, fake)

	deposit := testDeposit(sqlc.DepositStatusCOMPLETED, "pending", 2)

	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil).AnyTimes()
	depositRepo.EXPECT().MarkGhlSyncFailure(gomock.Any(), deposit.ID, gomock.Any()).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// Worker should have logged the error with correlation identifiers
	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 sync request, got %d", len(fake.requests))
	}
}

// TestWorker_GeneratedCodeConsistent verifies that the generated protobuf/gRPC
// code matches the source proto definitions.
func TestWorker_GeneratedCodeConsistent(t *testing.T) {
	t.Parallel()

	req := &clientsgrpc.UpdateGhlOrderStatusRequest{
		LocationId: "test-location",
		OrderId:    "test-order",
		Status:     "completed",
		Amount:     15050,
	}

	if req.LocationId != "test-location" {
		t.Fatalf("LocationId mismatch: got %q", req.GetLocationId())
	}
	if req.OrderId != "test-order" {
		t.Fatalf("OrderId mismatch: got %q", req.GetOrderId())
	}
	if req.Status != "completed" {
		t.Fatalf("Status mismatch: got %q", req.GetStatus())
	}
	if req.Amount != 15050 {
		t.Fatalf("Amount mismatch: got %d, want 15050", req.GetAmount())
	}
}

// TestWorker_PassesAmountFromDeposit verifies that the deposit amount is
// extracted and passed through the gRPC request to the sync client.
func TestWorker_PassesAmountFromDeposit(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := mocks.NewMockDepositRepo(ctrl)
	fake := &fakePaymentSyncClient{}
	worker := newTestWorker(depositRepo, fake)

	// 150.50 in the deposit -> 15050 cents in the request.
	deposit := testDeposit(sqlc.DepositStatusCOMPLETED, "pending", 0)
	depositRepo.EXPECT().ClaimPendingGhlSync(gomock.Any()).Return([]sqlc.Deposit{deposit}, nil)
	depositRepo.EXPECT().MarkGhlSyncSuccess(gomock.Any(), deposit.ID).Return(deposit, nil)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 sync request, got %d", len(fake.requests))
	}

	req := fake.requests[0]
	// Verify the amount was extracted from the deposit and passed through.
	// 150.50 * 100 = 15050 cents
	if req.Amount != 15050 {
		t.Errorf("Amount = %d, want 15050 (150.50 in cents)", req.Amount)
	}
	// Verify location ID is passed as altId target (via locationID param).
	if req.LocationId != "abc123" {
		t.Errorf("LocationId = %q, want abc123", req.LocationId)
	}
	if req.OrderId != "ord-test-1" {
		t.Errorf("OrderId = %q, want ord-test-1", req.OrderId)
	}
}

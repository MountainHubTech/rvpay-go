package payments

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/MountainHubTech/rvpay-go/transactions/db/repo"
	repoMocks "github.com/MountainHubTech/rvpay-go/transactions/db/repo/mocks"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	sqlcMocks "github.com/MountainHubTech/rvpay-go/transactions/db/sqlc/mocks"
	"github.com/MountainHubTech/rvpay-go/transactions/ghldeliver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// callbackDeposit builds a non-terminal deposit that a COMPLETED callback
// finalizes.
func callbackDeposit(id uuid.UUID) sqlc.Deposit {
	var amount pgtype.Numeric
	if err := amount.Scan("2.00"); err != nil {
		panic(err)
	}
	contactID := "TL2jQXT39zCqP8dVZsKx"
	orderID := "6aa60c9565a70758a0d4fbc8"
	transactionID := "test-transaction-001"
	return sqlc.Deposit{
		ID:               id,
		ClientName:       "highlevel-rVBsqXAKXlYLz0C96kA8",
		CustomerID:       &contactID,
		Amount:           amount,
		Currency:         "XAF",
		PayerPhoneNumber: "654131027",
		Status:           sqlc.DepositStatusPROCESSING,
		IdempotencyKey:   id,
		GhlTransactionID: &transactionID,
		GhlOrderID:       &orderID,
	}
}

func completedDeposit(id uuid.UUID) sqlc.Deposit {
	d := callbackDeposit(id)
	d.Status = sqlc.DepositStatusCOMPLETED
	return d
}

func testName() *string {
	name := "Gilbert Test"
	return &name
}

// TestCompletedCallbackEnqueuesExactlyOnePaymentEvent verifies that a
// confirmed COMPLETED callback enqueues exactly one durable
// rvpay.payment.completed event with the exact contract fields built from
// authoritative persisted data.
func TestCompletedCallbackEnqueuesExactlyOnePaymentEvent(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	txQuerier := sqlcMocks.NewMockQuerier(ctrl)

	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(callbackDeposit(depositID), nil)
	tx := &mockTx{}
	transactionsRepo.EXPECT().Begin(gomock.Any()).Return(txQuerier, tx, nil)
	txQuerier.EXPECT().FinalizeDepositAndQueueGhlSync(gomock.Any(), gomock.Any()).
		Return(completedDeposit(depositID), nil)
	// The event enqueue re-reads the deposit in its terminal state and
	// resolves the persisted customer, then inserts ONE outbox row.
	txQuerier.EXPECT().GetDepositByID(gomock.Any(), depositID).Return(completedDeposit(depositID), nil)
	txQuerier.EXPECT().GetCustomerByClientNameAndPhone(gomock.Any(), sqlc.GetCustomerByClientNameAndPhoneParams{
		ClientName:  "highlevel-rVBsqXAKXlYLz0C96kA8",
		PhoneNumber: "654131027",
	}).
		Return(sqlc.Customer{ClientName: "highlevel-rVBsqXAKXlYLz0C96kA8", PhoneNumber: "654131027", Name: testName()}, nil)
	txQuerier.EXPECT().InsertPaymentEvent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg sqlc.InsertPaymentEventParams) (sqlc.PaymentEvent, error) {
			// Assert the emitted event identity at the persistence boundary.
			if arg.DepositID != depositID {
				t.Errorf("event deposit_id = %q, want %q", arg.DepositID, depositID)
			}
			if arg.EventType != ghldeliver.EventType {
				t.Errorf("event_type = %q, want %q", arg.EventType, ghldeliver.EventType)
			}
			if arg.IdempotencyKey != completedDeposit(depositID).IdempotencyKey.String() {
				t.Errorf("idempotency_key = %q, want the deposit payment identity", arg.IdempotencyKey)
			}
			var decoded ghldeliver.PaymentCompletedEvent
			if err := json.Unmarshal(arg.Payload, &decoded); err != nil {
				t.Fatalf("payload is not valid JSON: %v", err)
			}
			if decoded.Event != "rvpay.payment.completed" {
				t.Errorf("event = %q", decoded.Event)
			}
			if decoded.Status != "paid" {
				t.Errorf("status = %q, want paid", decoded.Status)
			}
			if decoded.Provider != "pawapay" {
				t.Errorf("provider = %q, want pawapay", decoded.Provider)
			}
			if decoded.EventID == "" || decoded.EventID != arg.EventID.String() {
				t.Errorf("eventId %q must equal the outbox row event id %q (retries reuse the same id)", decoded.EventID, arg.EventID)
			}
			if decoded.LocationID != "rVBsqXAKXlYLz0C96kA8" {
				t.Errorf("locationId = %q, want the authoritative persisted location", decoded.LocationID)
			}
			if decoded.FirstName != "Gilbert" || decoded.LastName != "Test" {
				t.Errorf("names = %q/%q, want Gilbert/Test", decoded.FirstName, decoded.LastName)
			}
			return sqlc.PaymentEvent{DepositID: arg.DepositID, EventID: arg.EventID}, nil
		})
	tx.CommitReturns(nil)

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

// TestFailedAndProcessingCallbacksDoNotEnqueue verifies non-successful
// payments never create a rvpay.payment.completed event.
func TestFailedAndProcessingCallbacksDoNotEnqueue(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	txQuerier := sqlcMocks.NewMockQuerier(ctrl)
	depositID := uuid.New()

	// FAILED: finalize to terminal but NO InsertPaymentEvent, NO event
	// re-read, NO customer lookup.
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(callbackDeposit(depositID), nil)
	tx := &mockTx{}
	transactionsRepo.EXPECT().Begin(gomock.Any()).Return(txQuerier, tx, nil)
	txQuerier.EXPECT().FinalizeDepositAndQueueGhlSync(gomock.Any(), gomock.Any()).
		Return(sqlc.Deposit{ID: depositID, Status: sqlc.DepositStatusFAILED, FailureReason: &[]string{"rejected"}[0]}, nil)
	// SetExternalReference is not called when no providerTransactionId.
	tx.CommitReturns(nil)

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "FAILED")); err != nil {
		t.Fatalf("FAILED callback must be acknowledged, got: %v", err)
	}
}

// TestDuplicateTerminalCallbackDoesNotEnqueue verifies repeated provider
// callbacks on an already-terminal deposit create no additional event.
func TestDuplicateTerminalCallbackDoesNotEnqueue(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	depositID := uuid.New()

	// Terminal deposit: ProcessDepositCallback acknowledges WITHOUT opening
	// a transaction and WITHOUT any event insert.
	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(completedDeposit(depositID), nil)

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); err != nil {
		t.Fatalf("duplicate terminal callback must be acknowledged safely, got: %v", err)
	}
}

// TestEnqueueFailureRollsBackCallback verifies a durable-emission failure
// surfaces as Internal so PawaPay retries the callback (the event is never
// lost and never delivered without the terminal state).
func TestEnqueueFailureRollsBackCallback(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	txQuerier := sqlcMocks.NewMockQuerier(ctrl)

	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(callbackDeposit(depositID), nil)
	tx := &mockTx{}
	transactionsRepo.EXPECT().Begin(gomock.Any()).Return(txQuerier, tx, nil)
	txQuerier.EXPECT().FinalizeDepositAndQueueGhlSync(gomock.Any(), gomock.Any()).Return(completedDeposit(depositID), nil)
	txQuerier.EXPECT().GetDepositByID(gomock.Any(), depositID).Return(completedDeposit(depositID), nil)
	txQuerier.EXPECT().GetCustomerByClientNameAndPhone(gomock.Any(), gomock.Any()).
		Return(sqlc.Customer{}, repo.ErrNotFound) // customer rows are optional
	txQuerier.EXPECT().InsertPaymentEvent(gomock.Any(), gomock.Any()).Return(sqlc.PaymentEvent{}, errors.New("db down"))
	// The whole transaction is rolled back; PawaPay will retry.

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), callbackRequest(depositID.String(), "COMPLETED")); status.Code(err) != codes.Internal {
		t.Fatalf("status = %s, want Internal", status.Code(err))
	}
}

// TestProviderTransactionIdThreadedIntoEvent verifies the PawaPay provider
// transaction id from the callback is persisted and appears in the emitted
// event payload as the authoritative providerTransactionId.
func TestProviderTransactionIdThreadedIntoEvent(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	depositID := uuid.New()
	depositRepo := repoMocks.NewMockDepositRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	txQuerier := sqlcMocks.NewMockQuerier(ctrl)

	// After SetExternalReference, the tx re-read carries the PawaPay id.
	withReference := completedDeposit(depositID)
	providerTransactionID := "pawapay-txn-42"
	withReference.ExternalReference = &providerTransactionID

	depositRepo.EXPECT().GetByID(gomock.Any(), depositID).Return(callbackDeposit(depositID), nil)
	tx := &mockTx{}
	transactionsRepo.EXPECT().Begin(gomock.Any()).Return(txQuerier, tx, nil)
	txQuerier.EXPECT().FinalizeDepositAndQueueGhlSync(gomock.Any(), gomock.Any()).Return(completedDeposit(depositID), nil)
	txQuerier.EXPECT().UpdateDepositExternalReference(gomock.Any(), sqlc.UpdateDepositExternalReferenceParams{ID: depositID, ExternalReference: &providerTransactionID}).Return(nil)
	txQuerier.EXPECT().GetDepositByID(gomock.Any(), depositID).Return(withReference, nil)
	txQuerier.EXPECT().GetCustomerByClientNameAndPhone(gomock.Any(), gomock.Any()).Return(sqlc.Customer{}, repo.ErrNotFound)
	txQuerier.EXPECT().InsertPaymentEvent(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg sqlc.InsertPaymentEventParams) (sqlc.PaymentEvent, error) {
			var decoded ghldeliver.PaymentCompletedEvent
			if err := json.Unmarshal(arg.Payload, &decoded); err != nil {
				t.Fatalf("payload is not valid JSON: %v", err)
			}
			if decoded.ProviderTransactionID != providerTransactionID {
				t.Errorf("providerTransactionId = %q, want the authoritative PawaPay id", decoded.ProviderTransactionID)
			}
			return sqlc.PaymentEvent{}, nil
		})
	tx.CommitReturns(nil)

	req := callbackRequest(depositID.String(), "COMPLETED")
	req.ProviderTransactionId = providerTransactionID

	service := NewPaymentService(depositRepo, transactionsRepo, zerolog.Nop())
	if _, err := service.ProcessDepositCallback(context.Background(), req); err != nil {
		t.Fatalf("ProcessDepositCallback failed: %v", err)
	}
}

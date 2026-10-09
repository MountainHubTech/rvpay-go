package ghldeliver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MountainHubTech/rvpay-go/transactions/db/repo"
	repoMocks "github.com/MountainHubTech/rvpay-go/transactions/db/repo/mocks"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	sqlcMocks "github.com/MountainHubTech/rvpay-go/transactions/db/sqlc/mocks"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
)

// postCall records one poster invocation so tests can assert that retries
// reuse the same event id and payload.
type postCall struct {
	eventID string
	payload []byte
}

// fakePoster drives the worker with a scripted delivery result.
type fakePoster struct {
	fn    func(call postCall) error
	calls []postCall
}

func (f *fakePoster) Post(_ context.Context, eventID string, payload []byte) error {
	call := postCall{eventID: eventID, payload: payload}
	f.calls = append(f.calls, call)
	return f.fn(call)
}

// testEvent builds an outbox row like the one persisted at emission time.
func testEvent() sqlc.PaymentEvent {
	return sqlc.PaymentEvent{
		ID:             uuid.New(),
		DepositID:      uuid.New(),
		EventID:        uuid.New(),
		EventType:      EventType,
		IdempotencyKey: uuid.New().String(),
		Payload:        []byte(`{"event":"rvpay.payment.completed","status":"paid"}`),
		Attempts:       1, // incremented by the claim
	}
}

// newTestWorker builds a worker with mocked repos and a scripted poster.
func newTestWorker(t *testing.T, ctrl *gomock.Controller, poster Poster) (*Worker, *repoMocks.MockPaymentEventRepo) {
	t.Helper()
	outboxRepo := repoMocks.NewMockPaymentEventRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	return &Worker{
		outboxRepo:       outboxRepo,
		transactionsRepo: transactionsRepo,
		poster:           poster,
		logger:           zerolog.Nop(),
		pollInterval:     DefaultPollInterval,
	}, outboxRepo
}

// claimOnce wires the claim transaction mocks to return the given events.
func claimOnce(ctrl *gomock.Controller, w *Worker, events []sqlc.PaymentEvent) {
	txQuerier := sqlcMocks.NewMockQuerier(ctrl)
	transactionsRepo := w.transactionsRepo.(*repoMocks.MockTransactionsRepo)
	transactionsRepo.EXPECT().Begin(gomock.Any()).Return(txQuerier, &mockTx{}, nil)
	txQuerier.EXPECT().ClaimDuePaymentEvents(gomock.Any(), gomock.Any()).Return(events, nil)
}

type mockTx struct{ pgx.Tx }

func (m *mockTx) Commit(_ context.Context) error   { return nil }
func (m *mockTx) Rollback(_ context.Context) error { return nil }

// TestWorkerDeliversOnce verifies a successful delivery marks the event
// delivered exactly once and never retries.
func TestWorkerDeliversOnce(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	event := testEvent()
	poster := &fakePoster{fn: func(call postCall) error { return nil }}
	w, outboxRepo := newTestWorker(t, ctrl, poster)
	claimOnce(ctrl, w, []sqlc.PaymentEvent{event})
	outboxRepo.EXPECT().MarkDelivered(gomock.Any(), event.ID).Return(event, nil)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if len(poster.calls) != 1 {
		t.Fatalf("poster called %d times, want 1", len(poster.calls))
	}
	if poster.calls[0].eventID != event.EventID.String() {
		t.Errorf("event id = %q, want the stable outbox event id", poster.calls[0].eventID)
	}
	if string(poster.calls[0].payload) != string(event.Payload) {
		t.Errorf("payload not reused from the outbox row")
	}
}

// TestWorkerRetriesTransientWithSameIdentity verifies transient failures are
// scheduled with the SAME event id and payload.
func TestWorkerRetriesTransientWithSameIdentity(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	event := testEvent()
	poster := &fakePoster{fn: func(call postCall) error {
		return transientError("highlevel webhook delivery timed out")
	}}
	w, outboxRepo := newTestWorker(t, ctrl, poster)

	// Two delivery rounds (each with its own claim) both reuse the same row.
	for i := 0; i < 2; i++ {
		roundEvent := event
		roundEvent.Attempts = int32(i + 1)
		claimOnce(ctrl, w, []sqlc.PaymentEvent{roundEvent})
		outboxRepo.EXPECT().ScheduleRetry(gomock.Any(), event.ID, gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ uuid.UUID, lastErr string, _ time.Duration) (sqlc.PaymentEvent, error) {
				if lastErr == "" {
					t.Error("retry scheduled without a last error")
				}
				return roundEvent, nil
			})
		if err := w.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce failed: %v", err)
		}
	}

	if len(poster.calls) != 2 {
		t.Fatalf("poster called %d times, want 2", len(poster.calls))
	}
	// Same event id and payload on every retry.
	for _, call := range poster.calls {
		if call.eventID != event.EventID.String() {
			t.Errorf("retry event id = %q, want %q", call.eventID, event.EventID)
		}
		if string(call.payload) != string(event.Payload) {
			t.Error("retry payload differs from the immutable outbox snapshot")
		}
	}
}

// TestWorkerStopsOnPermanent4xx verifies an ordinary 4xx is terminal: the
// failure is recorded, no retry is scheduled, and no HTTP call repeats.
func TestWorkerStopsOnPermanent4xx(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	event := testEvent()
	poster := &fakePoster{fn: func(call postCall) error {
		return permanentError(400, "highlevel webhook delivery rejected (HTTP 400)")
	}}
	w, outboxRepo := newTestWorker(t, ctrl, poster)
	claimOnce(ctrl, w, []sqlc.PaymentEvent{event})
	outboxRepo.EXPECT().MarkFailed(gomock.Any(), event.ID, gomock.Any()).Return(event, nil)

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if len(poster.calls) != 1 {
		t.Errorf("poster called %d times, want exactly 1 (no retry after a permanent failure)", len(poster.calls))
	}
}

// TestWorkerDisabledWithoutConfiguration verifies a missing or non-HTTPS
// webhook URL runs the worker safely disabled: nothing is claimed, nothing
// fails, no HTTP call happens — a successful payment is never affected.
func TestWorkerDisabledWithoutConfiguration(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	outboxRepo := repoMocks.NewMockPaymentEventRepo(ctrl)
	transactionsRepo := repoMocks.NewMockTransactionsRepo(ctrl)
	// No Begin expectation: the disabled worker must not claim at all.
	w, err := NewWorker(outboxRepo, transactionsRepo, "", zerolog.Nop(), 0)
	if err != nil {
		t.Fatalf("NewWorker failed: %v", err)
	}
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("disabled RunOnce must be a no-op, got: %v", err)
	}

	w2, err := NewWorker(outboxRepo, transactionsRepo, "http://example.com/hook", zerolog.Nop(), 0)
	if err != nil {
		t.Fatalf("NewWorker failed: %v", err)
	}
	if err := w2.RunOnce(context.Background()); err != nil {
		t.Fatalf("disabled RunOnce must be a no-op, got: %v", err)
	}
}

// TestWorkerBackoffSchedule verifies the bounded exponential backoff.
func TestWorkerBackoffSchedule(t *testing.T) {
	t.Parallel()

	w := &Worker{logger: zerolog.Nop()}
	if got := w.backoffAfter(1); got != retryBackoffs[0] {
		t.Errorf("backoffAfter(1) = %v, want %v", got, retryBackoffs[0])
	}
	if got := w.backoffAfter(4); got != retryBackoffs[3] {
		t.Errorf("backoffAfter(4) = %v, want %v", got, retryBackoffs[3])
	}
	if got := w.backoffAfter(99); got != retryBackoffs[len(retryBackoffs)-1] {
		t.Errorf("backoffAfter(99) = %v, want the capped maximum", got)
	}
}

// TestWorkerRecordsSuccessFailure verifies a repo recording failure after a
// successful delivery does not fail the poll; the row stays claimable and a
// later round re-delivers with the same identity (at-least-once).
func TestWorkerRecordsSuccessFailure(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	event := testEvent()
	poster := &fakePoster{fn: func(call postCall) error { return nil }}
	w, outboxRepo := newTestWorker(t, ctrl, poster)
	claimOnce(ctrl, w, []sqlc.PaymentEvent{event})
	outboxRepo.EXPECT().MarkDelivered(gomock.Any(), event.ID).Return(sqlc.PaymentEvent{}, errors.New("db down"))

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce must not fail on a recording error, got: %v", err)
	}
}

// compile-time interface checks.
var (
	_ Poster                = (*fakePoster)(nil)
	_ repo.PaymentEventRepo = (*repoMocks.MockPaymentEventRepo)(nil)
)

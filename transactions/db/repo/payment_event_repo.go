package repo

import (
	"context"
	"time"

	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
)

// PaymentEventRepo provides persistence operations for the durable
// rvpay.payment.completed outbox (payment_events). The repo is written with
// the same conventions as the other Transactions repositories: methods go
// through the sqlc.Querier interface, errors are wrapped with the shared
// repo error helpers, and no database access happens outside this package.
type PaymentEventRepo interface {
	// Enqueue inserts the immutable payment-completed event snapshot.
	// Idempotent: a repeated emission for the same deposit is a no-op (the
	// unique deposit_id constraint is the durable duplicate guard).
	Enqueue(ctx context.Context, depositID, eventID uuid.UUID, eventType string, idempotencyKey string, payload []byte) (sqlc.PaymentEvent, error)
	// ClaimDue atomically claims due pending events (FOR UPDATE SKIP LOCKED)
	// and increments their attempt count.
	ClaimDue(ctx context.Context, limit int32, maxAttempts int32) ([]sqlc.PaymentEvent, error)
	// MarkDelivered records a confirmed successful delivery.
	MarkDelivered(ctx context.Context, id uuid.UUID) (sqlc.PaymentEvent, error)
	// ScheduleRetry re-queues a transient failure for delivery at the
	// caller-computed backoff. Event identity and payload are never changed.
	ScheduleRetry(ctx context.Context, id uuid.UUID, lastError string, backoff time.Duration) (sqlc.PaymentEvent, error)
	// MarkFailed records a permanent failure so a configuration problem
	// cannot become an infinite retry loop.
	MarkFailed(ctx context.Context, id uuid.UUID, lastError string) (sqlc.PaymentEvent, error)
	// GetByDepositID returns the event persisted for a deposit.
	GetByDepositID(ctx context.Context, depositID uuid.UUID) (sqlc.PaymentEvent, error)
}

// NewPaymentEventRepo builds a PaymentEventRepo over the given Querier (pool
// or transaction).
func NewPaymentEventRepo(q sqlc.Querier) PaymentEventRepo {
	return &paymentEventRepo{q: q}
}

type paymentEventRepo struct {
	q sqlc.Querier
}

func (r *paymentEventRepo) Enqueue(ctx context.Context, depositID, eventID uuid.UUID, eventType string, idempotencyKey string, payload []byte) (sqlc.PaymentEvent, error) {
	event, err := r.q.InsertPaymentEvent(ctx, sqlc.InsertPaymentEventParams{
		DepositID:      depositID,
		EventID:        eventID,
		EventType:      eventType,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	})
	if err != nil {
		return sqlc.PaymentEvent{}, wrapError(err)
	}
	return event, nil
}

func (r *paymentEventRepo) ClaimDue(ctx context.Context, limit int32, maxAttempts int32) ([]sqlc.PaymentEvent, error) {
	events, err := r.q.ClaimDuePaymentEvents(ctx, sqlc.ClaimDuePaymentEventsParams{
		Limit:    limit,
		Attempts: maxAttempts,
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return events, nil
}

func (r *paymentEventRepo) MarkDelivered(ctx context.Context, id uuid.UUID) (sqlc.PaymentEvent, error) {
	event, err := r.q.RecordPaymentEventSuccess(ctx, id)
	if err != nil {
		return sqlc.PaymentEvent{}, wrapNotFound(err)
	}
	return event, nil
}

func (r *paymentEventRepo) ScheduleRetry(ctx context.Context, id uuid.UUID, lastError string, backoff time.Duration) (sqlc.PaymentEvent, error) {
	event, err := r.q.RecordPaymentEventRetry(ctx, sqlc.RecordPaymentEventRetryParams{
		ID:        id,
		LastError: textRef(lastError),
		Column3:   backoff.Seconds(),
	})
	if err != nil {
		return sqlc.PaymentEvent{}, wrapNotFound(err)
	}
	return event, nil
}

func (r *paymentEventRepo) MarkFailed(ctx context.Context, id uuid.UUID, lastError string) (sqlc.PaymentEvent, error) {
	event, err := r.q.RecordPaymentEventFailure(ctx, sqlc.RecordPaymentEventFailureParams{
		ID:        id,
		LastError: textRef(lastError),
	})
	if err != nil {
		return sqlc.PaymentEvent{}, wrapNotFound(err)
	}
	return event, nil
}

func (r *paymentEventRepo) GetByDepositID(ctx context.Context, depositID uuid.UUID) (sqlc.PaymentEvent, error) {
	event, err := r.q.GetPaymentEventByDepositID(ctx, depositID)
	if err != nil {
		return sqlc.PaymentEvent{}, wrapNotFound(err)
	}
	return event, nil
}

package ghldeliver

import (
	"context"
	"errors"
	"time"

	"github.com/MountainHubTech/rvpay-go/transactions/db/repo"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/rs/zerolog"
)

// MaxDeliveryAttempts is the bounded total number of delivery attempts per
// event (one initial attempt plus retries). Exhausted events are recorded
// 'failed' and never re-claimed.
const MaxDeliveryAttempts int32 = 5

// DefaultPollInterval is the pause between outbox polls when empty.
const DefaultPollInterval = 10 * time.Second

// retryBackoffs are the delays applied after attempt n fails transiently
// (index n-1). The schedule is bounded: 1m, 5m, 15m, 60m.
var retryBackoffs = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	60 * time.Minute,
}

// Worker is the durable HighLevel inbound-webhook delivery worker. It polls
// the payment_events outbox (delivery_status='pending'), claims due rows
// atomically, and POSTs the immutable payload snapshot to the configured
// HIGHLEVEL_INBOUND_WEBHOOK_URL.
//
// Retry behavior (bounded):
//   - transient failure (timeout / network / HTTP 5xx / 408 / 429) with
//     attempts < MaxDeliveryAttempts -> re-queued 'pending' with an
//     exponential backoff (1m, 5m, 15m, 60m);
//   - attempts exhausted -> recorded 'failed' with the last error;
//   - permanent HTTP 4xx -> recorded 'failed' immediately; ordinary
//     configuration problems never become an infinite retry loop.
//
// Every attempt reuses the same event id, idempotency key and payload bytes
// from the outbox row. An event is marked delivered only after a 2xx
// response. Shutdown is cooperative via ctx cancellation. No job framework,
// no broker, no Redis.
type Worker struct {
	outboxRepo repo.PaymentEventRepo
	// transactionsRepo is used only to open the short claim transaction (FOR
	// UPDATE SKIP LOCKED). Delivery happens strictly after the claim commit.
	transactionsRepo repo.TransactionsRepo
	poster           Poster
	logger           zerolog.Logger
	pollInterval     time.Duration
}

// NewWorker creates the delivery worker. webhookURL is the configured
// HIGHLEVEL_INBOUND_WEBHOOK_URL; when it is missing or not a valid HTTPS URL
// the worker runs safely disabled — it logs the configuration variable NAME
// (never the value) and leaves events pending, so payments are never affected
// and delivery resumes after the configuration is corrected and the service
// is restarted.
func NewWorker(outboxRepo repo.PaymentEventRepo, transactionsRepo repo.TransactionsRepo, webhookURL string, logger zerolog.Logger, pollInterval time.Duration) (*Worker, error) {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	w := &Worker{
		outboxRepo:       outboxRepo,
		transactionsRepo: transactionsRepo,
		logger:           logger,
		pollInterval:     pollInterval,
	}
	if webhookURL == "" {
		logger.Warn().Str("config", "HIGHLEVEL_INBOUND_WEBHOOK_URL").Msg("highlevel inbound webhook delivery is disabled: HIGHLEVEL_INBOUND_WEBHOOK_URL is not configured; events remain queued and payments are unaffected")
		return w, nil
	}
	if !ValidateInboundWebhookURL(webhookURL) {
		logger.Warn().Str("config", "HIGHLEVEL_INBOUND_WEBHOOK_URL").Msg("highlevel inbound webhook delivery is disabled: HIGHLEVEL_INBOUND_WEBHOOK_URL is not a valid HTTPS URL; events remain queued and payments are unaffected")
		return w, nil
	}
	w.poster = NewHTTPPoster(webhookURL, DefaultTimeout)
	return w, nil
}

// Run polls and delivers until ctx is cancelled. It blocks; run it in a
// goroutine alongside the servers.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Info().Msg("highlevel webhook delivery worker started")
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		if err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error().Err(err).Msg("highlevel webhook delivery poll failed")
		}
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("highlevel webhook delivery worker stopped")
			return
		case <-ticker.C:
		}
	}
}

// RunOnce claims and processes one batch of due events.
func (w *Worker) RunOnce(ctx context.Context) error {
	events, err := w.claim(ctx)
	if err != nil {
		return err
	}
	for _, event := range events {
		w.deliver(ctx, event)
	}
	return nil
}

// claim atomically claims due pending events inside a short transaction. FOR
// UPDATE SKIP LOCKED prevents two concurrent workers from claiming the same
// row. When the worker is safely disabled (no poster), nothing is claimed.
func (w *Worker) claim(ctx context.Context) ([]sqlc.PaymentEvent, error) {
	if w.poster == nil {
		return nil, nil
	}
	txQuerier, tx, err := w.transactionsRepo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && ctx.Err() == nil {
			w.logger.Error().Err(rbErr).Msg("could not roll back outbox claim transaction")
		}
	}()

	claimed, err := repo.NewPaymentEventRepo(txQuerier).ClaimDue(ctx, 10, MaxDeliveryAttempts)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return claimed, nil
}

// deliver performs one delivery attempt for a claimed event. The event id,
// idempotency key and payload are taken from the outbox row and are never
// regenerated; an event is marked delivered only after a successful response.
func (w *Worker) deliver(ctx context.Context, event sqlc.PaymentEvent) {
	eventID := event.EventID.String()

	err := w.poster.Post(ctx, eventID, event.Payload)
	var deliveryErr *DeliveryError
	switch {
	case err == nil:
		if _, recErr := w.outboxRepo.MarkDelivered(ctx, event.ID); recErr != nil {
			w.logger.Error().Err(recErr).
				Str("event_id", eventID).
				Str("deposit_id", event.DepositID.String()).
				Msg("highlevel webhook delivered but success could not be recorded; delivery will be retried with the same event id")
			return
		}
		w.logger.Info().
			Str("event_id", eventID).
			Str("deposit_id", event.DepositID.String()).
			Str("event", event.EventType).
			Int32("attempt", event.Attempts).
			Msg("highlevel inbound webhook delivered")
	case errors.As(err, &deliveryErr) && deliveryErr.Kind == FailurePermanent:
		w.logger.Error().
			Str("event_id", eventID).
			Str("deposit_id", event.DepositID.String()).
			Int("http_status", deliveryErr.HTTPStatus).
			Int32("attempt", event.Attempts).
			Str("delivery_result", "permanent_failure").
			Msg("highlevel webhook delivery failed permanently; stopping retries for this event")
		if _, recErr := w.outboxRepo.MarkFailed(ctx, event.ID, deliveryErr.Error()); recErr != nil {
			w.logger.Error().Err(recErr).Str("event_id", eventID).Msg("could not record highlevel webhook permanent failure")
		}
	default:
		backoff := w.backoffAfter(event.Attempts)
		w.logger.Warn().
			Str("event_id", eventID).
			Str("deposit_id", event.DepositID.String()).
			Int32("attempt", event.Attempts).
			Str("delivery_result", "retry_scheduled").
			Int64("retry_in_seconds", int64(backoff.Seconds())).
			Msg("highlevel webhook delivery failed transiently; reusing the same event id for the retry")
		if _, recErr := w.outboxRepo.ScheduleRetry(ctx, event.ID, sanitizedDeliveryError(err), backoff); recErr != nil {
			w.logger.Error().Err(recErr).Str("event_id", eventID).Msg("could not schedule highlevel webhook retry")
		}
	}
}

// backoffAfter returns the delay applied after the given attempt number
// failed transiently. Attempts beyond the schedule reuse the longest backoff.
func (w *Worker) backoffAfter(attempts int32) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	idx := int(attempts) - 1
	if idx >= len(retryBackoffs) {
		idx = len(retryBackoffs) - 1
	}
	return retryBackoffs[idx]
}

// sanitizedDeliveryError keeps the URL-free message from DeliveryError; any
// other error type is reduced to a generic label so the configured webhook
// URL (which net/http embeds in url.Error messages) can never reach the logs.
func sanitizedDeliveryError(err error) string {
	var deliveryErr *DeliveryError
	if errors.As(err, &deliveryErr) {
		return deliveryErr.Error()
	}
	return "highlevel webhook delivery failed (transport error)"
}

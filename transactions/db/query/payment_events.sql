-- name: InsertPaymentEvent :one
-- Idempotent emission: one logical event per deposit. A concurrent duplicate
-- callback either loses the terminal-state race (ErrNotFound on finalize) or
-- is a no-op here. Returns an empty row when the event already exists.
INSERT INTO payment_events (deposit_id, event_id, event_type, idempotency_key, payload)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (deposit_id) DO NOTHING
RETURNING *;

-- name: ClaimDuePaymentEvents :many
-- Atomically claims due, non-exhausted pending events for delivery and
-- increments the attempt count. FOR UPDATE + SKIP LOCKED prevents two
-- concurrent workers from claiming the same row.
UPDATE payment_events AS pe
SET attempts = pe.attempts + 1,
    updated_at = NOW()
WHERE pe.id IN (
    SELECT inner_pe.id FROM payment_events AS inner_pe
    WHERE inner_pe.delivery_status = 'pending'
      AND inner_pe.attempts < $2
      AND inner_pe.next_retry_at <= NOW()
    ORDER BY inner_pe.created_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING pe.*;

-- name: RecordPaymentEventSuccess :one
UPDATE payment_events
SET delivery_status = 'delivered',
    last_error = NULL,
    delivered_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RecordPaymentEventRetry :one
-- Schedules a transient failure for retry at the caller-computed backoff time.
-- The event identity (event_id, idempotency_key, payload) is never changed.
UPDATE payment_events
SET delivery_status = 'pending',
    last_error = $2,
    next_retry_at = NOW() + make_interval(secs => $3::double precision),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RecordPaymentEventFailure :one
-- Records a permanent (non-retryable) failure so a configuration problem
-- cannot become an infinite retry loop. Payload and identity are preserved.
UPDATE payment_events
SET delivery_status = 'failed',
    last_error = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetPaymentEventByDepositID :one
SELECT * FROM payment_events WHERE deposit_id = $1;

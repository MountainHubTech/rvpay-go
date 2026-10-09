-- name: CreatePayout :one
INSERT INTO payouts (
    client_id,
    merchant_id,
    amount,
    currency,
    provider,
    destination_reference,
    status,
    idempotency_key
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPayoutByID :one
SELECT * FROM payouts WHERE id = $1;

-- name: GetPayoutByExternalReference :one
SELECT * FROM payouts WHERE external_reference = $1;

-- name: GetPayoutByIdempotencyKey :one
SELECT * FROM payouts WHERE idempotency_key = $1;

-- name: ListPayoutsByClient :many
SELECT * FROM payouts
WHERE client_id = $1
ORDER BY created_at DESC;

-- name: ListPayoutsByMerchant :many
SELECT * FROM payouts
WHERE merchant_id = $1
ORDER BY created_at DESC;

-- name: ListPayoutsByStatus :many
SELECT * FROM payouts
WHERE status = $1
ORDER BY created_at DESC;

-- name: UpdatePayoutStatus :one
UPDATE payouts
SET status = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- Payout overview aggregates (for /v1/public/payouts/overview/stats and the
-- overview snapshot). All money is stored as NUMERIC(18,2); sums are returned
-- as int64 minor-unit values by the caller's choice — here we return the raw
-- numeric sum and let the service format it. Status filtering uses the
-- payout_status enum values: REQUESTED, PROCESSING, COMPLETED, FAILED.

-- name: CountPayoutsByStatus :one
SELECT COUNT(*) FROM payouts WHERE status = $1;

-- name: SumPayoutAmountByStatus :one
SELECT COALESCE(SUM(amount), 0) FROM payouts WHERE status = $1;

-- name: CountPayoutsInWindow :one
SELECT COUNT(*) FROM payouts WHERE created_at >= $1;

-- name: ListPayoutsFiltered :many
SELECT *
FROM payouts
WHERE ($1::TEXT = '' OR destination_reference ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR status = $2::payout_status)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountPayoutsFiltered :one
SELECT COUNT(*)
FROM payouts
WHERE ($1::TEXT = '' OR destination_reference ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR status = $2::payout_status);

-- name: UpdatePayoutStatusAndCompletedAt :one
UPDATE payouts
SET status = $2,
    completed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdatePayoutStatusAndFailedAt :one
UPDATE payouts
SET status = $2,
    failed_at = NOW(),
    failure_reason = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;
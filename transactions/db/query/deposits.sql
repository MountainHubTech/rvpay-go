-- name: CreateDeposit :one
INSERT INTO deposits (
    client_name,
    customer_id,
    merchant_id,
    amount,
    currency,
    payment_type,
    payer_phone_number,
    provider,
    status,
    idempotency_key,
    ghl_transaction_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetDepositByID :one
SELECT * FROM deposits WHERE id = $1;

-- name: GetDepositByExternalReference :one
SELECT * FROM deposits WHERE external_reference = $1;

-- name: GetDepositByGHLTransactionID :one
SELECT * FROM deposits WHERE ghl_transaction_id = $1;

-- name: GetDepositByGHLChargeID :one
SELECT * FROM deposits WHERE ghl_charge_id = $1;

-- name: GetDepositByIdempotencyKey :one
SELECT * FROM deposits WHERE idempotency_key = $1;

-- name: ListDepositsByClient :many
SELECT * FROM deposits
WHERE client_name = $1
ORDER BY created_at DESC;

-- name: ListDepositsByCustomer :many
SELECT * FROM deposits
WHERE customer_id = $1
ORDER BY created_at DESC;

-- name: ListDepositsByMerchant :many
SELECT * FROM deposits
WHERE merchant_id = $1
ORDER BY created_at DESC;

-- name: ListDepositsByStatus :many
SELECT * FROM deposits
WHERE status = $1
ORDER BY created_at DESC;

-- name: UpdateDepositStatus :one
UPDATE deposits
SET status = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateDepositStatusAndCompletedAt :one
UPDATE deposits
SET status = $2,
    completed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateDepositStatusAndFailedAt :one
UPDATE deposits
SET status = $2,
    failed_at = NOW(),
    failure_reason = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- Overview-snapshot aggregates. Deposits drive revenue + volume; payouts are
-- queried separately. All money is NUMERIC(18,2).

-- name: SumDepositAmountInWindow :one
SELECT COALESCE(SUM(amount), 0)
FROM deposits
WHERE status = 'COMPLETED'
  AND created_at >= $1;

-- name: CountDepositsInWindow :one
SELECT COUNT(*)
FROM deposits
WHERE created_at >= $1;

-- name: RevenueOverTimeInWindow :many
-- Returns per-day revenue buckets for the window. The bucket label is a
-- calendar date; revenue is the raw numeric sum (service formats to minor
-- units). Buckets with no deposits are omitted (no zero-filling), matching a
-- sparse time series.
SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS period_label,
       COALESCE(SUM(amount), 0) AS revenue
FROM deposits
WHERE status = 'COMPLETED'
  AND created_at >= $1
GROUP BY period_label
ORDER BY period_label;

-- name: ListRecentDeposits :many
SELECT *
FROM deposits
ORDER BY created_at DESC
LIMIT $1;

-- name: ListDepositsFiltered :many
SELECT *
FROM deposits
WHERE ($1::TEXT = '' OR client_name ILIKE '%' || $1 || '%' OR customer_id ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR status = $2::deposit_status)
  AND ($3::TEXT = '' OR client_name = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountDepositsFiltered :one
SELECT COUNT(*)
FROM deposits
WHERE ($1::TEXT = '' OR client_name ILIKE '%' || $1 || '%' OR customer_id ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR status = $2::deposit_status)
  AND ($3::TEXT = '' OR client_name = $3);

-- name: UpdateDepositExternalReference :exec
UPDATE deposits
SET external_reference = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateDepositGHLReference :one
UPDATE deposits
SET ghl_transaction_id = $2,
    ghl_charge_id = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

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
    ghl_transaction_id,
    ghl_order_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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

-- name: FinalizeDepositAndQueueGhlSync :one
-- Atomically transitions a non-terminal deposit to a terminal PawaPay state
-- (COMPLETED/FAILED) AND enqueues the server-side GHL synchronization intent
-- (ghl_sync_status = 'pending') when a GHL order id is present. A single
-- statement IS one database transaction: the deposit status update and the
-- queue insertion commit together and the external GHL call is never made
-- inside this statement. PROCESSING callbacks leave the deposit non-terminal
-- and therefore never enqueue a final GHL update.
UPDATE deposits
SET status = $2,
    completed_at = CASE WHEN $2::deposit_status = 'COMPLETED' THEN NOW() ELSE completed_at END,
    failed_at = CASE WHEN $2::deposit_status = 'FAILED' THEN NOW() ELSE failed_at END,
    failure_reason = CASE WHEN $2::deposit_status = 'FAILED' THEN $3 ELSE failure_reason END,
    ghl_sync_status = CASE
        WHEN ghl_order_id IS NULL OR ghl_order_id = '' THEN 'none'
        WHEN $2::deposit_status IN ('COMPLETED', 'FAILED') THEN 'pending'
        ELSE 'none' END,
    ghl_sync_attempts = CASE
        WHEN $2::deposit_status IN ('COMPLETED', 'FAILED') THEN 0 ELSE ghl_sync_attempts END,
    ghl_sync_last_error = CASE
        WHEN $2::deposit_status IN ('COMPLETED', 'FAILED') THEN NULL ELSE ghl_sync_last_error END,
    updated_at = NOW()
WHERE id = $1
  AND status IN ('INITIATED', 'PROCESSING')
RETURNING *;

-- name: ClaimPendingGhlSync :many
-- Atomically claims all currently-pending GHL synchronizations for processing
-- and increments the attempt count. Two concurrent workers cannot claim the
-- same row: the UPDATE takes a row lock and re-evaluates the WHERE on the
-- updated row, so a row claimed by one worker no longer matches
-- ghl_sync_status = 'pending' for the other. Only deposits with fewer than two
-- attempts are claimed (two total attempts).
UPDATE deposits
SET ghl_sync_status = 'processing',
    ghl_sync_attempts = ghl_sync_attempts + 1,
    updated_at = NOW()
WHERE ghl_sync_status = 'pending'
  AND ghl_order_id IS NOT NULL
  AND ghl_order_id <> ''
  AND ghl_sync_attempts < 2
RETURNING *;

-- name: RecordGhlSyncSuccess :one
UPDATE deposits
SET ghl_sync_status = 'completed',
    ghl_sync_last_error = NULL,
    ghl_sync_failed_at = NULL,
    ghl_synced_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: RecordGhlSyncRetry :one
-- Re-queues a failed GHL update for its single retry (attempts < 2). The
-- authoritative PawaPay deposit status is never touched.
UPDATE deposits
SET ghl_sync_status = 'pending',
    ghl_sync_last_error = $2,
    updated_at = NOW()
WHERE id = $1
  AND ghl_sync_attempts < 2
RETURNING *;

-- name: RecordGhlSyncFailure :one
-- Records the final GHL synchronization failure after two attempts without
-- altering the authoritative PawaPay terminal deposit status.
UPDATE deposits
SET ghl_sync_status = 'failed',
    ghl_sync_last_error = $2,
    ghl_sync_failed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListDisputesFiltered :many
SELECT id,
       deposit_id,
       client_name,
       dispute_type,
       amount,
       currency,
       status,
       opened_at,
       due_at
FROM disputes
WHERE ($1::TEXT = '' OR client_name ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR status = $2::dispute_status)
ORDER BY opened_at DESC
LIMIT $3 OFFSET $4;

-- name: CountDisputesFiltered :one
SELECT COUNT(*)
FROM disputes
WHERE ($1::TEXT = '' OR client_name ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR status = $2::dispute_status);

-- name: GetDisputeStats :one
SELECT COUNT(*) FILTER (WHERE status = 'NEEDS_RESPONSE') AS needs_response,
       COUNT(*) FILTER (WHERE status = 'UNDER_REVIEW')  AS under_review
FROM disputes;

-- name: GetDisputeByID :one
SELECT id,
       deposit_id,
       client_name,
       dispute_type,
       amount,
       currency,
       status,
       opened_at,
       due_at,
       evidence_submitted,
       resolved_at
FROM disputes
WHERE id = $1;

-- name: InsertDispute :one
INSERT INTO disputes (deposit_id, client_name, dispute_type, amount, currency, status, due_at)
VALUES ($1, $2, $3, $4, $5, 'NEEDS_RESPONSE'::dispute_status, $6)
RETURNING *;

-- name: SubmitEvidence :one
UPDATE disputes
SET evidence_submitted = true,
    status = 'UNDER_REVIEW',
    resolved_at = NULL,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

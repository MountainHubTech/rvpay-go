-- 000006_ghl_order_sync.up.sql
-- Persists the GoHighLevel order identifier on the deposit and the metadata
-- used to drive the server-side GHL order/payment status synchronization.
--
-- Background: PawaPay is authoritative for the RVPay deposit's final state
-- (COMPLETED/FAILED), but the corresponding GHL order used to remain stuck in
-- "pending" because RVPay never stored the GHL orderId and had no outbound
-- call to update the GHL order. This migration adds:
--   ghl_order_id         the GHL order id captured from the payment iframe.
--   ghl_sync_status      'none' | 'pending' | 'processing' | 'completed' |
--                        'failed' (the durable synchronization queue state).
--   ghl_sync_attempts    number of GHL update attempts already made.
--   ghl_sync_last_error  the last GHL synchronization failure message.
--   ghl_sync_failed_at   when the final (2nd) attempt failed.
--   ghl_synced_at        when GHL confirmed the update.
--
-- Index decision: ghl_order_id is NOT globally unique across every GHL
-- location, so no UNIQUE constraint is added. The RVPay client_name already
-- carries the location ("highlevel-<locationId>"); a non-unique index on
-- ghl_order_id is enough for correlation and reconciliation. Queue claims also
-- scan ghl_sync_status, which is covered by the worker's targeted UPDATE.

ALTER TABLE deposits
    ADD COLUMN ghl_order_id TEXT,
    ADD COLUMN ghl_sync_status TEXT NOT NULL DEFAULT 'none',
    ADD COLUMN ghl_sync_attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN ghl_sync_last_error TEXT,
    ADD COLUMN ghl_sync_failed_at TIMESTAMPTZ,
    ADD COLUMN ghl_synced_at TIMESTAMPTZ;

CREATE INDEX idx_deposits_ghl_order_id ON deposits (ghl_order_id);
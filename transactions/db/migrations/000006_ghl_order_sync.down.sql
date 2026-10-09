-- 000006_ghl_order_sync.down.sql
-- Reverses 000006 by removing the GHL order synchronization columns. The
-- down migration is only safe when no deployment depends on the sync metadata.

DROP INDEX IF EXISTS idx_deposits_ghl_order_id;

ALTER TABLE deposits
    DROP COLUMN ghl_order_id,
    DROP COLUMN ghl_sync_status,
    DROP COLUMN ghl_sync_attempts,
    DROP COLUMN ghl_sync_last_error,
    DROP COLUMN ghl_sync_failed_at,
    DROP COLUMN ghl_synced_at;
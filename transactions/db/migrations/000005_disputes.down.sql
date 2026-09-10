DROP INDEX IF EXISTS idx_disputes_opened_at;
DROP INDEX IF EXISTS idx_disputes_client_name;
DROP INDEX IF EXISTS idx_disputes_status;
DROP INDEX IF EXISTS idx_disputes_deposit_id;
DROP TABLE IF EXISTS disputes;
DROP TYPE IF EXISTS dispute_status;

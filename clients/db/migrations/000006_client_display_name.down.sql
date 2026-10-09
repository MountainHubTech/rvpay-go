-- 000006_client_display_name.down.sql
-- Reverses 000006_client_display_name.up.sql. Only the derived display name is
-- removed; clients.client_name (the correlation identifier used by the
-- Transactions service) and every client id / external_account_id are
-- untouched.

ALTER TABLE clients
    DROP COLUMN IF EXISTS display_name;

-- 000006_client_display_name.up.sql
-- Adds the authoritative HighLevel location/sub-account DISPLAY name for a
-- client.
--
-- Column decision (why a new column instead of reusing client_name):
--   clients.client_name is the deterministic correlation identifier
--   ("highlevel-<locationId>"). It is the value the Admin Dashboard sends as
--   the Transactions sub-account filter (ListDepositsFiltered matches
--   deposits.client_name exactly), and the Transactions service derives the
--   HighLevel locationId from it (transactions/ghlsync, transactions/ghldeliver).
--   Repurposing it for display would break that correlation. The human-readable
--   location name therefore lives in display_name while client_name stays the
--   identifier.
--
-- No name is fabricated by this migration: the column starts NULL and stays
-- NULL until the application fetches the authoritative name from the
-- authenticated HighLevel location GET (GET /locations/{locationId}, which
-- requires the locations.readonly OAuth scope). A NULL display_name is
-- rendered as the existing client_name by the dashboard, never as an error or
-- an invented value.

ALTER TABLE clients
    ADD COLUMN display_name TEXT;

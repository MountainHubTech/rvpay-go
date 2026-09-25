-- name: CreateClient :one
INSERT INTO clients (client_name, status)
VALUES ($1, $2)
RETURNING *;

-- name: GetClientByID :one
SELECT id, client_name, status, created_at, updated_at, display_name
FROM clients
WHERE id = $1;

-- name: GetClientByName :one
SELECT id, client_name, status, created_at, updated_at, display_name
FROM clients
WHERE client_name = $1;

-- name: ListClients :many
SELECT id, client_name, status, created_at, updated_at, display_name
FROM clients
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListActiveClients :many
SELECT id, client_name, status, created_at, updated_at, display_name
FROM clients
WHERE status = 'ACTIVE'
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountClients :one
SELECT COUNT(*) FROM clients;

-- name: CountSubAccountsFiltered :one
SELECT COUNT(*)
FROM clients c
LEFT JOIN integrations i ON i.client_id = c.id
WHERE ($1::TEXT = '' OR c.client_name ILIKE '%' || $1 || '%' OR c.display_name ILIKE '%' || $1 || '%' OR i.external_account_id ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR c.status = $2::client_status);

-- name: ListSubAccountsFiltered :many
-- display_name is a nullable column (see migration 000006), so it is coalesced
-- with the always-present client_name (the deterministic highlevel-<locationId>
-- identifier) here. The same COALESCE/NULLIF expression is used for ordering
-- below, and converters.subAccountRowToProto keeps its identical fallback, so
-- the repository never scans a SQL NULL into the non-null Go string field.
-- external_account_id comes from a LEFT JOIN and is equally NULL for a client
-- with no integration row; it is coalesced to '' for the same reason (the
-- converter already treats an empty location as "no location").
SELECT c.id, c.client_name, c.status, c.created_at,
       COALESCE(NULLIF(c.display_name, ''), c.client_name) AS display_name,
       COALESCE(i.external_account_id, '') AS external_account_id
FROM clients c
LEFT JOIN integrations i ON i.client_id = c.id
WHERE ($1::TEXT = '' OR c.client_name ILIKE '%' || $1 || '%' OR c.display_name ILIKE '%' || $1 || '%' OR i.external_account_id ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR c.status = $2::client_status)
ORDER BY
  CASE WHEN $3::TEXT = 'name' AND $4::TEXT = 'asc' THEN COALESCE(NULLIF(c.display_name, ''), c.client_name) END ASC,
  CASE WHEN $3::TEXT = 'name' AND $4::TEXT = 'desc' THEN COALESCE(NULLIF(c.display_name, ''), c.client_name) END DESC,
  CASE WHEN $3::TEXT = 'name' THEN COALESCE(NULLIF(c.display_name, ''), c.client_name) END ASC,
  CASE WHEN $4::TEXT = 'desc' THEN c.created_at END DESC,
  c.created_at ASC
LIMIT $5 OFFSET $6;

-- name: ClientExistsByID :one
SELECT EXISTS(
    SELECT 1 FROM clients WHERE id = $1
);

-- name: UpdateClientStatus :one
UPDATE clients
SET status = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, client_name, status, created_at, updated_at, display_name;

-- name: UpdateClientDisplayName :one
-- Persists the authoritative HighLevel location/sub-account name for a client.
-- Guarded so the operation is idempotent and can never destroy a valid name
-- or degrade a real name into an identifier/error string:
--   * only a missing (NULL) or empty display_name is filled;
--   * an already-populated display_name is left untouched (0 rows returned);
--   * client_name, the client id and external_account_id are never modified.
UPDATE clients
SET display_name = $2,
    updated_at = NOW()
WHERE id = $1
  AND (display_name IS NULL OR display_name = '')
RETURNING id, client_name, status, created_at, updated_at, display_name;

-- name: ListClientsNeedingDisplayName :many
-- Backfill/reconciliation source: HighLevel clients whose authoritative
-- display name has not been persisted yet. Only clients with a real
-- HighLevel integration mapping (integrations.external_account_id = GHL
-- locationId) are returned, so the backfill never attempts a client it cannot
-- resolve a location for. Idempotent by construction: a client whose
-- display_name is populated by a previous run is no longer returned.
SELECT c.id, c.client_name, i.external_account_id
FROM clients c
JOIN integrations i ON i.client_id = c.id
JOIN platforms p ON p.id = i.platform_id
WHERE p.slug = 'highlevel'
  AND i.external_account_id IS NOT NULL
  AND i.external_account_id <> ''
  AND (c.display_name IS NULL OR c.display_name = '')
ORDER BY c.created_at ASC
LIMIT $1 OFFSET $2;

-- name: DeleteClient :execrows
DELETE FROM clients WHERE id = $1;
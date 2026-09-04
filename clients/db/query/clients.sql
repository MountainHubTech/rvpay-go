-- name: CreateClient :one
INSERT INTO clients (client_name, status)
VALUES ($1, $2)
RETURNING *;

-- name: GetClientByID :one
SELECT id, client_name, status, created_at, updated_at
FROM clients
WHERE id = $1;

-- name: GetClientByName :one
SELECT id, client_name, status, created_at, updated_at
FROM clients
WHERE client_name = $1;

-- name: ListClients :many
SELECT id, client_name, status, created_at, updated_at
FROM clients
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListActiveClients :many
SELECT id, client_name, status, created_at, updated_at
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
WHERE ($1::TEXT = '' OR c.client_name ILIKE '%' || $1 || '%' OR i.external_account_id ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR c.status = $2::client_status);

-- name: ListSubAccountsFiltered :many
SELECT c.id, c.client_name, c.status, c.created_at,
       i.external_account_id
FROM clients c
LEFT JOIN integrations i ON i.client_id = c.id
WHERE ($1::TEXT = '' OR c.client_name ILIKE '%' || $1 || '%' OR i.external_account_id ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR c.status = $2::client_status)
ORDER BY
  CASE WHEN $3::TEXT = 'name' AND $4::TEXT = 'asc' THEN c.client_name END ASC,
  CASE WHEN $3::TEXT = 'name' AND $4::TEXT = 'desc' THEN c.client_name END DESC,
  CASE WHEN $3::TEXT = 'name' THEN c.client_name END ASC,
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
RETURNING id, client_name, status, created_at, updated_at;

-- name: DeleteClient :execrows
DELETE FROM clients WHERE id = $1;
-- name: CreateAccessToken :one
INSERT INTO access_tokens (user_id, token_hash, refresh_token_hash, expires_at, refresh_expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAccessTokenByTokenHash :one
SELECT at.*
FROM access_tokens at
INNER JOIN users u ON u.id = at.user_id
WHERE at.token_hash = $1
  AND at.expires_at > NOW()
  AND u.user_role = 'admin';

-- name: GetAccessTokenByRefreshTokenHash :one
SELECT at.*
FROM access_tokens at
INNER JOIN users u ON u.id = at.user_id
WHERE at.refresh_token_hash = $1
  AND at.refresh_expires_at > NOW()
  AND u.user_role = 'admin'
ORDER BY at.created_at DESC
LIMIT 1;

-- name: DeleteAccessTokenByTokenHash :exec
DELETE FROM access_tokens WHERE token_hash = $1;

-- name: DeleteExpiredAccessTokens :execrows
DELETE FROM access_tokens WHERE refresh_expires_at <= NOW();
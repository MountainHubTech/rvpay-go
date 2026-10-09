-- name: CreateUser :one
INSERT INTO users (name, email, password_hash, user_role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUserRefreshTokenHash :exec
UPDATE users
SET refresh_token_hash = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ClearUserRefreshTokenHash :exec
UPDATE users
SET refresh_token_hash = '',
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateUserNameEmail :one
UPDATE users
SET name = $2,
    email = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPasswordHash :one
UPDATE users
SET password_hash = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListUsers :many
SELECT id,
       name,
       email,
       user_role,
       refresh_token_hash,
       created_at
FROM users
WHERE ($1::TEXT = '' OR name ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR user_role = $2::user_role)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountUsers :one
SELECT COUNT(*)
FROM users
WHERE ($1::TEXT = '' OR name ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%')
  AND ($2::TEXT = '' OR user_role = $2::user_role);
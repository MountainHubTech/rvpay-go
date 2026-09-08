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
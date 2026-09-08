package auth

import "errors"

var (
	// ErrInvalidCredentials is returned when the email/password pair does
	// not match an administrator record.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrInvalidToken is returned when a bearer or refresh token is missing,
	// unknown, expired or not backed by an administrator.
	ErrInvalidToken = errors.New("invalid or expired token")
	// ErrEmailExists is returned when bootstrap finds the seed email
	// already present (idempotent seeding).
	ErrEmailExists = errors.New("administrator already exists")
)

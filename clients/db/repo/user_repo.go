package repo

import (
	"context"

	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
)

// UserRepo provides persistence operations for database-managed users.
// Users (including administrators) are created through the gRPC
// CreateUser method; the database is the single source of truth for who
// the administrators are.
type UserRepo interface {
	Create(ctx context.Context, name, email string, passwordHash string, userRole sqlc.UserRole) (sqlc.User, error)
	GetByEmail(ctx context.Context, email string) (sqlc.User, error)
	GetByID(ctx context.Context, userID uuid.UUID) (sqlc.User, error)
	UpdateNameEmail(ctx context.Context, userID uuid.UUID, name, email string) (sqlc.User, error)
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) (sqlc.User, error)
	UpdateRefreshTokenHash(ctx context.Context, userID uuid.UUID, refreshTokenHash string) error
	ClearRefreshTokenHash(ctx context.Context, userID uuid.UUID) error
}

type userRepo struct {
	q sqlc.Querier
}

// NewUserRepo creates a user repository backed by the given querier.
func NewUserRepo(q sqlc.Querier) UserRepo {
	return &userRepo{q: q}
}

func (r *userRepo) Create(ctx context.Context, name, email string, passwordHash string, userRole sqlc.UserRole) (sqlc.User, error) {
	record, err := r.q.CreateUser(ctx, sqlc.CreateUserParams{
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		UserRole:     userRole,
	})
	if err != nil {
		return sqlc.User{}, wrapError(err)
	}
	return record, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (sqlc.User, error) {
	record, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return sqlc.User{}, wrapNotFound(err)
	}
	return record, nil
}

func (r *userRepo) GetByID(ctx context.Context, userID uuid.UUID) (sqlc.User, error) {
	record, err := r.q.GetUserByID(ctx, userID)
	if err != nil {
		return sqlc.User{}, wrapNotFound(err)
	}
	return record, nil
}

// UpdateNameEmail updates only the display name and email of a user. The
// user's role, password and token state are intentionally untouched.
func (r *userRepo) UpdateNameEmail(ctx context.Context, userID uuid.UUID, name, email string) (sqlc.User, error) {
	record, err := r.q.UpdateUserNameEmail(ctx, sqlc.UpdateUserNameEmailParams{
		ID:    userID,
		Name:  name,
		Email: email,
	})
	if err != nil {
		return sqlc.User{}, wrapError(err)
	}
	return record, nil
}

// UpdatePasswordHash replaces the stored Argon2id password hash. The
// plaintext password is never passed here.
func (r *userRepo) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) (sqlc.User, error) {
	record, err := r.q.UpdateUserPasswordHash(ctx, sqlc.UpdateUserPasswordHashParams{
		ID:           userID,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return sqlc.User{}, wrapError(err)
	}
	return record, nil
}

func (r *userRepo) UpdateRefreshTokenHash(ctx context.Context, userID uuid.UUID, refreshTokenHash string) error {
	err := r.q.UpdateUserRefreshTokenHash(ctx, sqlc.UpdateUserRefreshTokenHashParams{
		ID:               userID,
		RefreshTokenHash: refreshTokenHash,
	})
	return wrapError(err)
}

func (r *userRepo) ClearRefreshTokenHash(ctx context.Context, userID uuid.UUID) error {
	err := r.q.ClearUserRefreshTokenHash(ctx, userID)
	return wrapError(err)
}

package repo

import (
	"context"

	"github.com/I-Frostbyte/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
)

// UserRepo provides persistence operations for database-managed users.
// Users (including administrators) are created directly in the database by
// the operator; the authentication service only reads and updates them.
type UserRepo interface {
	Create(ctx context.Context, name, email string, passwordHash string, userRole sqlc.UserRole) (sqlc.User, error)
	GetByEmail(ctx context.Context, email string) (sqlc.User, error)
	GetByID(ctx context.Context, userID uuid.UUID) (sqlc.User, error)
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

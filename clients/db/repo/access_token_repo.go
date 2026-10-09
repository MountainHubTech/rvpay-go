package repo

import (
	"context"
	"time"

	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
)

// AccessTokenRepo provides persistence operations for administrator access
// tokens. Only SHA-256 hashes of tokens are ever persisted.
type AccessTokenRepo interface {
	Create(ctx context.Context, userID uuid.UUID, tokenHash, refreshTokenHash string, expiresAt, refreshExpiresAt time.Time) (sqlc.AccessToken, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (sqlc.AccessToken, error)
	GetByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (sqlc.AccessToken, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	DeleteExpired(ctx context.Context) (int64, error)
}

type accessTokenRepo struct {
	q sqlc.Querier
}

// NewAccessTokenRepo creates an access-token repository backed by the given
// querier.
func NewAccessTokenRepo(q sqlc.Querier) AccessTokenRepo {
	return &accessTokenRepo{q: q}
}

func (r *accessTokenRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash, refreshTokenHash string, expiresAt, refreshExpiresAt time.Time) (sqlc.AccessToken, error) {
	record, err := r.q.CreateAccessToken(ctx, sqlc.CreateAccessTokenParams{
		UserID:           userID,
		TokenHash:        tokenHash,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        expiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	})
	if err != nil {
		return sqlc.AccessToken{}, wrapError(err)
	}
	return record, nil
}

func (r *accessTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (sqlc.AccessToken, error) {
	record, err := r.q.GetAccessTokenByTokenHash(ctx, tokenHash)
	if err != nil {
		return sqlc.AccessToken{}, wrapNotFound(err)
	}
	return record, nil
}

func (r *accessTokenRepo) GetByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (sqlc.AccessToken, error) {
	record, err := r.q.GetAccessTokenByRefreshTokenHash(ctx, refreshTokenHash)
	if err != nil {
		return sqlc.AccessToken{}, wrapNotFound(err)
	}
	return record, nil
}

func (r *accessTokenRepo) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	err := r.q.DeleteAccessTokenByTokenHash(ctx, tokenHash)
	return wrapError(err)
}

// DeleteByUserID removes every access-token row for a user. It is used when
// a password changes so no pre-existing session credential survives.
func (r *accessTokenRepo) DeleteByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	rows, err := r.q.DeleteAccessTokensByUserID(ctx, userID)
	if err != nil {
		return 0, wrapError(err)
	}
	return rows, nil
}

func (r *accessTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	rows, err := r.q.DeleteExpiredAccessTokens(ctx)
	if err != nil {
		return 0, wrapError(err)
	}
	return rows, nil
}

package auth

// In-memory test doubles for the repositories. They mirror the real
// persistence semantics the tests rely on: token lookup by SHA-256 hash,
// expiry filtering, single-row-per-user refresh mapping.

import (
	"context"
	"sync"
	"time"

	"github.com/I-Frostbyte/rvpay-go/clients/db/repo"
	"github.com/I-Frostbyte/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
)

type fakeUserRepo struct {
	mu    sync.Mutex
	users map[string]sqlc.User // keyed by email
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]sqlc.User{}}
}

func (f *fakeUserRepo) seed(name, email, password string, role sqlc.UserRole) (sqlc.User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return sqlc.User{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	user := sqlc.User{
		ID:           uuid.New(),
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		UserRole:     role,
	}
	f.users[email] = user
	return user, nil
}

func (f *fakeUserRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.users)
}

func (f *fakeUserRepo) Create(ctx context.Context, name, email string, passwordHash string, userRole sqlc.UserRole) (sqlc.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	user := sqlc.User{ID: uuid.New(), Name: name, Email: email, PasswordHash: passwordHash, UserRole: userRole}
	f.users[email] = user
	return user, nil
}

func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (sqlc.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.users[email]
	if !ok {
		return sqlc.User{}, repo.ErrNotFound
	}
	return user, nil
}

func (f *fakeUserRepo) GetByID(ctx context.Context, userID uuid.UUID) (sqlc.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, user := range f.users {
		if user.ID == userID {
			return user, nil
		}
	}
	return sqlc.User{}, repo.ErrNotFound
}

func (f *fakeUserRepo) UpdateNameEmail(ctx context.Context, userID uuid.UUID, name, email string) (sqlc.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, user := range f.users {
		if user.ID == userID {
			delete(f.users, key)
			user.Name = name
			user.Email = email
			f.users[email] = user
			return user, nil
		}
	}
	return sqlc.User{}, repo.ErrNotFound
}

func (f *fakeUserRepo) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) (sqlc.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, user := range f.users {
		if user.ID == userID {
			user.PasswordHash = passwordHash
			f.users[key] = user
			return user, nil
		}
	}
	return sqlc.User{}, repo.ErrNotFound
}

func (f *fakeUserRepo) UpdateRefreshTokenHash(ctx context.Context, userID uuid.UUID, refreshTokenHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for email, user := range f.users {
		if user.ID == userID {
			user.RefreshTokenHash = refreshTokenHash
			f.users[email] = user
			return nil
		}
	}
	return repo.ErrNotFound
}

func (f *fakeUserRepo) ClearRefreshTokenHash(ctx context.Context, userID uuid.UUID) error {
	return f.UpdateRefreshTokenHash(ctx, userID, "")
}

type tokenRow struct {
	id               uuid.UUID
	userID           uuid.UUID
	refreshTokenHash string
	expiresAt        time.Time
	refreshExpiresAt time.Time
}

type fakeAccessTokenRepo struct {
	mu    sync.Mutex
	rows  map[string]tokenRow // keyed by access-token hash
	byRef map[string]string   // refresh-token hash → access-token hash
}

func newFakeAccessTokenRepo() *fakeAccessTokenRepo {
	return &fakeAccessTokenRepo{rows: map[string]tokenRow{}, byRef: map[string]string{}}
}

func (f *fakeAccessTokenRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash, refreshTokenHash string, expiresAt, refreshExpiresAt time.Time) (sqlc.AccessToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := tokenRow{
		id:               uuid.New(),
		userID:           userID,
		refreshTokenHash: refreshTokenHash,
		expiresAt:        expiresAt,
		refreshExpiresAt: refreshExpiresAt,
	}
	f.rows[tokenHash] = row
	f.byRef[refreshTokenHash] = tokenHash
	return sqlc.AccessToken{
		ID:               row.id,
		UserID:           userID,
		TokenHash:        tokenHash,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        expiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

// GetByTokenHash mirrors the SQL semantics: expired rows do not exist.
func (f *fakeAccessTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (sqlc.AccessToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[tokenHash]
	if !ok || !time.Now().Before(row.expiresAt) {
		return sqlc.AccessToken{}, repo.ErrNotFound
	}
	return f.toSQLC(tokenHash, row), nil
}

// GetByRefreshTokenHash mirrors the SQL semantics: expired refresh mappings
// do not exist.
func (f *fakeAccessTokenRepo) GetByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (sqlc.AccessToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tokenHash, ok := f.byRef[refreshTokenHash]
	if !ok {
		return sqlc.AccessToken{}, repo.ErrNotFound
	}
	row := f.rows[tokenHash]
	if !time.Now().Before(row.refreshExpiresAt) {
		return sqlc.AccessToken{}, repo.ErrNotFound
	}
	return f.toSQLC(tokenHash, row), nil
}

func (f *fakeAccessTokenRepo) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[tokenHash]
	if !ok {
		return nil
	}
	delete(f.rows, tokenHash)
	delete(f.byRef, row.refreshTokenHash)
	return nil
}

// DeleteByUserID mirrors the SQL semantics: every access-token row of the
// user is removed, which also removes their refresh mappings.
func (f *fakeAccessTokenRepo) DeleteByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var deleted int64
	for tokenHash, row := range f.rows {
		if row.userID == userID {
			delete(f.rows, tokenHash)
			delete(f.byRef, row.refreshTokenHash)
			deleted++
		}
	}
	return deleted, nil
}

func (f *fakeAccessTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

func (f *fakeAccessTokenRepo) toSQLC(tokenHash string, row tokenRow) sqlc.AccessToken {
	return sqlc.AccessToken{
		ID:               row.id,
		UserID:           row.userID,
		TokenHash:        tokenHash,
		RefreshTokenHash: row.refreshTokenHash,
		ExpiresAt:        row.expiresAt,
		RefreshExpiresAt: row.refreshExpiresAt,
	}
}

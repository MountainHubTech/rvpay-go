package repo

import (
	"context"

	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
)

// ClientRepo provides persistence operations for clients.
type ClientRepo interface {
	Create(ctx context.Context, name string, status sqlc.ClientStatus) (sqlc.Client, error)
	GetByID(ctx context.Context, id uuid.UUID) (sqlc.Client, error)
	GetByName(ctx context.Context, name string) (sqlc.Client, error)
	List(ctx context.Context, limit, offset int32) ([]sqlc.Client, error)
	ListActive(ctx context.Context, limit, offset int32) ([]sqlc.Client, error)
	Count(ctx context.Context) (int64, error)
	ExistsByID(ctx context.Context, id uuid.UUID) (bool, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.ClientStatus) (sqlc.Client, error)
	// UpdateDisplayName persists the authoritative HighLevel location name as
	// the client's display name. It is guarded: only a missing/empty
	// display_name is filled and a valid name is never overwritten, so it is
	// safe to call repeatedly (idempotent) and from a retry path.
	UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) (sqlc.Client, error)
	// ListNeedingDisplayName returns HighLevel clients whose authoritative
	// display name has not been persisted yet, together with their HighLevel
	// locationId (integrations.external_account_id). It is the backfill /
	// reconciliation source and shrinks monotonically as names are persisted,
	// which makes a repeated backfill run a no-op.
	ListNeedingDisplayName(ctx context.Context, limit, offset int32) ([]sqlc.ListClientsNeedingDisplayNameRow, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListSubAccounts(ctx context.Context, search, status, sort, order string, limit, offset int32) ([]sqlc.ListSubAccountsFilteredRow, error)
	CountSubAccounts(ctx context.Context, search, status string) (int64, error)
}

type clientRepo struct {
	q sqlc.Querier
}

// NewClientRepo creates a client repository backed by the given querier.
func NewClientRepo(q sqlc.Querier) ClientRepo {
	return &clientRepo{q: q}
}

func (r *clientRepo) Create(ctx context.Context, name string, status sqlc.ClientStatus) (sqlc.Client, error) {
	client, err := r.q.CreateClient(ctx, sqlc.CreateClientParams{
		ClientName: name,
		Status:     status,
	})
	if err != nil {
		return sqlc.Client{}, wrapError(err)
	}
	return client, nil
}

func (r *clientRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.Client, error) {
	client, err := r.q.GetClientByID(ctx, id)
	if err != nil {
		return sqlc.Client{}, wrapNotFound(err)
	}
	return client, nil
}

func (r *clientRepo) GetByName(ctx context.Context, name string) (sqlc.Client, error) {
	client, err := r.q.GetClientByName(ctx, name)
	if err != nil {
		return sqlc.Client{}, wrapNotFound(err)
	}
	return client, nil
}

func (r *clientRepo) List(ctx context.Context, limit, offset int32) ([]sqlc.Client, error) {
	clients, err := r.q.ListClients(ctx, sqlc.ListClientsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return clients, nil
}

func (r *clientRepo) ListActive(ctx context.Context, limit, offset int32) ([]sqlc.Client, error) {
	clients, err := r.q.ListActiveClients(ctx, sqlc.ListActiveClientsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return clients, nil
}

func (r *clientRepo) Count(ctx context.Context) (int64, error) {
	count, err := r.q.CountClients(ctx)
	if err != nil {
		return 0, wrapError(err)
	}
	return count, nil
}

func (r *clientRepo) ExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	exists, err := r.q.ClientExistsByID(ctx, id)
	if err != nil {
		return false, wrapError(err)
	}
	return exists, nil
}

func (r *clientRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.ClientStatus) (sqlc.Client, error) {
	client, err := r.q.UpdateClientStatus(ctx, sqlc.UpdateClientStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return sqlc.Client{}, wrapNotFound(err)
	}
	return client, nil
}

func (r *clientRepo) UpdateDisplayName(ctx context.Context, id uuid.UUID, displayName string) (sqlc.Client, error) {
	client, err := r.q.UpdateClientDisplayName(ctx, sqlc.UpdateClientDisplayNameParams{
		ID:          id,
		DisplayName: displayName,
	})
	if err != nil {
		return sqlc.Client{}, wrapNotFound(err)
	}
	return client, nil
}

// ListNeedingDisplayName returns HighLevel clients whose display name is still
// unset. The query only returns clients with a HighLevel integration mapped to
// a non-empty locationId, so the caller can always resolve an authoritative
// location for each row.
func (r *clientRepo) ListNeedingDisplayName(ctx context.Context, limit, offset int32) ([]sqlc.ListClientsNeedingDisplayNameRow, error) {
	rows, err := r.q.ListClientsNeedingDisplayName(ctx, sqlc.ListClientsNeedingDisplayNameParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return rows, nil
}

func (r *clientRepo) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.q.DeleteClient(ctx, id)
	if err != nil {
		return wrapError(err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *clientRepo) ListSubAccounts(ctx context.Context, search, status, sort, order string, limit, offset int32) ([]sqlc.ListSubAccountsFilteredRow, error) {
	rows, err := r.q.ListSubAccountsFiltered(ctx, sqlc.ListSubAccountsFilteredParams{
		Column1: search,
		Column2: status,
		Column3: sort,
		Column4: order,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return rows, nil
}

func (r *clientRepo) CountSubAccounts(ctx context.Context, search, status string) (int64, error) {
	count, err := r.q.CountSubAccountsFiltered(ctx, sqlc.CountSubAccountsFilteredParams{
		Column1: search,
		Column2: status,
	})
	if err != nil {
		return 0, wrapError(err)
	}
	return count, nil
}

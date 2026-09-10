package repo

import (
	"context"

	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
)

// DisputeRepo provides persistence operations for customer disputes
// (chargebacks / payout issues) surfaced on the Admin Dashboard disputes page.
type DisputeRepo interface {
	ListFiltered(ctx context.Context, search, status string, limit, offset int32) ([]sqlc.ListDisputesFilteredRow, error)
	CountFiltered(ctx context.Context, search, status string) (int64, error)
	GetStats(ctx context.Context) (sqlc.GetDisputeStatsRow, error)
	GetByID(ctx context.Context, id uuid.UUID) (sqlc.GetDisputeByIDRow, error)
	SubmitEvidence(ctx context.Context, id uuid.UUID) (sqlc.Dispute, error)
}

type disputeRepo struct {
	q sqlc.Querier
}

// NewDisputeRepo creates a dispute repository backed by the given querier.
func NewDisputeRepo(q sqlc.Querier) DisputeRepo {
	return &disputeRepo{q: q}
}

func (r *disputeRepo) ListFiltered(ctx context.Context, search, status string, limit, offset int32) ([]sqlc.ListDisputesFilteredRow, error) {
	disputes, err := r.q.ListDisputesFiltered(ctx, sqlc.ListDisputesFilteredParams{
		Column1: search,
		Column2: status,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return disputes, nil
}

func (r *disputeRepo) CountFiltered(ctx context.Context, search, status string) (int64, error) {
	count, err := r.q.CountDisputesFiltered(ctx, sqlc.CountDisputesFilteredParams{
		Column1: search,
		Column2: status,
	})
	if err != nil {
		return 0, wrapError(err)
	}
	return count, nil
}

func (r *disputeRepo) GetStats(ctx context.Context) (sqlc.GetDisputeStatsRow, error) {
	stats, err := r.q.GetDisputeStats(ctx)
	if err != nil {
		return sqlc.GetDisputeStatsRow{}, wrapError(err)
	}
	return stats, nil
}

func (r *disputeRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.GetDisputeByIDRow, error) {
	dispute, err := r.q.GetDisputeByID(ctx, id)
	if err != nil {
		return sqlc.GetDisputeByIDRow{}, wrapNotFound(err)
	}
	return dispute, nil
}

func (r *disputeRepo) SubmitEvidence(ctx context.Context, id uuid.UUID) (sqlc.Dispute, error) {
	dispute, err := r.q.SubmitEvidence(ctx, id)
	if err != nil {
		return sqlc.Dispute{}, wrapNotFound(err)
	}
	return dispute, nil
}
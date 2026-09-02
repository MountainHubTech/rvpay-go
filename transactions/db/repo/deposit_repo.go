package repo

import (
	"context"

	"github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// textPtr returns a pointer to s for nullable-text columns, mapping the empty
// string to SQL NULL (an absent external identifier is NULL, not "").
func textPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// textRef returns a pointer to s preserving the exact value ("" stays "").
func textRef(s string) *string {
	return &s
}

// DepositRepo provides persistence operations for deposits.
//
// The deposit identifier fields carry external string identifiers for the
// HighLevel payment flow: clientName is the RVPay client name, customerID is
// the external customer identifier, and merchantID is the external merchant
// identifier. They are NOT RVPay UUIDs.
type DepositRepo interface {
	Create(ctx context.Context, clientName string, customerID string, merchantID string, amount pgtype.Numeric, currency string, paymentType sqlc.PaymentType, payerPhoneNumber string, provider sqlc.PaymentProvider, status sqlc.DepositStatus, idempotencyKey uuid.UUID, ghlTransactionID string) (sqlc.Deposit, error)
	GetByID(ctx context.Context, id uuid.UUID) (sqlc.Deposit, error)
	GetByExternalReference(ctx context.Context, externalReference string) (sqlc.Deposit, error)
	GetByGHLTransactionID(ctx context.Context, ghlTransactionID string) (sqlc.Deposit, error)
	GetByGHLChargeID(ctx context.Context, ghlChargeID string) (sqlc.Deposit, error)
	GetByIdempotencyKey(ctx context.Context, idempotencyKey uuid.UUID) (sqlc.Deposit, error)
	ListByClient(ctx context.Context, clientName string) ([]sqlc.Deposit, error)
	ListByCustomer(ctx context.Context, customerID string) ([]sqlc.Deposit, error)
	ListByMerchant(ctx context.Context, merchantID string) ([]sqlc.Deposit, error)
	ListByStatus(ctx context.Context, status sqlc.DepositStatus) ([]sqlc.Deposit, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.DepositStatus) (sqlc.Deposit, error)
	MarkCompleted(ctx context.Context, id uuid.UUID, status sqlc.DepositStatus) (sqlc.Deposit, error)
	MarkFailed(ctx context.Context, id uuid.UUID, status sqlc.DepositStatus, failureReason string) (sqlc.Deposit, error)
	UpdateGHLReference(ctx context.Context, id uuid.UUID, ghlTransactionID, ghlChargeID string) (sqlc.Deposit, error)
}

type depositRepo struct {
	q sqlc.Querier
}

// NewDepositRepo creates a deposit repository backed by the given querier.
func NewDepositRepo(q sqlc.Querier) DepositRepo {
	return &depositRepo{q: q}
}

func (r *depositRepo) Create(ctx context.Context, clientName string, customerID string, merchantID string, amount pgtype.Numeric, currency string, paymentType sqlc.PaymentType, payerPhoneNumber string, provider sqlc.PaymentProvider, status sqlc.DepositStatus, idempotencyKey uuid.UUID, ghlTransactionID string) (sqlc.Deposit, error) {
	deposit, err := r.q.CreateDeposit(ctx, sqlc.CreateDepositParams{
		ClientName:       clientName,
		CustomerID:       textPtr(customerID),
		MerchantID:       textPtr(merchantID),
		Amount:           amount,
		Currency:         currency,
		PaymentType:      paymentType,
		PayerPhoneNumber: payerPhoneNumber,
		Provider:         provider,
		Status:           status,
		IdempotencyKey:   idempotencyKey,
		GhlTransactionID: textPtr(ghlTransactionID),
	})
	if err != nil {
		return sqlc.Deposit{}, wrapError(err)
	}
	return deposit, nil
}

func (r *depositRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.Deposit, error) {
	deposit, err := r.q.GetDepositByID(ctx, id)
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

func (r *depositRepo) GetByExternalReference(ctx context.Context, externalReference string) (sqlc.Deposit, error) {
	deposit, err := r.q.GetDepositByExternalReference(ctx, textRef(externalReference))
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

func (r *depositRepo) GetByGHLTransactionID(ctx context.Context, ghlTransactionID string) (sqlc.Deposit, error) {
	deposit, err := r.q.GetDepositByGHLTransactionID(ctx, textRef(ghlTransactionID))
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

func (r *depositRepo) GetByGHLChargeID(ctx context.Context, ghlChargeID string) (sqlc.Deposit, error) {
	deposit, err := r.q.GetDepositByGHLChargeID(ctx, textRef(ghlChargeID))
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

func (r *depositRepo) GetByIdempotencyKey(ctx context.Context, idempotencyKey uuid.UUID) (sqlc.Deposit, error) {
	deposit, err := r.q.GetDepositByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

func (r *depositRepo) ListByClient(ctx context.Context, clientName string) ([]sqlc.Deposit, error) {
	deposits, err := r.q.ListDepositsByClient(ctx, clientName)
	if err != nil {
		return nil, wrapError(err)
	}
	return deposits, nil
}

func (r *depositRepo) ListByCustomer(ctx context.Context, customerID string) ([]sqlc.Deposit, error) {
	deposits, err := r.q.ListDepositsByCustomer(ctx, textPtr(customerID))
	if err != nil {
		return nil, wrapError(err)
	}
	return deposits, nil
}

func (r *depositRepo) ListByMerchant(ctx context.Context, merchantID string) ([]sqlc.Deposit, error) {
	deposits, err := r.q.ListDepositsByMerchant(ctx, textPtr(merchantID))
	if err != nil {
		return nil, wrapError(err)
	}
	return deposits, nil
}

func (r *depositRepo) ListByStatus(ctx context.Context, status sqlc.DepositStatus) ([]sqlc.Deposit, error) {
	deposits, err := r.q.ListDepositsByStatus(ctx, status)
	if err != nil {
		return nil, wrapError(err)
	}
	return deposits, nil
}

func (r *depositRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.DepositStatus) (sqlc.Deposit, error) {
	deposit, err := r.q.UpdateDepositStatus(ctx, sqlc.UpdateDepositStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

func (r *depositRepo) MarkCompleted(ctx context.Context, id uuid.UUID, status sqlc.DepositStatus) (sqlc.Deposit, error) {
	deposit, err := r.q.UpdateDepositStatusAndCompletedAt(ctx, sqlc.UpdateDepositStatusAndCompletedAtParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

func (r *depositRepo) MarkFailed(ctx context.Context, id uuid.UUID, status sqlc.DepositStatus, failureReason string) (sqlc.Deposit, error) {
	deposit, err := r.q.UpdateDepositStatusAndFailedAt(ctx, sqlc.UpdateDepositStatusAndFailedAtParams{
		ID:            id,
		Status:        status,
		FailureReason: textRef(failureReason),
	})
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

func (r *depositRepo) UpdateGHLReference(ctx context.Context, id uuid.UUID, ghlTransactionID, ghlChargeID string) (sqlc.Deposit, error) {
	deposit, err := r.q.UpdateDepositGHLReference(ctx, sqlc.UpdateDepositGHLReferenceParams{
		ID:               id,
		GhlTransactionID: textRef(ghlTransactionID),
		GhlChargeID:      textRef(ghlChargeID),
	})
	if err != nil {
		return sqlc.Deposit{}, wrapNotFound(err)
	}
	return deposit, nil
}

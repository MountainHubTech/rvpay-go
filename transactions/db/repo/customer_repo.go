package repo

import (
	"context"

	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// uuidToPg converts a nullable uuid.UUID to the pgtype.UUID representation
// used by the generated sqlc code for the nullable customers.merchant_id.
func uuidToPg(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	var bytes [16]byte
	copy(bytes[:], id[:])
	return pgtype.UUID{Bytes: bytes, Valid: true}
}

// CustomerRepo provides persistence operations for customers.
type CustomerRepo interface {
	Create(ctx context.Context, clientName string, merchantID *uuid.UUID, phoneNumber string, name, address *string, status sqlc.CustomerStatus) (sqlc.Customer, error)
	GetByID(ctx context.Context, id uuid.UUID) (sqlc.Customer, error)
	GetByClientNameAndPhone(ctx context.Context, clientName, phoneNumber string) (sqlc.Customer, error)
	GetByClientAndMerchantAndPhone(ctx context.Context, clientName string, merchantID uuid.UUID, phoneNumber string) (sqlc.Customer, error)
	ListByClientName(ctx context.Context, clientName string) ([]sqlc.Customer, error)
	ListByMerchant(ctx context.Context, merchantID uuid.UUID) ([]sqlc.Customer, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.CustomerStatus) (sqlc.Customer, error)
}

type customerRepo struct {
	q sqlc.Querier
}

// NewCustomerRepo creates a customer repository backed by the given querier.
func NewCustomerRepo(q sqlc.Querier) CustomerRepo {
	return &customerRepo{q: q}
}

func (r *customerRepo) Create(ctx context.Context, clientName string, merchantID *uuid.UUID, phoneNumber string, name, address *string, status sqlc.CustomerStatus) (sqlc.Customer, error) {
	customer, err := r.q.CreateCustomer(ctx, sqlc.CreateCustomerParams{
		ClientName:  clientName,
		MerchantID:  uuidToPg(merchantID),
		PhoneNumber: phoneNumber,
		Name:        name,
		Address:     address,
		Status:      status,
	})
	if err != nil {
		return sqlc.Customer{}, wrapError(err)
	}
	return customer, nil
}

func (r *customerRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.Customer, error) {
	customer, err := r.q.GetCustomerByID(ctx, id)
	if err != nil {
		return sqlc.Customer{}, wrapNotFound(err)
	}
	return customer, nil
}

func (r *customerRepo) GetByClientNameAndPhone(ctx context.Context, clientName, phoneNumber string) (sqlc.Customer, error) {
	customer, err := r.q.GetCustomerByClientNameAndPhone(ctx, sqlc.GetCustomerByClientNameAndPhoneParams{
		ClientName:  clientName,
		PhoneNumber: phoneNumber,
	})
	if err != nil {
		return sqlc.Customer{}, wrapNotFound(err)
	}
	return customer, nil
}

func (r *customerRepo) GetByClientAndMerchantAndPhone(ctx context.Context, clientName string, merchantID uuid.UUID, phoneNumber string) (sqlc.Customer, error) {
	customer, err := r.q.GetCustomerByClientAndMerchantAndPhone(ctx, sqlc.GetCustomerByClientAndMerchantAndPhoneParams{
		ClientName:  clientName,
		MerchantID:  uuidToPg(&merchantID),
		PhoneNumber: phoneNumber,
	})
	if err != nil {
		return sqlc.Customer{}, wrapNotFound(err)
	}
	return customer, nil
}

func (r *customerRepo) ListByClientName(ctx context.Context, clientName string) ([]sqlc.Customer, error) {
	customers, err := r.q.ListCustomersByClientName(ctx, clientName)
	if err != nil {
		return nil, wrapError(err)
	}
	return customers, nil
}

func (r *customerRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID) ([]sqlc.Customer, error) {
	customers, err := r.q.ListCustomersByMerchant(ctx, uuidToPg(&merchantID))
	if err != nil {
		return nil, wrapError(err)
	}
	return customers, nil
}

func (r *customerRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.CustomerStatus) (sqlc.Customer, error) {
	customer, err := r.q.UpdateCustomerStatus(ctx, sqlc.UpdateCustomerStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return sqlc.Customer{}, wrapNotFound(err)
	}
	return customer, nil
}

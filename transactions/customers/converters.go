package customers

import (
	transactionsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// customerToProto maps a persisted customer to its protobuf representation.
func customerToProto(customer sqlc.Customer) *transactionsgrpc.Customer {
	return &transactionsgrpc.Customer{
		Id:          customer.ID.String(),
		ClientName:  customer.ClientName,
		MerchantId:  uuidToPgString(customer.MerchantID),
		PhoneNumber: customer.PhoneNumber,
		Status:      sqlcCustomerStatusToGrpc(customer.Status),
		CreatedAt:   timestamppb.New(customer.CreatedAt),
		UpdatedAt:   timestamppb.New(customer.UpdatedAt),
		Name:        derefString(customer.Name),
		Address:     derefString(customer.Address),
	}
}

// uuidToPgString renders a nullable merchant UUID as its canonical string
// form; a NULL merchant renders as the empty string.
func uuidToPgString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	u, err := uuid.FromBytes(id.Bytes[:])
	if err != nil {
		return ""
	}
	return u.String()
}

// derefString maps a nullable text column to the empty string on the wire
// layer only; NULL vs "" is preserved in persistence.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// sqlcCustomerStatusToGrpc maps a persisted customer status to its protobuf
// representation. Unknown statuses map to the unspecified zero value.
func sqlcCustomerStatusToGrpc(customerStatus sqlc.CustomerStatus) transactionsgrpc.CustomerStatus {
	switch customerStatus {
	case sqlc.CustomerStatusCREATED:
		return transactionsgrpc.CustomerStatus_CUSTOMER_STATUS_CREATED
	case sqlc.CustomerStatusACTIVE:
		return transactionsgrpc.CustomerStatus_CUSTOMER_STATUS_ACTIVE
	default:
		return transactionsgrpc.CustomerStatus_CUSTOMER_STATUS_UNSPECIFIED
	}
}

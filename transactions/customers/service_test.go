package customers

import (
	"context"
	"errors"
	"testing"

	transactionsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/MountainHubTech/rvpay-go/transactions/db/repo"
	"github.com/MountainHubTech/rvpay-go/transactions/db/repo/mocks"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateCustomer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *transactionsgrpc.CreateCustomerRequest
		code codes.Code
	}{
		{name: "missing request", code: codes.InvalidArgument},
		{name: "missing client name", req: &transactionsgrpc.CreateCustomerRequest{ClientName: "  "}, code: codes.InvalidArgument},
		{name: "invalid merchant id", req: &transactionsgrpc.CreateCustomerRequest{ClientName: "highlevel-abc", MerchantId: "not-a-uuid"}, code: codes.InvalidArgument},
		{name: "empty phone number", req: &transactionsgrpc.CreateCustomerRequest{ClientName: "highlevel-abc", MerchantId: uuid.New().String(), PhoneNumber: ""}, code: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			customerRepo := mocks.NewMockCustomerRepo(ctrl)
			service := NewCustomerService(customerRepo, zerolog.Nop())

			_, err := service.CreateCustomer(context.Background(), tt.req)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("status code = %s, want %s", got, tt.code)
			}
		})
	}
}

func TestCreateCustomerSuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	customerRepo := mocks.NewMockCustomerRepo(ctrl)
	service := NewCustomerService(customerRepo, zerolog.Nop())

	merchantID := uuid.New()
	var gotClientName, gotPhone string
	customerRepo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), sqlc.CustomerStatusCREATED).
		DoAndReturn(func(_ context.Context, clientName string, _ *uuid.UUID, phoneNumber string, _, _ *string, _ sqlc.CustomerStatus) (sqlc.Customer, error) {
			gotClientName = clientName
			gotPhone = phoneNumber
			return sqlc.Customer{
				ID:          uuid.New(),
				ClientName:  clientName,
				MerchantID:  pgtype.UUID{Bytes: merchantID, Valid: true},
				PhoneNumber: phoneNumber,
			}, nil
		})

	resp, err := service.CreateCustomer(context.Background(), &transactionsgrpc.CreateCustomerRequest{
		ClientName:  "highlevel-abc",
		MerchantId:  merchantID.String(),
		PhoneNumber: "+237600000000",
	})
	if err != nil {
		t.Fatalf("CreateCustomer failed: %v", err)
	}
	if gotClientName != "highlevel-abc" || gotPhone != "+237600000000" {
		t.Fatalf("create args = (%q, %q), want (highlevel-abc, +237600000000)", gotClientName, gotPhone)
	}
	if resp.Customer.MerchantId != merchantID.String() {
		t.Fatalf("merchant id = %s, want %s", resp.Customer.MerchantId, merchantID.String())
	}
}

func TestCreateCustomerDuplicate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	customerRepo := mocks.NewMockCustomerRepo(ctrl)
	service := NewCustomerService(customerRepo, zerolog.Nop())

	customerRepo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), sqlc.CustomerStatusCREATED).
		Return(sqlc.Customer{}, repo.ErrDuplicate)

	_, err := service.CreateCustomer(context.Background(), &transactionsgrpc.CreateCustomerRequest{
		ClientName:  "highlevel-abc",
		MerchantId:  uuid.New().String(),
		PhoneNumber: "+237600000000",
	})
	if got := status.Code(err); got != codes.AlreadyExists {
		t.Fatalf("status code = %s, want %s", got, codes.AlreadyExists)
	}
}

func TestCreateCustomerMerchantNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	customerRepo := mocks.NewMockCustomerRepo(ctrl)
	service := NewCustomerService(customerRepo, zerolog.Nop())

	customerRepo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), sqlc.CustomerStatusCREATED).
		Return(sqlc.Customer{}, repo.ErrConstraint)

	_, err := service.CreateCustomer(context.Background(), &transactionsgrpc.CreateCustomerRequest{
		ClientName:  "highlevel-abc",
		MerchantId:  uuid.New().String(),
		PhoneNumber: "+237600000000",
	})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("status code = %s, want %s", got, codes.NotFound)
	}
}

func TestGetCustomer(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	customerRepo := mocks.NewMockCustomerRepo(ctrl)
	service := NewCustomerService(customerRepo, zerolog.Nop())

	customerID := uuid.New()
	customerRepo.EXPECT().GetByID(gomock.Any(), customerID).
		Return(sqlc.Customer{
			ID:          customerID,
			ClientName:  "highlevel-abc",
			MerchantID:  pgtype.UUID{Bytes: uuid.New(), Valid: true},
			PhoneNumber: "+237600000000",
		}, nil)

	resp, err := service.GetCustomer(context.Background(), &transactionsgrpc.GetCustomerRequest{
		CustomerId: customerID.String(),
	})
	if err != nil {
		t.Fatalf("GetCustomer failed: %v", err)
	}
	if resp.Customer.Id != customerID.String() {
		t.Fatalf("customer id = %s, want %s", resp.Customer.Id, customerID.String())
	}
}

func TestGetCustomerNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	customerRepo := mocks.NewMockCustomerRepo(ctrl)
	service := NewCustomerService(customerRepo, zerolog.Nop())

	customerRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(sqlc.Customer{}, repo.ErrNotFound)

	_, err := service.GetCustomer(context.Background(), &transactionsgrpc.GetCustomerRequest{
		CustomerId: uuid.New().String(),
	})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("status code = %s, want %s", got, codes.NotFound)
	}
}

func TestGetCustomerInvalidID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	customerRepo := mocks.NewMockCustomerRepo(ctrl)
	service := NewCustomerService(customerRepo, zerolog.Nop())

	_, err := service.GetCustomer(context.Background(), &transactionsgrpc.GetCustomerRequest{
		CustomerId: "not-a-uuid",
	})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s", got, codes.InvalidArgument)
	}
}

func TestGetCustomerRepositoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	customerRepo := mocks.NewMockCustomerRepo(ctrl)
	service := NewCustomerService(customerRepo, zerolog.Nop())

	customerRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(sqlc.Customer{}, errors.New("database down"))

	_, err := service.GetCustomer(context.Background(), &transactionsgrpc.GetCustomerRequest{
		CustomerId: uuid.New().String(),
	})
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
}

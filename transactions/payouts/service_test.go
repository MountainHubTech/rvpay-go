package payouts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/I-Frostbyte/pawapay_client"
	commongrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/commongrpc"
	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo/mocks"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRequestPayoutValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *transactionsgrpc.CreatePayoutRequest
		code codes.Code
	}{
		{name: "missing request", code: codes.InvalidArgument},
		{name: "invalid client id", req: &transactionsgrpc.CreatePayoutRequest{ClientId: "not-a-uuid"}, code: codes.InvalidArgument},
		{name: "invalid merchant id", req: &transactionsgrpc.CreatePayoutRequest{ClientId: uuid.New().String(), MerchantId: "not-a-uuid"}, code: codes.InvalidArgument},
		{name: "missing amount", req: &transactionsgrpc.CreatePayoutRequest{ClientId: uuid.New().String(), MerchantId: uuid.New().String()}, code: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			payoutRepo := mocks.NewMockPayoutRepo(ctrl)
			service := NewPayoutService(payoutRepo, zerolog.Nop(), pawapay_client.Client{})

			_, err := service.RequestPayout(context.Background(), tt.req)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("status code = %s, want %s", got, tt.code)
			}
		})
	}
}

func TestRequestPayoutZeroAmount(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), pawapay_client.Client{})

	_, err := service.RequestPayout(context.Background(), &transactionsgrpc.CreatePayoutRequest{
		ClientId:   uuid.New().String(),
		MerchantId: uuid.New().String(),
		Amount:     &commongrpc.Money{Amount: "0", Currency: "XAF"},
	})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s", got, codes.InvalidArgument)
	}
}

func TestRequestPayoutMissingDestination(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), pawapay_client.Client{})

	_, err := service.RequestPayout(context.Background(), &transactionsgrpc.CreatePayoutRequest{
		ClientId:   uuid.New().String(),
		MerchantId: uuid.New().String(),
		Amount:     &commongrpc.Money{Amount: "1000.00", Currency: "XAF"},
		Provider:   commongrpc.Provider_PROVIDER_MTN_MOMO,
	})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s", got, codes.InvalidArgument)
	}
}

func TestRequestPayoutSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/payouts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"payoutId":"po-1","status":"ACCEPTED"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), *pawapay_client.NewClient(srv.URL, "test-key"))

	var amount pgtype.Numeric
	if err := amount.Scan("1000.00"); err != nil {
		t.Fatalf("failed to scan amount: %v", err)
	}

	payoutRepo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "XAF", sqlc.PaymentProviderMTNMOMO, "USER-123", sqlc.PayoutStatusREQUESTED, gomock.Any()).
		Return(sqlc.Payout{ID: uuid.New()}, nil)

	resp, err := service.RequestPayout(context.Background(), &transactionsgrpc.CreatePayoutRequest{
		ClientId:             uuid.New().String(),
		MerchantId:           uuid.New().String(),
		Amount:               &commongrpc.Money{Amount: "1000.00", Currency: "XAF"},
		Provider:             commongrpc.Provider_PROVIDER_MTN_MOMO,
		DestinationReference: "USER-123",
	})
	if err != nil {
		t.Fatalf("RequestPayout failed: %v", err)
	}
	if resp.Payout == nil {
		t.Fatal("payout should not be nil")
	}
}

func TestRequestPayoutProviderError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"failureCode":"INTERNAL","failureMessage":"boom"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), *pawapay_client.NewClient(srv.URL, "test-key"))

	payoutRepo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "XAF", sqlc.PaymentProviderMTNMOMO, "USER-123", sqlc.PayoutStatusREQUESTED, gomock.Any()).
		Return(sqlc.Payout{ID: uuid.New()}, nil)

	_, err := service.RequestPayout(context.Background(), &transactionsgrpc.CreatePayoutRequest{
		ClientId:             uuid.New().String(),
		MerchantId:           uuid.New().String(),
		Amount:               &commongrpc.Money{Amount: "1000.00", Currency: "XAF"},
		Provider:             commongrpc.Provider_PROVIDER_MTN_MOMO,
		DestinationReference: "USER-123",
	})
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
}

func TestRequestPayoutDuplicate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), pawapay_client.Client{})

	payoutRepo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), sqlc.PayoutStatusREQUESTED, gomock.Any()).
		Return(sqlc.Payout{}, repo.ErrDuplicate)

	_, err := service.RequestPayout(context.Background(), &transactionsgrpc.CreatePayoutRequest{
		ClientId:             uuid.New().String(),
		MerchantId:           uuid.New().String(),
		Amount:               &commongrpc.Money{Amount: "1000.00", Currency: "XAF"},
		Provider:             commongrpc.Provider_PROVIDER_MTN_MOMO,
		DestinationReference: "USER-123",
	})
	if got := status.Code(err); got != codes.AlreadyExists {
		t.Fatalf("status code = %s, want %s", got, codes.AlreadyExists)
	}
}

func TestGetPayout(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), pawapay_client.Client{})

	payoutID := uuid.New()
	payoutRepo.EXPECT().GetByID(gomock.Any(), payoutID).
		Return(sqlc.Payout{ID: payoutID}, nil)

	resp, err := service.GetPayout(context.Background(), &transactionsgrpc.GetPayoutRequest{
		PayoutId: payoutID.String(),
	})
	if err != nil {
		t.Fatalf("GetPayout failed: %v", err)
	}
	if resp.Payout.Id != payoutID.String() {
		t.Fatalf("payout id = %s, want %s", resp.Payout.Id, payoutID.String())
	}
}

func TestGetPayoutNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), pawapay_client.Client{})

	payoutRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(sqlc.Payout{}, repo.ErrNotFound)

	_, err := service.GetPayout(context.Background(), &transactionsgrpc.GetPayoutRequest{
		PayoutId: uuid.New().String(),
	})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("status code = %s, want %s", got, codes.NotFound)
	}
}

func TestGetPayoutInvalidID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), pawapay_client.Client{})

	_, err := service.GetPayout(context.Background(), &transactionsgrpc.GetPayoutRequest{
		PayoutId: "not-a-uuid",
	})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s", got, codes.InvalidArgument)
	}
}

func TestGetPayoutRepositoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	payoutRepo := mocks.NewMockPayoutRepo(ctrl)
	service := NewPayoutService(payoutRepo, zerolog.Nop(), pawapay_client.Client{})

	payoutRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).
		Return(sqlc.Payout{}, errors.New("database down"))

	_, err := service.GetPayout(context.Background(), &transactionsgrpc.GetPayoutRequest{
		PayoutId: uuid.New().String(),
	})
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
}

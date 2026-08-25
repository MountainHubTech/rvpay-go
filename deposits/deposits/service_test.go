package deposits

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/I-Frostbyte/pawapay_client"
	repomocks "github.com/I-Frostbyte/rvpay-go/deposits/db/repo/mocks"
	"github.com/I-Frostbyte/rvpay-go/deposits/db/sqlc"
	sqlcmocks "github.com/I-Frostbyte/rvpay-go/deposits/db/sqlc/mocks"
	depositsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/depositsgrpc"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestInitiateDepositRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	service := NewDepositsService(nil, zerolog.Nop(), pawapay_client.Client{})
	validPayer := &depositsgrpc.Participant{
		Type: depositsgrpc.DepositType_DEPOSIT_PORTAL_MMO,
		AccountDetails: &depositsgrpc.AccountDetails{
			PhoneNumber: "+237699541235",
			Provider:    depositsgrpc.DepositProvider_DEPOSIT_PROVIDER_MTN_MOMO_CMR,
		},
	}

	tests := []struct {
		name string
		req  *depositsgrpc.CreateDepositRequest
		code codes.Code
	}{
		{name: "missing request", code: codes.InvalidArgument},
		{name: "invalid amount", req: &depositsgrpc.CreateDepositRequest{Amount: "invalid"}, code: codes.InvalidArgument},
		{name: "zero amount", req: &depositsgrpc.CreateDepositRequest{Amount: "0"}, code: codes.InvalidArgument},
		{name: "invalid client ID", req: &depositsgrpc.CreateDepositRequest{Amount: "1", ClientId: "invalid"}, code: codes.InvalidArgument},
		{name: "missing payer", req: &depositsgrpc.CreateDepositRequest{Amount: "1", ClientId: "0e8caa3c-77fb-4e69-9241-79a8a9be5bdb"}, code: codes.InvalidArgument},
		{name: "missing phone number", req: &depositsgrpc.CreateDepositRequest{Amount: "1", ClientId: "0e8caa3c-77fb-4e69-9241-79a8a9be5bdb", Payer: &depositsgrpc.Participant{AccountDetails: &depositsgrpc.AccountDetails{}}}, code: codes.InvalidArgument},
		{name: "unsupported payer type", req: &depositsgrpc.CreateDepositRequest{Amount: "1", ClientId: "0e8caa3c-77fb-4e69-9241-79a8a9be5bdb", Payer: &depositsgrpc.Participant{Type: depositsgrpc.DepositType_DEPOSIT_PORTAL_CARD, AccountDetails: validPayer.GetAccountDetails()}}, code: codes.InvalidArgument},
		{name: "unsupported provider", req: &depositsgrpc.CreateDepositRequest{Amount: "1", ClientId: "0e8caa3c-77fb-4e69-9241-79a8a9be5bdb", Payer: &depositsgrpc.Participant{Type: validPayer.GetType(), AccountDetails: &depositsgrpc.AccountDetails{PhoneNumber: validPayer.GetAccountDetails().GetPhoneNumber()}}}, code: codes.InvalidArgument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := service.InitiateDeposit(context.Background(), tt.req)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("status code = %s, want %s", got, tt.code)
			}
		})
	}
}

func TestInitiateDepositSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/deposits" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"depositId":"dep-1","status":"ACCEPTED"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockDepositsRepo(ctrl)
	querierMock := sqlcmocks.NewMockQuerier(ctrl)
	repoMock.EXPECT().Do().Return(querierMock)

	clientID := uuid.New()
	querierMock.EXPECT().GetClientByID(gomock.Any(), clientID).Return(sqlc.Client{ID: clientID}, nil)
	querierMock.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{
		ID:               uuid.New(),
		Currency:         "XAF",
		PayerType:        sqlc.PayerTypeMMO,
		PayerPhoneNumber: "+237699541235",
		PayerProvider:    sqlc.PaymentProviderMTNMOMOCMR,
	}, nil)

	service := NewDepositsService(repoMock, zerolog.Nop(), *pawapay_client.NewClient(srv.URL, "test-key"))

	resp, err := service.InitiateDeposit(context.Background(), &depositsgrpc.CreateDepositRequest{
		Amount:   "1500.50",
		ClientId: clientID.String(),
		Currency: "XAF",
		Payer: &depositsgrpc.Participant{
			Type: depositsgrpc.DepositType_DEPOSIT_PORTAL_MMO,
			AccountDetails: &depositsgrpc.AccountDetails{
				PhoneNumber: "+237699541235",
				Provider:    depositsgrpc.DepositProvider_DEPOSIT_PROVIDER_MTN_MOMO_CMR,
			},
		},
	})
	if err != nil {
		t.Fatalf("InitiateDeposit failed: %v", err)
	}
	if resp.DepositId == "" {
		t.Fatal("deposit id should not be empty")
	}
}

func TestInitiateDepositProviderError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"failureCode":"INTERNAL","failureMessage":"boom"}`))
	}))
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockDepositsRepo(ctrl)
	querierMock := sqlcmocks.NewMockQuerier(ctrl)
	repoMock.EXPECT().Do().Return(querierMock)

	clientID := uuid.New()
	querierMock.EXPECT().GetClientByID(gomock.Any(), clientID).Return(sqlc.Client{ID: clientID}, nil)
	querierMock.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{
		ID:               uuid.New(),
		Currency:         "XAF",
		PayerType:        sqlc.PayerTypeMMO,
		PayerPhoneNumber: "+237699541235",
		PayerProvider:    sqlc.PaymentProviderMTNMOMOCMR,
	}, nil)

	service := NewDepositsService(repoMock, zerolog.Nop(), *pawapay_client.NewClient(srv.URL, "test-key"))

	_, err := service.InitiateDeposit(context.Background(), &depositsgrpc.CreateDepositRequest{
		Amount:   "1500.50",
		ClientId: clientID.String(),
		Currency: "XAF",
		Payer: &depositsgrpc.Participant{
			Type: depositsgrpc.DepositType_DEPOSIT_PORTAL_MMO,
			AccountDetails: &depositsgrpc.AccountDetails{
				PhoneNumber: "+237699541235",
				Provider:    depositsgrpc.DepositProvider_DEPOSIT_PROVIDER_MTN_MOMO_CMR,
			},
		},
	})
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
}

func TestGrpcPayerTypeToSqlc(t *testing.T) {
	t.Parallel()

	if _, err := grpcPayerTypeToSqlc(depositsgrpc.DepositType_DEPOSIT_PORTAL_MMO); err != nil {
		t.Fatalf("MMO payer type returned error: %v", err)
	}
	if _, err := grpcPayerTypeToSqlc(depositsgrpc.DepositType_DEPOSIT_PORTAL_CARD); err == nil {
		t.Fatal("CARD payer type did not return an error")
	}
}

func TestGrpcProviderToSqlc(t *testing.T) {
	t.Parallel()

	for _, provider := range []depositsgrpc.DepositProvider{
		depositsgrpc.DepositProvider_DEPOSIT_PROVIDER_MTN_MOMO_CMR,
		depositsgrpc.DepositProvider_DEPOSIT_PROVIDER_ORANGE_MOMO_CMR,
	} {
		if _, err := grpcProviderToSqlc(provider); err != nil {
			t.Fatalf("provider %s returned error: %v", provider, err)
		}
	}
	if _, err := grpcProviderToSqlc(depositsgrpc.DepositProvider_DEPOSIT_PROVIDER_UNSPECIFIED); err == nil {
		t.Fatal("unspecified provider did not return an error")
	}
}

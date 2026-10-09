package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	commongrpc "github.com/MountainHubTech/rvpay-go/grpc/go/commongrpc"
	transactionsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/transactionsgrpc"
	commonobservability "github.com/MountainHubTech/rvpay-go/shared/observability"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeMerchantService implements transactionsgrpc.MerchantServiceServer for
// gateway wiring tests. It embeds the generated Unimplemented type so any RPC
// not overridden fails with codes.Unimplemented.
type fakeMerchantService struct {
	transactionsgrpc.UnimplementedMerchantServiceServer

	getMerchantErr error
}

func (f *fakeMerchantService) GetMerchant(_ context.Context, req *transactionsgrpc.GetMerchantRequest) (*transactionsgrpc.GetMerchantResponse, error) {
	if f.getMerchantErr != nil {
		return nil, f.getMerchantErr
	}
	return &transactionsgrpc.GetMerchantResponse{
		Merchant: &transactionsgrpc.Merchant{
			Id:        req.GetMerchantId(),
			Name:      "PawaPay",
			Slug:      "pawapay",
			Status:    transactionsgrpc.MerchantStatus_MERCHANT_STATUS_ACTIVE,
			CreatedAt: timestamppb.New(time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)),
		},
	}, nil
}

// fakeDepositService implements transactionsgrpc.DepositServiceServer for
// gateway wiring tests, exercising a shared commongrpc.Money field.
type fakeDepositService struct {
	transactionsgrpc.UnimplementedDepositServiceServer
}

func (f *fakeDepositService) GetDeposit(_ context.Context, req *transactionsgrpc.GetDepositRequest) (*transactionsgrpc.GetDepositResponse, error) {
	return &transactionsgrpc.GetDepositResponse{
		Deposit: &transactionsgrpc.Deposit{
			Id:          req.GetDepositId(),
			ClientName:  "highlevel-abc123",
			MerchantId:  "mch_1",
			Amount:      &commongrpc.Money{Amount: "1000.00", Currency: "XAF"},
			Status:      transactionsgrpc.DepositStatus_DEPOSIT_STATUS_COMPLETED,
			Provider:    commongrpc.Provider_PROVIDER_MTN_MOMO,
			PaymentType: commongrpc.PaymentType_PAYMENT_TYPE_MMO,
		},
	}, nil
}

// newTransactionsGateway constructs the exact gateway wiring used by
// transactions/cmd/grpc-service/main.go: a grpc-gateway runtime.ServeMux with
// the generated Register...HandlerServer functions, mounted behind the root
// HTTP mux alongside /healthz.
func newTransactionsGateway(t *testing.T, merchant *fakeMerchantService, deposit *fakeDepositService, payment *fakePaymentService, allowedOrigins ...string) *httptest.Server {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	gatewayMux := runtime.NewServeMux()
	if err := transactionsgrpc.RegisterMerchantServiceHandlerServer(ctx, gatewayMux, merchant); err != nil {
		t.Fatalf("register merchant grpc-gateway handler: %v", err)
	}
	if err := transactionsgrpc.RegisterDepositServiceHandlerServer(ctx, gatewayMux, deposit); err != nil {
		t.Fatalf("register deposit grpc-gateway handler: %v", err)
	}
	if err := transactionsgrpc.RegisterPaymentServiceHandlerServer(ctx, gatewayMux, payment); err != nil {
		t.Fatalf("register payment grpc-gateway handler: %v", err)
	}

	httpMux := http.NewServeMux()
	// Mirror main.go: the gateway is mounted behind the CORS middleware. Test
	// origins default to the production allowlist; tests may override them.
	corsOrigins := allowedOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"https://admindashboard.rvpay.xyz"}
	}
	httpMux.Handle("/", commonobservability.CORS(zerolog.Nop(), corsOrigins, gatewayMux))
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := httptest.NewServer(httpMux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGateway_MerchantRoute_JSONMapping(t *testing.T) {
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, &fakePaymentService{})

	resp, err := http.Get(srv.URL + "/v1/public/merchants/mch_1")
	if err != nil {
		t.Fatalf("GET /v1/public/merchants/mch_1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	merchant, ok := body["merchant"].(map[string]interface{})
	if !ok {
		t.Fatalf("response field %q missing or not an object: %v", "merchant", body)
	}

	if got := merchant["id"]; got != "mch_1" {
		t.Errorf("merchant.id = %v, want %q", got, "mch_1")
	}
	if got := merchant["name"]; got != "PawaPay" {
		t.Errorf("merchant.name = %v, want %q", got, "PawaPay")
	}
	if got := merchant["status"]; got != "MERCHANT_STATUS_ACTIVE" {
		t.Errorf("merchant.status = %v, want %q", got, "MERCHANT_STATUS_ACTIVE")
	}
}

func TestGateway_DepositRoute_SharedMoneyMapping(t *testing.T) {
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, &fakePaymentService{})

	resp, err := http.Get(srv.URL + "/v1/public/deposits/dep_1")
	if err != nil {
		t.Fatalf("GET /v1/public/deposits/dep_1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	deposit, ok := body["deposit"].(map[string]interface{})
	if !ok {
		t.Fatalf("response field %q missing or not an object: %v", "deposit", body)
	}

	amount, ok := deposit["amount"].(map[string]interface{})
	if !ok {
		t.Fatalf("deposit.amount missing or not an object: %v", deposit)
	}
	if got := amount["amount"]; got != "1000.00" {
		t.Errorf("amount.amount = %v, want %q", got, "1000.00")
	}
	if got := amount["currency"]; got != "XAF" {
		t.Errorf("amount.currency = %v, want %q", got, "XAF")
	}
}

func TestGateway_ErrorPropagation(t *testing.T) {
	fake := &fakeMerchantService{getMerchantErr: status.Error(codes.NotFound, "merchant not found")}
	srv := newTransactionsGateway(t, fake, &fakeDepositService{}, &fakePaymentService{})

	resp, err := http.Get(srv.URL + "/v1/public/merchants/missing")
	if err != nil {
		t.Fatalf("GET /v1/public/merchants/missing: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGateway_UnimplementedRPC(t *testing.T) {
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, &fakePaymentService{})

	// CreateMerchant is not implemented by the fake; the generated
	// UnimplementedMerchantServiceServer must map it to HTTP 501.
	resp, err := http.Post(srv.URL+"/v1/public/merchants", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /v1/public/merchants: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotImplemented)
	}
}

func TestGateway_Healthz(t *testing.T) {
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, &fakePaymentService{})

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, err = http.Post(srv.URL+"/healthz", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("healthz POST status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

// TestGateway_CORSPreflight_AllowedOrigin verifies that the browser preflight
// for POST /v1/public/deposits from the admin dashboard origin is answered
// with 204 and the required CORS headers instead of being terminated by the
// grpc-gateway mux.
func TestGateway_CORSPreflight_AllowedOrigin(t *testing.T) {
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, &fakePaymentService{})

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/v1/public/deposits", nil)
	if err != nil {
		t.Fatalf("build preflight request: %v", err)
	}
	req.Header.Set("Origin", "https://admindashboard.rvpay.xyz")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "authorization, content-type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /v1/public/deposits: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://admindashboard.rvpay.xyz" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the request origin", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, "GET, POST, OPTIONS")
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, "Content-Type, Authorization")
	}
}

// TestGateway_CORSPreflight_OriginNotAllowed verifies that non-allowlisted
// origins never receive CORS headers (no wildcard behavior).
func TestGateway_CORSPreflight_OriginNotAllowed(t *testing.T) {
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, &fakePaymentService{})

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/v1/public/deposits", nil)
	if err != nil {
		t.Fatalf("build preflight request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /v1/public/deposits: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for non-allowlisted origin", got)
	}
}

// TestGateway_CORS_AllowedOriginOnActualRequest verifies that actual (non-
// preflight) requests from an allowlisted origin carry the CORS header so the
// browser accepts the response.
func TestGateway_CORS_AllowedOriginOnActualRequest(t *testing.T) {
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, &fakePaymentService{})

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/public/deposits/dep_1", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "https://admindashboard.rvpay.xyz")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/public/deposits/dep_1: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://admindashboard.rvpay.xyz" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the request origin", got)
	}
}

// TestGateway_CORS_OverriddenAllowlist verifies the middleware honors a
// custom allowlist (HTTP_CORS_ALLOWED_ORIGINS configuration).
func TestGateway_CORS_OverriddenAllowlist(t *testing.T) {
	srv := newTransactionsGateway(
		t,
		&fakeMerchantService{},
		&fakeDepositService{},
		&fakePaymentService{},
		"https://dashboard.example",
	)

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/v1/public/deposits", nil)
	if err != nil {
		t.Fatalf("build preflight request: %v", err)
	}
	req.Header.Set("Origin", "https://dashboard.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /v1/public/deposits: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://dashboard.example" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://dashboard.example")
	}
}

// fakePaymentService implements transactionsgrpc.PaymentServiceServer for
// gateway wiring tests. It captures the VerifyPayment request so tests can
// assert query-parameter binding.
type fakePaymentService struct {
	transactionsgrpc.UnimplementedPaymentServiceServer

	gotVerifyReq   *transactionsgrpc.VerifyPaymentRequest
	gotCallbackReq *transactionsgrpc.ProcessDepositCallbackRequest
}

func (f *fakePaymentService) VerifyPayment(_ context.Context, req *transactionsgrpc.VerifyPaymentRequest) (*transactionsgrpc.VerifyPaymentResponse, error) {
	f.gotVerifyReq = req
	return &transactionsgrpc.VerifyPaymentResponse{Success: true}, nil
}

func (f *fakePaymentService) ProcessDepositCallback(_ context.Context, req *transactionsgrpc.ProcessDepositCallbackRequest) (*transactionsgrpc.ProcessDepositCallbackResponse, error) {
	f.gotCallbackReq = req
	return &transactionsgrpc.ProcessDepositCallbackResponse{}, nil
}

// TestGateway_VerifyPaymentRoute_GetWithQueryParams verifies the public HTTP
// contract of the payment verification endpoint: GET /v1/public/payments/verify
// with ghlTransactionId, ghlChargeId, and subscriptionId as query parameters
// (the exact form the Admin Dashboard polls).
func TestGateway_VerifyPaymentRoute_GetWithQueryParams(t *testing.T) {
	payment := &fakePaymentService{}
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, payment)

	resp, err := http.Get(srv.URL + "/v1/public/payments/verify?ghlTransactionId=tx-1&ghlChargeId=ch-2&subscriptionId=sub-3")
	if err != nil {
		t.Fatalf("GET verify failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if payment.gotVerifyReq == nil {
		t.Fatal("VerifyPayment was not invoked by the gateway")
	}
	if payment.gotVerifyReq.GetGhlTransactionId() != "tx-1" {
		t.Fatalf("ghl_transaction_id = %q, want %q", payment.gotVerifyReq.GetGhlTransactionId(), "tx-1")
	}
	if payment.gotVerifyReq.GetGhlChargeId() != "ch-2" {
		t.Fatalf("ghl_charge_id = %q, want %q", payment.gotVerifyReq.GetGhlChargeId(), "ch-2")
	}
	if payment.gotVerifyReq.GetSubscriptionId() != "sub-3" {
		t.Fatalf("subscription_id = %q, want %q", payment.gotVerifyReq.GetSubscriptionId(), "sub-3")
	}

	var body struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success {
		t.Fatal("success = false, want true")
	}
}

// TestGateway_PawaPayDepositCallback_Post verifies the PawaPay V2 Deposit
// Status Callback route: POST /v1/public/deposits/callback binds the
// documented PawaPay JSON payload (including fields outside the RVPay
// contract, which the gateway must discard) and returns HTTP 200.
func TestGateway_PawaPayDepositCallback_Post(t *testing.T) {
	payment := &fakePaymentService{}
	srv := newTransactionsGateway(t, &fakeMerchantService{}, &fakeDepositService{}, payment)

	// The exact PawaPay V2 Deposit Status Callback shape, including fields
	// RVPay does not model (payer, metadata, created).
	body := `{"depositId":"0f14d0ab-9605-4a62-a9e4-5ed26688389b","status":"COMPLETED","amount":"25","currency":"XAF","country":"CMR","payer":{"type":"MMO","accountDetails":{"phoneNumber":"237654131027","provider":"MTN_MOMO_CMR"}},"providerTransactionId":"pp-txn-123","failureReason":{"failureCode":"","failureMessage":""},"metadata":{"orderId":"order-1"},"created":"2026-09-02T12:00:00Z"}`

	resp, err := http.Post(srv.URL+"/v1/public/deposits/callback", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST callback failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if payment.gotCallbackReq == nil {
		t.Fatal("ProcessDepositCallback was not invoked by the gateway")
	}
	if got := payment.gotCallbackReq.GetDepositId(); got != "0f14d0ab-9605-4a62-a9e4-5ed26688389b" {
		t.Fatalf("deposit_id = %q, want the echoed PawaPay depositId", got)
	}
	if got := payment.gotCallbackReq.GetStatus(); got != "COMPLETED" {
		t.Fatalf("status = %q, want COMPLETED", got)
	}
	if got := payment.gotCallbackReq.GetProviderTransactionId(); got != "pp-txn-123" {
		t.Fatalf("provider_transaction_id = %q, want pp-txn-123", got)
	}
	if got := payment.gotCallbackReq.GetFailureReason().GetFailureCode(); got != "" {
		t.Fatalf("failure_code = %q, want empty", got)
	}
}

// fakeOverviewService implements transactionsgrpc.DashboardOverviewServiceServer
// for the overview-route gateway tests.
type fakeOverviewService struct {
	transactionsgrpc.UnimplementedDashboardOverviewServiceServer

	// listReq records the request the gateway produced so tests can assert
	// query-parameter binding end to end.
	listReq *transactionsgrpc.ListTransactionsRequest
	// listErr, when set, is returned by ListTransactions (error-mapping tests).
	listErr error
	// listRowSubAccount overrides the sub_account echoed on the canned row.
	listRowSubAccount string
}

// ListTransactions returns a single canned row and records the request.
func (f *fakeOverviewService) ListTransactions(_ context.Context, req *transactionsgrpc.ListTransactionsRequest) (*transactionsgrpc.ListTransactionsResponse, error) {
	f.listReq = req
	if f.listErr != nil {
		return nil, f.listErr
	}
	subAccount := f.listRowSubAccount
	if subAccount == "" {
		subAccount = req.GetSubAccount()
	}
	return &transactionsgrpc.ListTransactionsResponse{
		Rows: []*transactionsgrpc.TransactionListRow{{
			Id:         "dep-1",
			ShortId:    "dep-1",
			SubAccount: subAccount,
			Customer:   "cust-1",
			Amount:     "XAF 1000.00",
			Status:     "Success",
			Gateway:    "MTN MoMo",
			Date:       "Sep 08, 2026",
		}},
		Total:    1,
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	}, nil
}

func (f *fakeOverviewService) GetOverviewSnapshot(_ context.Context, _ *transactionsgrpc.GetOverviewSnapshotRequest) (*transactionsgrpc.GetOverviewSnapshotResponse, error) {
	return &transactionsgrpc.GetOverviewSnapshotResponse{
		TotalRevenue:      25,
		RevenueCurrency:   "XAF",
		TransactionVolume: 2,
		RevenueOverTime:   []*transactionsgrpc.RevenueBucket{{PeriodLabel: "Day 1", Revenue: 25}},
		RecentPayouts:     []*transactionsgrpc.OverviewPayoutRow{{SubAccount: "Acme", Amount: "XAF 1000", Status: "Paid", Date: "Sep 07, 2026"}},
	}, nil
}

func TestGateway_OverviewSnapshotRoute(t *testing.T) {
	// Proves the Dashboard route GET /v1/public/transactions/overview/snapshot
	// reaches the GetOverviewSnapshot RPC (permitted /v1/public/transactions*
	// ALB prefix).
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	gatewayMux := runtime.NewServeMux()
	if err := transactionsgrpc.RegisterDashboardOverviewServiceHandlerServer(ctx, gatewayMux, &fakeOverviewService{}); err != nil {
		t.Fatalf("register overview grpc-gateway handler: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/", gatewayMux)
	srv := httptest.NewServer(httpMux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/v1/public/transactions/overview/snapshot?period=7d")
	if err != nil {
		t.Fatalf("GET /v1/public/transactions/overview/snapshot: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	// int64 fields encode as JSON strings in protojson (grpc-gateway default).
	if got := body["totalRevenue"]; got != "25" {
		t.Errorf("totalRevenue = %v, want \"25\"", got)
	}
	if got := body["transactionVolume"]; got != "2" {
		t.Errorf("transactionVolume = %v, want \"2\"", got)
	}
}

func TestGateway_OverviewSnapshotRoute_NotOnOldPath(t *testing.T) {
	// The old /v1/public/overview/snapshot route must no longer be registered.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	gatewayMux := runtime.NewServeMux()
	if err := transactionsgrpc.RegisterDashboardOverviewServiceHandlerServer(ctx, gatewayMux, &fakeOverviewService{}); err != nil {
		t.Fatalf("register overview grpc-gateway handler: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/", gatewayMux)
	srv := httptest.NewServer(httpMux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/v1/public/overview/snapshot")
	if err != nil {
		t.Fatalf("GET old overview path: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("old route /v1/public/overview/snapshot unexpectedly returned %d", resp.StatusCode)
	}
}

// newOverviewGateway mounts only the DashboardOverviewService gateway handler
// behind the root HTTP mux, mirroring transactions/cmd/grpc-service/main.go.
func newOverviewGateway(t *testing.T, overview *fakeOverviewService) *httptest.Server {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	gatewayMux := runtime.NewServeMux()
	if err := transactionsgrpc.RegisterDashboardOverviewServiceHandlerServer(ctx, gatewayMux, overview); err != nil {
		t.Fatalf("register overview grpc-gateway handler: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/", gatewayMux)
	srv := httptest.NewServer(httpMux)
	t.Cleanup(srv.Close)
	return srv
}

// TestGateway_ListTransactionsRoute_IdentifierParamsBindToRPC proves the
// sub-account identifiers and pagination reach the ListTransactions RPC from
// the HTTP query string of the existing GET /v1/public/transactions route —
// both the proto (snake_case) field names used by the repository convention
// (page_size/sub_account) and the lowerCamelCase JSON aliases.
func TestGateway_ListTransactionsRoute_IdentifierParamsBindToRPC(t *testing.T) {
	fake := &fakeOverviewService{}
	srv := newOverviewGateway(t, fake)

	clientID := "7f2c1e34-1f61-4bd0-9a5f-0c9d5f2b8a11"
	resp, err := http.Get(srv.URL + "/v1/public/transactions?client_id=" + clientID + "&page=2&page_size=5")
	if err != nil {
		t.Fatalf("GET /v1/public/transactions: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if fake.listReq == nil {
		t.Fatal("ListTransactions RPC was not invoked")
	}
	if got := fake.listReq.GetClientId(); got != clientID {
		t.Errorf("client_id = %q, want %q", got, clientID)
	}
	if fake.listReq.GetPage() != 2 || fake.listReq.GetPageSize() != 5 {
		t.Errorf("page/page_size = %d/%d, want 2/5", fake.listReq.GetPage(), fake.listReq.GetPageSize())
	}

	// location_id (proto name).
	resp, err = http.Get(srv.URL + "/v1/public/transactions?location_id=loc-1")
	if err != nil {
		t.Fatalf("GET with location_id: %v", err)
	}
	resp.Body.Close()
	if got := fake.listReq.GetLocationId(); got != "loc-1" {
		t.Errorf("location_id = %q, want loc-1", got)
	}

	// locationId / clientId JSON-name aliases.
	resp, err = http.Get(srv.URL + "/v1/public/transactions?locationId=loc-2")
	if err != nil {
		t.Fatalf("GET with locationId: %v", err)
	}
	resp.Body.Close()
	if got := fake.listReq.GetLocationId(); got != "loc-2" {
		t.Errorf("locationId alias = %q, want loc-2 (grpc-gateway JSON-name query binding)", got)
	}
}

// TestGateway_ListTransactionsRoute_SuccessSerialization proves the successful
// response serializes the existing row shape (ids, sub_account, amount,
// status, gateway, date) with total/page/page_size.
func TestGateway_ListTransactionsRoute_SuccessSerialization(t *testing.T) {
	fake := &fakeOverviewService{listRowSubAccount: "highlevel-loc-a"}
	srv := newOverviewGateway(t, fake)

	resp, err := http.Get(srv.URL + "/v1/public/transactions?location_id=loc-a&page=1&page_size=20")
	if err != nil {
		t.Fatalf("GET /v1/public/transactions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	// int64 fields encode as JSON strings in protojson (grpc-gateway default);
	// int32 fields (page_size) stay JSON numbers.
	if got := body["total"]; got != "1" {
		t.Errorf("total = %v, want \"1\"", got)
	}
	if got := body["pageSize"]; got != float64(20) {
		t.Errorf("pageSize = %v, want 20", got)
	}
	rows, ok := body["rows"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("rows = %v, want exactly 1 row", body["rows"])
	}
	row, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("row = %T, want object", rows[0])
	}
	if got := row["subAccount"]; got != "highlevel-loc-a" {
		t.Errorf("subAccount = %v, want highlevel-loc-a", got)
	}
	for _, key := range []string{"id", "shortId", "customer", "amount", "status", "gateway", "date"} {
		if _, ok := row[key]; !ok {
			t.Errorf("row is missing serialized field %q", key)
		}
	}
}

// TestGateway_ListTransactionsRoute_InvalidIdentifierMapsTo400 proves a
// malformed identifier surfaces as HTTP 400 through the existing gateway
// error mapping (the service returns codes.InvalidArgument).
func TestGateway_ListTransactionsRoute_InvalidIdentifierMapsTo400(t *testing.T) {
	fake := &fakeOverviewService{
		listErr: status.Error(codes.InvalidArgument, "client_id must be a valid UUID"),
	}
	srv := newOverviewGateway(t, fake)

	resp, err := http.Get(srv.URL + "/v1/public/transactions?client_id=not-a-uuid")
	if err != nil {
		t.Fatalf("GET /v1/public/transactions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for a malformed client_id", resp.StatusCode, http.StatusBadRequest)
	}
	if fake.listReq == nil || fake.listReq.GetClientId() != "not-a-uuid" {
		t.Errorf("malformed identifier must still reach the RPC for validation, got %+v", fake.listReq)
	}
}

// TestGateway_ListTransactionsRoute_UnknownClientMapsTo404 proves an unknown
// client id surfaces as HTTP 404 through the gateway error mapping (the
// service returns codes.NotFound after the Clients lookup).
func TestGateway_ListTransactionsRoute_UnknownClientMapsTo404(t *testing.T) {
	fake := &fakeOverviewService{
		listErr: status.Error(codes.NotFound, "client not found"),
	}
	srv := newOverviewGateway(t, fake)

	resp, err := http.Get(srv.URL + "/v1/public/transactions?client_id=0c9d5f2b-8a11-4e3f-b7c6-111111111111")
	if err != nil {
		t.Fatalf("GET /v1/public/transactions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d for an unknown client id", resp.StatusCode, http.StatusNotFound)
	}
}

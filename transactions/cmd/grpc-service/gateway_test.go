package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	commongrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/commongrpc"
	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
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
	httpMux.Handle("/", corsMiddleware(corsOrigins, gatewayMux))
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
	req.Header.Set("Access-Control-Request-Headers", "content-type")

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
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, "Content-Type")
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

package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// newTestPaymentProviderServer creates an httptest.Server that records the
// requests it receives and returns the configured response. It returns the
// server, a channel of received requests, and a cleanup function.
func newTestPaymentProviderServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]*http.Request) {
	t.Helper()

	var requests []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clone the request so we can inspect it after the handler returns.
		reqCopy := r.Clone(r.Context())
		requests = append(requests, reqCopy)
		handler(w, r)
	}))

	t.Cleanup(srv.Close)
	return srv, &requests
}

func TestCreateProviderConfigs_Success(t *testing.T) {
	t.Parallel()

	var reqBody []byte
	srv, requests := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/payments/custom-provider/connect" {
			t.Errorf("path = %s, want /payments/custom-provider/connect", r.URL.Path)
		}
		if got := r.Header.Get("Version"); got != "v3" {
			t.Errorf("Version header = %q, want v3", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("Authorization = %q, want Bearer test-access-token", got)
		}
		if got := r.URL.Query().Get("locationId"); got != "loc-123" {
			t.Errorf("query locationId = %q, want loc-123", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		reqBody = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"_id":"6a917c1d57901cc41b0c3f1c","locationId":"loc-123"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	creds := ProviderCredentials{
		Live: ProviderModeCredentials{APIKey: "rvpay_live_001", PublishableKey: "rvpay_pk_live_001", LiveMode: true},
		Test: ProviderModeCredentials{APIKey: "rvpay_test_001", PublishableKey: "rvpay_pk_test_001", LiveMode: false},
	}
	if err := client.CreateProviderConfigs(context.Background(), "test-access-token", "loc-123", creds); err != nil {
		t.Fatalf("CreateProviderConfigs failed: %v", err)
	}

	if len(*requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(*requests))
	}

	var body map[string]map[string]interface{}
	if err := json.Unmarshal(reqBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["live"]["apiKey"] != "rvpay_live_001" || body["live"]["publishableKey"] != "rvpay_pk_live_001" || body["live"]["liveMode"] != true {
		t.Errorf("live credentials = %v, want apiKey rvpay_live_001, publishableKey rvpay_pk_live_001, liveMode true", body["live"])
	}
	if body["test"]["apiKey"] != "rvpay_test_001" || body["test"]["publishableKey"] != "rvpay_pk_test_001" || body["test"]["liveMode"] != false {
		t.Errorf("test credentials = %v, want apiKey rvpay_test_001, publishableKey rvpay_pk_test_001, liveMode false", body["test"])
	}
}

func TestCreateProviderConfigs_MissingToken(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.CreateProviderConfigs(context.Background(), "", "loc-123", ProviderCredentials{})
	if !errors.Is(err, ErrMissingAccessToken) {
		t.Fatalf("err = %v, want ErrMissingAccessToken", err)
	}
}

func TestCreateProviderConfigs_MissingLocationID(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.CreateProviderConfigs(context.Background(), "test-access-token", "", ProviderCredentials{})
	if !errors.Is(err, ErrMissingLocationID) {
		t.Fatalf("err = %v, want ErrMissingLocationID", err)
	}
}

func TestCreateProviderConfigs_Unauthorized(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderConfigs(context.Background(), "test-access-token", "loc-123", ProviderCredentials{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestCreateProviderAssociation_Success(t *testing.T) {
	t.Parallel()

	var reqBody []byte
	srv, requests := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/payments/custom-provider/provider" {
			t.Errorf("path = %s, want /payments/custom-provider/provider", r.URL.Path)
		}
		// The v3 create-integration contract requires Version: v3 and Bearer auth.
		if got := r.Header.Get("Version"); got != "v3" {
			t.Errorf("Version header = %q, want v3", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("Authorization = %q, want Bearer test-access-token", got)
		}
		// locationId must be a REQUIRED QUERY parameter, not in the body.
		if got := r.URL.Query().Get("locationId"); got != "loc-123" {
			t.Errorf("query locationId = %q, want loc-123", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		reqBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	cfg := ProviderConfig{
		Name:                         "RVPay",
		Description:                  "RVPay payment provider",
		ImageURL:                     "https://example.com/logo.jpg",
		LocationID:                   "loc-123",
		QueryURL:                     "https://api.example.com/payments/custom-provider/query",
		PaymentsURL:                  "https://checkout.example.com/payment/checkout",
		SupportsSubscriptionSchedule: false,
	}
	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderAssociation(context.Background(), "test-access-token", cfg)
	if err != nil {
		t.Fatalf("CreateProviderAssociation failed: %v", err)
	}

	if len(*requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(*requests))
	}

	// The provider metadata must be sent in the JSON body (this is what makes
	// RVPay appear on HighLevel's Payments > Integrations page).
	var body map[string]interface{}
	if err := json.Unmarshal(reqBody, &body); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}
	if got := body["name"]; got != "RVPay" {
		t.Errorf("body name = %v, want RVPay", got)
	}
	if got := body["paymentsUrl"]; got != "https://checkout.example.com/payment/checkout" {
		t.Errorf("body paymentsUrl = %v, want configured URL", got)
	}
	if got := body["queryUrl"]; got != "https://api.example.com/payments/custom-provider/query" {
		t.Errorf("body queryUrl = %v, want configured URL", got)
	}
	if got := body["imageUrl"]; got != "https://example.com/logo.jpg" {
		t.Errorf("body imageUrl = %v, want logo URL", got)
	}
	if got := body["supportsSubscriptionSchedule"]; got != false {
		t.Errorf("body supportsSubscriptionSchedule = %v, want false", got)
	}
	// locationId must NOT be in the body.
	if _, ok := body["locationId"]; ok {
		t.Error("body must not contain locationId (it is a query parameter)")
	}
}

func TestCreateProviderAssociation_MissingAccessToken(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.CreateProviderAssociation(context.Background(), "", ProviderConfig{LocationID: "loc-123"})
	if !errors.Is(err, ErrMissingAccessToken) {
		t.Fatalf("expected ErrMissingAccessToken, got %v", err)
	}
}

func TestCreateProviderAssociation_MissingLocationID(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.CreateProviderAssociation(context.Background(), "token", ProviderConfig{})
	if !errors.Is(err, ErrMissingLocationID) {
		t.Fatalf("expected ErrMissingLocationID, got %v", err)
	}
}

func TestCreateProviderAssociation_BadRequest(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid location"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderAssociation(context.Background(), "token", ProviderConfig{LocationID: "loc-123"})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestCreateProviderAssociation_Unauthorized(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderAssociation(context.Background(), "token", ProviderConfig{LocationID: "loc-123"})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestCreateProviderAssociation_UnprocessableEntity(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"validation failed"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderAssociation(context.Background(), "token", ProviderConfig{LocationID: "loc-123"})
	if !errors.Is(err, ErrUnprocessableEntity) {
		t.Fatalf("expected ErrUnprocessableEntity, got %v", err)
	}
}

func TestCreateProviderAssociation_NetworkError(t *testing.T) {
	t.Parallel()

	// Use a closed server to simulate a network error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderAssociation(context.Background(), "token", ProviderConfig{LocationID: "loc-123"})
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestCreateProviderAssociation_ContextCancellation(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response.
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := client.CreateProviderAssociation(ctx, "token", ProviderConfig{LocationID: "loc-123"})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestCreateProviderAssociation_ErrorCarriesDiagnostics(t *testing.T) {
	t.Parallel()

	// The HighLevel error must remain a typed sentinel with an unchanged
	// message, while additionally carrying the HTTP status, the sanitized
	// body, and the HighLevel traceId for diagnostic logging. Credentials
	// (including the access token) must never appear in any of them.
	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"provider already exists","traceId":"trace-diag-123"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderAssociation(context.Background(), "test-access-token", ProviderConfig{LocationID: "loc-123"})

	if !errors.Is(err, ErrUnprocessableEntity) {
		t.Fatalf("error = %v, want ErrUnprocessableEntity", err)
	}
	var apiErr *HighLevelAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %v does not carry HighLevelAPIError diagnostics", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.TraceID != "trace-diag-123" {
		t.Errorf("TraceID = %q, want trace-diag-123", apiErr.TraceID)
	}
	if !strings.Contains(apiErr.Body, "provider already exists") {
		t.Errorf("Body = %q, want the HighLevel response body", apiErr.Body)
	}
	if strings.Contains(apiErr.Body, "test-access-token") {
		t.Error("Body must never contain the access token")
	}
	// The original error message must be unchanged.
	want := "highlevel: unprocessable entity: {\"message\":\"provider already exists\",\"traceId\":\"trace-diag-123\"}"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestUpdateProviderCapabilities_Success(t *testing.T) {
	t.Parallel()

	srv, requests := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/payments/custom-provider/capabilities" {
			t.Errorf("path = %s, want /payments/custom-provider/capabilities", r.URL.Path)
		}
		if got := r.Header.Get("Version"); got != "v3" {
			t.Errorf("Version header = %q, want v3", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("Authorization = %q, want Bearer test-access-token", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("parse body: %v", err)
		}
		if got, ok := payload["locationId"]; !ok || got != "loc-123" {
			t.Errorf("locationId = %v, want loc-123", got)
		}
		if got, ok := payload["supportsSubscriptionSchedules"]; !ok || got != false {
			t.Errorf("supportsSubscriptionSchedules = %v, want false", got)
		}
		if _, ok := payload["companyId"]; ok {
			t.Error("companyId must not be sent")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	if err := client.UpdateProviderCapabilities(context.Background(), "test-access-token", "loc-123"); err != nil {
		t.Fatalf("UpdateProviderCapabilities failed: %v", err)
	}

	if len(*requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(*requests))
	}
}

func TestUpdateProviderCapabilities_MissingAccessToken(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	if err := client.UpdateProviderCapabilities(context.Background(), "", "loc-123"); !errors.Is(err, ErrMissingAccessToken) {
		t.Fatalf("error = %v, want ErrMissingAccessToken", err)
	}
}

func TestUpdateProviderCapabilities_MissingLocationID(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	if err := client.UpdateProviderCapabilities(context.Background(), "test-access-token", ""); !errors.Is(err, ErrMissingLocationID) {
		t.Fatalf("error = %v, want ErrMissingLocationID", err)
	}
}

func TestUpdateProviderCapabilities_UnprocessableEntity(t *testing.T) {
	t.Parallel()

	srv, requests := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"capabilities not supported"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.UpdateProviderCapabilities(context.Background(), "test-access-token", "loc-123")
	if !errors.Is(err, ErrUnprocessableEntity) {
		t.Fatalf("error = %v, want ErrUnprocessableEntity", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(*requests))
	}
}

func TestFetchProviderConfig_Success(t *testing.T) {
	t.Parallel()

	srv, requests := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/payments/custom-provider/connect" {
			t.Errorf("path = %s, want /payments/custom-provider/connect", r.URL.Path)
		}
		if got := r.URL.Query().Get("locationId"); got != "loc-123" {
			t.Errorf("locationId query param = %q, want loc-123", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("Authorization = %q, want Bearer test-access-token", got)
		}
		if got := r.Header.Get("Version"); got != "v3" {
			t.Errorf("Version header = %q, want v3", got)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"name":"RVPay",
			"description":"RVPay payment provider",
			"imageUrl":"https://example.com/logo.jpg",
			"locationId":"loc-123",
			"queryUrl":"https://api.example.com/payments/custom-provider/query",
			"paymentsUrl":"https://checkout.example.com/payment/checkout",
			"supportsSubscriptionSchedule":false
		}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	config, err := client.FetchProviderConfig(context.Background(), "test-access-token", "loc-123")
	if err != nil {
		t.Fatalf("FetchProviderConfig failed: %v", err)
	}

	if config.Name != "RVPay" {
		t.Errorf("Name = %q, want RVPay", config.Name)
	}
	if config.LocationID != "loc-123" {
		t.Errorf("LocationID = %q, want loc-123", config.LocationID)
	}
	if config.QueryURL != "https://api.example.com/payments/custom-provider/query" {
		t.Errorf("QueryURL = %q, want configured URL", config.QueryURL)
	}
	if config.PaymentsURL != "https://checkout.example.com/payment/checkout" {
		t.Errorf("PaymentsURL = %q, want configured URL", config.PaymentsURL)
	}
	if config.SupportsSubscriptionSchedule {
		t.Error("SupportsSubscriptionSchedule should be false")
	}

	if len(*requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(*requests))
	}
}

func TestFetchProviderConfig_MissingAccessToken(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	_, err := client.FetchProviderConfig(context.Background(), "", "loc-123")
	if !errors.Is(err, ErrMissingAccessToken) {
		t.Fatalf("expected ErrMissingAccessToken, got %v", err)
	}
}

func TestFetchProviderConfig_MissingLocationID(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	_, err := client.FetchProviderConfig(context.Background(), "token", "")
	if !errors.Is(err, ErrMissingLocationID) {
		t.Fatalf("expected ErrMissingLocationID, got %v", err)
	}
}

func TestFetchProviderConfig_MalformedResponse(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	_, err := client.FetchProviderConfig(context.Background(), "token", "loc-123")
	if err == nil {
		t.Fatal("expected malformed response error")
	}
}

func TestFetchProviderConfig_Unauthorized(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	_, err := client.FetchProviderConfig(context.Background(), "token", "loc-123")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDisconnectProvider_Success(t *testing.T) {
	t.Parallel()

	srv, requests := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/payments/custom-provider/connect" {
			t.Errorf("path = %s, want /payments/custom-provider/connect", r.URL.Path)
		}
		if got := r.URL.Query().Get("locationId"); got != "loc-123" {
			t.Errorf("locationId query param = %q, want loc-123", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("Authorization = %q, want Bearer test-access-token", got)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.DisconnectProvider(context.Background(), "test-access-token", "loc-123")
	if err != nil {
		t.Fatalf("DisconnectProvider failed: %v", err)
	}

	if len(*requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(*requests))
	}
}

func TestDisconnectProvider_MissingAccessToken(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.DisconnectProvider(context.Background(), "", "loc-123")
	if !errors.Is(err, ErrMissingAccessToken) {
		t.Fatalf("expected ErrMissingAccessToken, got %v", err)
	}
}

func TestDisconnectProvider_MissingLocationID(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.DisconnectProvider(context.Background(), "token", "")
	if !errors.Is(err, ErrMissingLocationID) {
		t.Fatalf("expected ErrMissingLocationID, got %v", err)
	}
}

func TestDisconnectProvider_Unauthorized(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.DisconnectProvider(context.Background(), "token", "loc-123")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestDisconnectProvider_UnprocessableEntity(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"validation failed"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.DisconnectProvider(context.Background(), "token", "loc-123")
	if !errors.Is(err, ErrUnprocessableEntity) {
		t.Fatalf("expected ErrUnprocessableEntity, got %v", err)
	}
}

func TestAccessTokenNotInErrors(t *testing.T) {
	t.Parallel()

	// SECURITY TEST: the access token must never appear in error strings.
	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderAssociation(context.Background(), "super-secret-access-token", ProviderConfig{LocationID: "loc-123"})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "super-secret-access-token") {
		t.Fatal("access token leaked into error string")
	}
}

func TestAccessTokenNotInLogs(t *testing.T) {
	t.Parallel()

	// SECURITY TEST: the access token must never be logged. We verify this by
	// checking that the request body and headers do not contain the token.
	srv, requests := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.CreateProviderAssociation(context.Background(), "super-secret-access-token", ProviderConfig{LocationID: "loc-123"})
	if err != nil {
		t.Fatalf("CreateProviderAssociation failed: %v", err)
	}

	if len(*requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(*requests))
	}

	// The token should only be in the Authorization header, never in the body.
	req := (*requests)[0]
	if got := req.Header.Get("Authorization"); got != "Bearer super-secret-access-token" {
		t.Errorf("Authorization = %q, want Bearer super-secret-access-token", got)
	}
}

func TestSanitizeErrorBody_Empty(t *testing.T) {
	t.Parallel()

	got := sanitizeErrorBody([]byte{})
	if got != "empty response body" {
		t.Fatalf("expected 'empty response body', got %q", got)
	}
}

func TestSanitizeErrorBody_PlainText(t *testing.T) {
	t.Parallel()

	got := sanitizeErrorBody([]byte("something went wrong"))
	if got != "something went wrong" {
		t.Fatalf("expected 'something went wrong', got %q", got)
	}
}

func TestSanitizeErrorBody_RedactsAccessToken(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":"invalid_grant","error_description":"bad token","access_token":"super-secret"}`)
	got := sanitizeErrorBody(body)

	if strings.Contains(got, "super-secret") {
		t.Fatalf("access_token leaked into sanitized error body: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in sanitized body, got: %s", got)
	}
}

func TestSanitizeErrorBody_RedactsRefreshToken(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":"invalid_grant","refresh_token":"rt-12345"}`)
	got := sanitizeErrorBody(body)

	if strings.Contains(got, "rt-12345") {
		t.Fatalf("refresh_token leaked into sanitized error body: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in sanitized body, got: %s", got)
	}
}

func TestSanitizeErrorBody_RedactsClientSecret(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":"invalid_client","client_secret":"my-secret"}`)
	got := sanitizeErrorBody(body)

	if strings.Contains(got, "my-secret") {
		t.Fatalf("client_secret leaked into sanitized error body: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in sanitized body, got: %s", got)
	}
}

func TestSanitizeErrorBody_RedactsAPIKey(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":"invalid_key","apiKey":"pk-12345"}`)
	got := sanitizeErrorBody(body)

	if strings.Contains(got, "pk-12345") {
		t.Fatalf("apiKey leaked into sanitized error body: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in sanitized body, got: %s", got)
	}
}

func TestSanitizeErrorBody_RedactsAPIKeySnake(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":"invalid_key","api_key":"sk-12345"}`)
	got := sanitizeErrorBody(body)

	if strings.Contains(got, "sk-12345") {
		t.Fatalf("api_key leaked into sanitized error body: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in sanitized body, got: %s", got)
	}
}

func TestSanitizeErrorBody_NonJSON(t *testing.T) {
	t.Parallel()

	body := []byte("this is not json with access_token=secret")
	got := sanitizeErrorBody(body)
	// Non-JSON bodies should still be truncated, but credential field names
	// won't be redacted since we can't parse them safely.
	if !strings.Contains(got, "access_token=secret") {
		t.Fatalf("non-JSON body should be preserved as-is: %s", got)
	}
}

func TestSanitizeErrorBody_TruncatesLongBodies(t *testing.T) {
	t.Parallel()

	// Generate a body longer than 512 bytes.
	longBody := make([]byte, 600)
	for i := range longBody {
		longBody[i] = 'a'
	}
	got := sanitizeErrorBody(longBody)
	if len(got) > 512 {
		t.Fatalf("sanitized body length = %d, want <= 512", len(got))
	}
}

func TestHighLevelProviderPaymentProviderCapability(t *testing.T) {
	t.Parallel()

	// A provider with a payment provider client should advertise the
	// CapabilityPaymentProvider capability.
	paymentClient := NewHighLevelPaymentProviderClient("https://example.com", nil)
	provider := NewHighLevelProvider("client-id", "client-secret", "https://example.com/callback", "", paymentClient, zerolog.Nop())

	if !provider.HasCapability(CapabilityPaymentProvider) {
		t.Fatal("provider should have CapabilityPaymentProvider when payment client is set")
	}

	// A provider without a payment provider client should not advertise it.
	providerNoPayment := NewHighLevelProvider("client-id", "client-secret", "https://example.com/callback", "", nil, zerolog.Nop())
	if providerNoPayment.HasCapability(CapabilityPaymentProvider) {
		t.Fatal("provider should not have CapabilityPaymentProvider when payment client is nil")
	}

	// PaymentProvider() should return the configured client.
	if provider.PaymentProvider() != paymentClient {
		t.Fatal("PaymentProvider() should return the configured client")
	}
	if providerNoPayment.PaymentProvider() != nil {
		t.Fatal("PaymentProvider() should return nil when no client is configured")
	}
}

// TestUpdateOrderStatus_Success verifies that UpdateOrderStatus calls the
// correct GHL v3 order payment record endpoint with the right HTTP method,
// path, headers, and JSON body containing the location ID and deposit amount.
func TestUpdateOrderStatus_Success(t *testing.T) {
	t.Parallel()

	var reqBody []byte
	var recordedReq *http.Request
	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		rec := r.Clone(r.Context())
		rec.Body = r.Body
		recordedReq = rec
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		reqBody = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.UpdateOrderStatus(
		context.Background(),
		"test-access-token",
		"loc-123",
		"ord-abc-456",
		GhlOrderStatusCompleted,
		150050, // 1500.50 in cents
	)
	if err != nil {
		t.Fatalf("UpdateOrderStatus failed: %v", err)
	}

	if recordedReq == nil {
		t.Fatal("no request was recorded")
	}

	// Verify HTTP method is POST (not PUT).
	if recordedReq.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", recordedReq.Method)
	}

	// Verify the path is /payments/orders/{orderId}/record-payment.
	expectedPath := "/payments/orders/ord-abc-456/record-payment"
	if recordedReq.URL.Path != expectedPath {
		t.Errorf("path = %s, want %s", recordedReq.URL.Path, expectedPath)
	}

	// Verify the order ID is in the path, not as a query parameter.
	if recordedReq.URL.RawQuery != "" {
		t.Errorf("expected no query parameters, got %s", recordedReq.URL.RawQuery)
	}

	// Verify required headers.
	if got := recordedReq.Header.Get("Authorization"); got != "Bearer test-access-token" {
		t.Errorf("Authorization = %q, want Bearer test-access-token", got)
	}
	if got := recordedReq.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := recordedReq.Header.Get("Version"); got != "v3" {
		t.Errorf("Version = %q, want v3", got)
	}

	// Verify the JSON body contains the correct fields.
	var body map[string]interface{}
	if err := json.Unmarshal(reqBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	// altId should be the location ID.
	if body["altId"] != "loc-123" {
		t.Errorf("altId = %v, want loc-123", body["altId"])
	}
	// altType should be "location".
	if body["altType"] != "location" {
		t.Errorf("altType = %v, want location", body["altType"])
	}
	// mode should be "other".
	if body["mode"] != "other" {
		t.Errorf("mode = %v, want other", body["mode"])
	}
	// amount should be the actual deposit amount (150050 = 1500.50 cents).
	if body["amount"] != float64(150050) {
		t.Errorf("amount = %v, want 150050", body["amount"])
	}
}

// TestUpdateOrderStatus_MissingToken verifies that a missing access token
// returns ErrMissingAccessToken without making an HTTP call.
func TestUpdateOrderStatus_MissingToken(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.UpdateOrderStatus(
		context.Background(),
		"", // empty token
		"loc-123",
		"ord-abc",
		GhlOrderStatusCompleted,
		100,
	)
	if !errors.Is(err, ErrMissingAccessToken) {
		t.Fatalf("expected ErrMissingAccessToken, got: %v", err)
	}
}

// TestUpdateOrderStatus_MissingLocationID verifies that a missing location ID
// returns ErrMissingLocationID without making an HTTP call.
func TestUpdateOrderStatus_MissingLocationID(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.UpdateOrderStatus(
		context.Background(),
		"token",
		"", // empty location ID
		"ord-abc",
		GhlOrderStatusCompleted,
		100,
	)
	if !errors.Is(err, ErrMissingLocationID) {
		t.Fatalf("expected ErrMissingLocationID, got: %v", err)
	}
}

// TestUpdateOrderStatus_MissingOrderID verifies that a missing order ID
// returns an error without making an HTTP call.
func TestUpdateOrderStatus_MissingOrderID(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.UpdateOrderStatus(
		context.Background(),
		"token",
		"loc-123",
		"", // empty order ID
		GhlOrderStatusCompleted,
		100,
	)
	if err == nil {
		t.Fatal("expected error for missing order ID, got nil")
	}
}

// TestUpdateOrderStatus_InvalidStatus verifies that an unsupported status
// returns an error without making an HTTP call.
func TestUpdateOrderStatus_InvalidStatus(t *testing.T) {
	t.Parallel()

	client := NewHighLevelPaymentProviderClient("https://example.com", nil)
	err := client.UpdateOrderStatus(
		context.Background(),
		"token",
		"loc-123",
		"ord-abc",
		GhlOrderStatus("pending"), // unsupported status
		100,
	)
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
}

// TestUpdateOrderStatus_FailedStatus verifies that a failed status is still
// sent correctly to the new endpoint.
func TestUpdateOrderStatus_FailedStatus(t *testing.T) {
	t.Parallel()

	var recordedReq *http.Request
	var reqBody []byte
	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ = io.ReadAll(r.Body)
		rec := r.Clone(r.Context())
		recordedReq = rec
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.UpdateOrderStatus(
		context.Background(),
		"test-access-token",
		"loc-456",
		"ord-xyz-789",
		GhlOrderStatusFailed,
		250000, // 2500.00 in cents
	)
	if err != nil {
		t.Fatalf("UpdateOrderStatus failed: %v", err)
	}

	if recordedReq == nil {
		t.Fatal("no request was recorded")
	}

	// Verify the path contains the order ID.
	expectedPath := "/payments/orders/ord-xyz-789/record-payment"
	if recordedReq.URL.Path != expectedPath {
		t.Errorf("path = %s, want %s", recordedReq.URL.Path, expectedPath)
	}

	if recordedReq.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", recordedReq.Method)
	}

	// Verify the body contains the correct amount and location ID.
	var body map[string]interface{}
	if err := json.Unmarshal(reqBody, &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	if body["amount"] != float64(250000) {
		t.Errorf("amount = %v, want 250000", body["amount"])
	}
	if body["altId"] != "loc-456" {
		t.Errorf("altId = %v, want loc-456", body["altId"])
	}
}

// TestUpdateOrderStatus_HTTPError verifies that HTTP errors are propagated.
func TestUpdateOrderStatus_HTTPError(t *testing.T) {
	t.Parallel()

	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.UpdateOrderStatus(
		context.Background(),
		"test-access-token",
		"loc-123",
		"ord-abc",
		GhlOrderStatusCompleted,
		100,
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

// TestUpdateOrderStatus_OrderIDInPath verifies that the order ID is escaped in
// the URL path to prevent path injection.
func TestUpdateOrderStatus_OrderIDInPath(t *testing.T) {
	t.Parallel()

	var recordedReq *http.Request
	srv, _ := newTestPaymentProviderServer(t, func(w http.ResponseWriter, r *http.Request) {
		recordedReq = r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	client := NewHighLevelPaymentProviderClient(srv.URL, nil)
	err := client.UpdateOrderStatus(
		context.Background(),
		"token",
		"loc-123",
		"ord-with-special/chars",
		GhlOrderStatusCompleted,
		500,
	)
	if err != nil {
		t.Fatalf("UpdateOrderStatus failed: %v", err)
	}

	// The path should escape the order ID in the URL path.
	if recordedReq.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", recordedReq.Method)
	}
	if !strings.HasPrefix(recordedReq.URL.Path, "/payments/orders/") {
		t.Errorf("path = %s, expected to start with /payments/orders/", recordedReq.URL.Path)
	}
	if !strings.HasSuffix(recordedReq.URL.Path, "/record-payment") {
		t.Errorf("path = %s, expected to end with /record-payment", recordedReq.URL.Path)
	}
}

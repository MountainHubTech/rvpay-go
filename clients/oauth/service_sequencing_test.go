package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/I-Frostbyte/rvpay-go/clients/db/sqlc"
	"github.com/I-Frostbyte/rvpay-go/clients/providers"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TestMain shortens the bounded base-config verification retry so the
// sequencing tests below do not pay the production 500 ms delay per attempt.
func TestMain(m *testing.M) {
	baseConfigVerifyDelay = time.Millisecond
	code := m.Run()
	os.Exit(code)
}

// requestRecorder records the order in which the mock HighLevel server
// receives the Custom Payment Provider lifecycle requests.
type requestRecorder struct {
	mu     sync.Mutex
	order  []string
	counts map[string]int
}

func newRequestRecorder() *requestRecorder {
	return &requestRecorder{counts: make(map[string]int)}
}

func (rec *requestRecorder) record(key string) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.order = append(rec.order, key)
	rec.counts[key]++
}

func (rec *requestRecorder) count(key string) int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.counts[key]
}

// lastIndex returns the index of the last occurrence of key in the recorded
// order, or -1 when the key was never recorded.
func (rec *requestRecorder) lastIndex(key string) int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	last := -1
	for i, k := range rec.order {
		if k == key {
			last = i
		}
	}
	return last
}

// newSequencingTestService builds an OAuth service wired to in-memory mocks
// and a HighLevel provider pointing at a mock server. It mirrors the
// newRegistrationTestService helper but supplies credential settings and an
// ACTIVE integration mapped to locationId "loc-123", returning the
// integration ID for direct RegisterProvider calls.
func newSequencingTestService(t *testing.T, paymentHandler http.HandlerFunc) (*Service, uuid.UUID, *mockPaymentProviderConfigRepo) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accessToken":"test-access-token","refreshToken":"test-refresh-token","expiresIn":3600,"tokenType":"Bearer","scope":"read write","locationId":"loc-123"}`))
		case "/v1/users/me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"loc-123"}`))
		default:
			paymentHandler(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	paymentClient := providers.NewHighLevelPaymentProviderClient(srv.URL, nil)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProviderWithURLs(
		"test-client",
		"test-secret",
		"https://example.com/callback",
		"",
		srv.URL+"/oauth/authorize",
		srv.URL+"/oauth/token",
		srv.URL+"/v1/users/me",
		paymentClient,
	))

	integrationRepo := newMockIntegrationRepo()
	tokenRepo := newMockOAuthTokenRepo()
	clientRepo := newMockClientRepo()
	platformRepo := newMockPlatformRepo()
	stateRepo := newMockOAuthStateRepo()
	configRepo := newMockPaymentProviderConfigRepo()

	clientID := uuid.New()
	platformID := uuid.New()
	clientRepo.clients[clientID.String()] = sqlc.Client{ID: clientID, ClientName: "highlevel-loc-123", Status: sqlc.ClientStatusACTIVE}
	platformRepo.platforms[platformID.String()] = sqlc.Platform{ID: platformID, Name: "HighLevel", Slug: "highlevel", Enabled: true}

	integration, err := integrationRepo.Create(context.Background(), clientID, platformID, "loc-123", sqlc.IntegrationStatusACTIVE)
	if err != nil {
		t.Fatalf("create integration: %v", err)
	}

	svc := NewService(
		integrationRepo,
		tokenRepo,
		clientRepo,
		platformRepo,
		stateRepo,
		configRepo,
		registry,
		"https://example.com/callback",
		sequencingSettings(),
		zerolog.Nop(),
	)

	return svc, integration.ID, configRepo
}

// assertCredentialsPostAfterGet verifies that the credential POST /connect
// occurred after the last confirming GET /connect.
func assertCredentialsPostAfterGet(t *testing.T, rec *requestRecorder) {
	t.Helper()
	if lastPost := rec.lastIndex("post-connect"); lastPost < rec.lastIndex("get-connect") {
		t.Fatalf("credential POST /connect occurred before the confirming GET /connect: %v", rec.order)
	}
}

// sequencingHandler returns a payment handler that dispatches on the
// Custom Payment Provider lifecycle requests using the supplied per-request
// behaviors.
type sequencingResponder func(method, path string, w http.ResponseWriter)

func sequencingHandler(respond sequencingResponder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/payments/custom-provider/provider", "/payments/custom-provider/connect", "/payments/custom-provider/capabilities":
			respond(r.Method, r.URL.Path, w)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func sequencingSettings() ProviderConfigSettings {
	return ProviderConfigSettings{
		Name:               "RVPay",
		Description:        "RVPay payment provider",
		ImageURL:           "https://example.com/logo.jpg",
		PaymentsURL:        "https://checkout.example.com/payment/checkout",
		QueryURL:           "https://api.example.com/payments/custom-provider/query",
		LiveAPIKey:         "live-api-key",
		LivePublishableKey: "live-publishable-key",
		TestAPIKey:         "test-api-key",
		TestPublishableKey: "test-publishable-key",
	}
}

// TestRegisterProvider_CredentialsOnlyAfterBaseConfigConfirmed verifies the
// required HighLevel v3 sequencing: POST /provider first, then GET /connect,
// and only after the base config is confirmed the credential POST /connect.
func TestRegisterProvider_CredentialsOnlyAfterBaseConfigConfirmed(t *testing.T) {
	t.Parallel()

	rec := newRequestRecorder()
	svc, integrationID, _ := newSequencingTestService(t, sequencingHandler(func(method, path string, w http.ResponseWriter) {
		switch {
		case path == "/payments/custom-provider/provider" && method == http.MethodPost:
			rec.record("provider")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodGet:
			rec.record("get-connect")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"RVPay","description":"RVPay payment provider","paymentsUrl":"https://checkout.example.com/payment/checkout","queryUrl":"https://api.example.com/payments/custom-provider/query","imageUrl":"https://example.com/logo.jpg","locationId":"loc-123","supportsSubscriptionSchedule":false}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodPost:
			rec.record("post-connect")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	if err := svc.RegisterProvider(context.Background(), integrationID, "loc-123", "test-access-token"); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	// 1. /provider occurs first.
	if len(rec.order) < 1 || rec.order[0] != "provider" {
		t.Fatalf("first request = %v, want provider first", rec.order)
	}
	// 2. GET /connect occurs after /provider.
	if rec.lastIndex("get-connect") < 1 {
		t.Fatalf("GET /connect did not occur after POST /provider: %v", rec.order)
	}
	// 3+4. The credential POST occurs exactly once, after the GET confirmed
	// the base config.
	if got := rec.count("post-connect"); got != 1 {
		t.Fatalf("credential POST /connect count = %d, want 1", got)
	}
	assertCredentialsPostAfterGet(t, rec)
}

// TestRegisterProvider_BaseConfigEventuallyReady verifies the bounded
// eventual-consistency retry: the GET /connect is retried while the base
// configuration does not exist yet, and the credential POST happens only
// after a confirming GET.
func TestRegisterProvider_BaseConfigEventuallyReady(t *testing.T) {
	t.Parallel()

	rec := newRequestRecorder()
	getAttempts := 0
	svc, integrationID, _ := newSequencingTestService(t, sequencingHandler(func(method, path string, w http.ResponseWriter) {
		switch {
		case path == "/payments/custom-provider/provider" && method == http.MethodPost:
			rec.record("provider")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodGet:
			rec.record("get-connect")
			getAttempts++
			if getAttempts < 3 {
				// Base config not created yet (HighLevel 422).
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"message":"Base config for integration is not created yet. Please create the base config before updating integration keys."}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"RVPay","description":"RVPay payment provider","paymentsUrl":"https://checkout.example.com/payment/checkout","queryUrl":"https://api.example.com/payments/custom-provider/query","locationId":"loc-123"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodPost:
			rec.record("post-connect")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	if err := svc.RegisterProvider(context.Background(), integrationID, "loc-123", "test-access-token"); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	if got := rec.count("get-connect"); got != 3 {
		t.Fatalf("GET /connect attempts = %d, want 3", got)
	}
	if got := rec.count("post-connect"); got != 1 {
		t.Fatalf("credential POST /connect count = %d, want 1 (only after confirmation)", got)
	}
	assertCredentialsPostAfterGet(t, rec)
}

// TestRegisterProvider_TraceOnlyConfigRetriesThenConfirms verifies that an
// HTTP-success GET /connect that returns only a trace body ({"traceId":"..."})
// is NOT treated as proof that the base configuration exists: verification
// retries the GET and the credential POST happens only after a later GET
// returns real provider metadata.
func TestRegisterProvider_TraceOnlyConfigRetriesThenConfirms(t *testing.T) {
	t.Parallel()

	rec := newRequestRecorder()
	getAttempts := 0
	svc, integrationID, _ := newSequencingTestService(t, sequencingHandler(func(method, path string, w http.ResponseWriter) {
		switch {
		case path == "/payments/custom-provider/provider" && method == http.MethodPost:
			rec.record("provider")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodGet:
			rec.record("get-connect")
			getAttempts++
			if getAttempts < 3 {
				// HTTP 200 but the base configuration has not materialized
				// yet: HighLevel answers with a trace-only body.
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"traceId":"trace-abc"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"RVPay","description":"RVPay payment provider","paymentsUrl":"https://checkout.example.com/payment/checkout","queryUrl":"https://api.example.com/payments/custom-provider/query","locationId":"loc-123"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodPost:
			rec.record("post-connect")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	if err := svc.RegisterProvider(context.Background(), integrationID, "loc-123", "test-access-token"); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	// The trace-only GETs were retried until real metadata arrived.
	if got := rec.count("get-connect"); got != 3 {
		t.Fatalf("GET /connect attempts = %d, want 3", got)
	}
	// The credential POST was not made until the confirming GET.
	if got := rec.count("post-connect"); got != 1 {
		t.Fatalf("credential POST /connect count = %d, want 1", got)
	}
	assertCredentialsPostAfterGet(t, rec)
}

// TestRegisterProvider_CapabilitiesBeforeProviderRegistration verifies that
// the capabilities update (PUT /payments/custom-provider/capabilities) runs
// first in the registration flow, using the already-resolved locationId,
// before the existing provider → GET /connect → POST /connect sequence.
func TestRegisterProvider_CapabilitiesBeforeProviderRegistration(t *testing.T) {
	t.Parallel()

	rec := newRequestRecorder()
	svc, integrationID, configRepo := newSequencingTestService(t, sequencingHandler(func(method, path string, w http.ResponseWriter) {
		switch {
		case path == "/payments/custom-provider/capabilities" && method == http.MethodPut:
			rec.record("capabilities")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		case path == "/payments/custom-provider/provider" && method == http.MethodPost:
			rec.record("provider")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodGet:
			rec.record("get-connect")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"RVPay","description":"RVPay payment provider","paymentsUrl":"https://checkout.example.com/payment/checkout","queryUrl":"https://api.example.com/payments/custom-provider/query","locationId":"loc-123"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodPost:
			rec.record("post-connect")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	if err := svc.RegisterProvider(context.Background(), integrationID, "loc-123", "test-access-token"); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	// The capabilities update precedes provider registration.
	if len(rec.order) < 2 || rec.order[0] != "capabilities" || rec.order[1] != "provider" {
		t.Fatalf("request order = %v, want capabilities before provider", rec.order)
	}
	if got := rec.count("capabilities"); got != 1 {
		t.Fatalf("capabilities PUT count = %d, want 1", got)
	}
	// The existing sequence is preserved after the capabilities update.
	assertCredentialsPostAfterGet(t, rec)
	if got := rec.count("post-connect"); got != 1 {
		t.Fatalf("credential POST /connect count = %d, want 1", got)
	}
	// Local persistence is unaffected.
	if len(configRepo.configs) != 1 {
		t.Fatalf("expected 1 local provider config, got %d", len(configRepo.configs))
	}
}

// TestRegisterProvider_BaseConfigNeverReadySkipsCredentials verifies that a
// base configuration that never becomes available results in a bounded GET
// retry, NO credential POST, and a still-successful (non-fatal) registration
// so it can be retried later.
func TestRegisterProvider_BaseConfigNeverReadySkipsCredentials(t *testing.T) {
	t.Parallel()

	rec := newRequestRecorder()
	svc, integrationID, configRepo := newSequencingTestService(t, sequencingHandler(func(method, path string, w http.ResponseWriter) {
		switch {
		case path == "/payments/custom-provider/provider" && method == http.MethodPost:
			rec.record("provider")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodGet:
			rec.record("get-connect")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Base config for integration is not created yet."}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodPost:
			rec.record("post-connect")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	if err := svc.RegisterProvider(context.Background(), integrationID, "loc-123", "test-access-token"); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	// 5. Bounded retry around the GET only.
	if got := rec.count("get-connect"); got != baseConfigVerifyAttempts {
		t.Fatalf("GET /connect attempts = %d, want %d (bounded)", got, baseConfigVerifyAttempts)
	}
	// 6. Exhausted verification must not send credentials.
	if got := rec.count("post-connect"); got != 0 {
		t.Fatalf("credential POST /connect count = %d, want 0 (base config never confirmed)", got)
	}
	// The registration itself remains non-fatal: the local config is persisted.
	if len(configRepo.configs) != 1 {
		t.Fatalf("expected 1 local provider config, got %d", len(configRepo.configs))
	}
}

// TestRegisterProvider_BaseConfigUnauthorizedSkipsCredentials verifies that
// an unauthorized base-config verification is not treated as "not ready yet"
// (no retries), and that credentials are not sent.
func TestRegisterProvider_BaseConfigUnauthorizedSkipsCredentials(t *testing.T) {
	t.Parallel()

	rec := newRequestRecorder()
	svc, integrationID, _ := newSequencingTestService(t, sequencingHandler(func(method, path string, w http.ResponseWriter) {
		switch {
		case path == "/payments/custom-provider/provider" && method == http.MethodPost:
			rec.record("provider")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodGet:
			rec.record("get-connect")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodPost:
			rec.record("post-connect")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	if err := svc.RegisterProvider(context.Background(), integrationID, "loc-123", "test-access-token"); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	// 8. Unauthorized responses remain unauthorized: no retry loop.
	if got := rec.count("get-connect"); got != 1 {
		t.Fatalf("GET /connect attempts = %d, want 1 (unauthorized must not be retried)", got)
	}
	if got := rec.count("post-connect"); got != 0 {
		t.Fatalf("credential POST /connect count = %d, want 0 (unauthorized)", got)
	}
}

// TestRegisterProvider_ExistingAssociationStillConfirmsBaseConfig verifies
// the idempotent re-registration path: a 422 from POST /provider (association
// already exists) is confirmed via GET /connect, and the credential POST only
// happens after that confirmation.
func TestRegisterProvider_ExistingAssociationStillConfirmsBaseConfig(t *testing.T) {
	t.Parallel()

	rec := newRequestRecorder()
	svc, integrationID, configRepo := newSequencingTestService(t, sequencingHandler(func(method, path string, w http.ResponseWriter) {
		switch {
		case path == "/payments/custom-provider/provider" && method == http.MethodPost:
			rec.record("provider")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"provider already exists"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodGet:
			rec.record("get-connect")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"RVPay","description":"RVPay payment provider","paymentsUrl":"https://checkout.example.com/payment/checkout","queryUrl":"https://api.example.com/payments/custom-provider/query","locationId":"loc-123"}`))
		case path == "/payments/custom-provider/connect" && method == http.MethodPost:
			rec.record("post-connect")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"_id":"prov-1"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))

	if err := svc.RegisterProvider(context.Background(), integrationID, "loc-123", "test-access-token"); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	if got := rec.count("post-connect"); got != 1 {
		t.Fatalf("credential POST /connect count = %d, want 1", got)
	}
	assertCredentialsPostAfterGet(t, rec)
	// 10. The local config is still persisted with the remote metadata.
	if len(configRepo.configs) != 1 {
		t.Fatalf("expected 1 local provider config, got %d", len(configRepo.configs))
	}
}

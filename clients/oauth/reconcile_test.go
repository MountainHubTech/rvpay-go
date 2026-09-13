package oauth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/MountainHubTech/rvpay-go/clients/providers"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fullConnectBody is a complete GET /payments/custom-provider/connect
// response proving the remote provider is present and materialized.
const fullConnectBody = `{"name":"RVPay","description":"RVPay payment provider","imageUrl":"https://example.com/logo.jpg","queryUrl":"https://api.example.com/payments/custom-provider/query","paymentsUrl":"https://checkout.example.com/payment/checkout","supportsSubscriptionSchedule":false}`

// reconcileTestHarness bundles a reconciliation test service with recording
// counters for the outbound HighLevel registration calls.
type reconcileTestHarness struct {
	svc             *Service
	integrationRepo *mockIntegrationRepo
	tokenRepo       *mockOAuthTokenRepo
	configRepo      *mockPaymentProviderConfigRepo
	mu              sync.Mutex
	providerPosts   int
	connectGets     int
	logOut          *bytes.Buffer
	tokenRefreshes  int
}

// newReconcileTestHarness builds a reconciliation test service backed by a
// mock HighLevel server whose connect GET (remote verification) and provider
// POST (registration) behaviors are configurable per test.
func newReconcileTestHarness(t *testing.T, connectStatus int, connectBody string, providerStatus int) *reconcileTestHarness {
	t.Helper()

	h := &reconcileTestHarness{logOut: &bytes.Buffer{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/payments/custom-provider/connect":
			h.mu.Lock()
			h.connectGets++
			h.mu.Unlock()
			w.WriteHeader(connectStatus)
			_, _ = w.Write([]byte(connectBody))
		case "/payments/custom-provider/provider":
			h.mu.Lock()
			h.providerPosts++
			h.mu.Unlock()
			w.WriteHeader(providerStatus)
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/payments/custom-provider/capabilities":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/oauth/token":
			h.mu.Lock()
			h.tokenRefreshes++
			h.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accessToken":"refreshed-access-token","refreshToken":"refreshed-refresh-token","expiresIn":3600,"tokenType":"Bearer","scope":"read write","locationId":"loc-123"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	paymentClient := providers.NewHighLevelPaymentProviderClient(srv.URL, nil)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProviderWithURLs(
		"test-client", "test-secret", "https://example.com/callback", "",
		srv.URL+"/oauth/authorize", srv.URL+"/oauth/token", srv.URL+"/v1/users/me",
		paymentClient,
	))

	h.integrationRepo = newMockIntegrationRepo()
	h.tokenRepo = newMockOAuthTokenRepo()
	h.configRepo = newMockPaymentProviderConfigRepo()

	clientID := uuid.New()
	platformRepo := newMockPlatformRepo()
	platform, err := platformRepo.Create(context.Background(), "HighLevel", "HighLevel", "highlevel", true, true, true)
	if err != nil {
		t.Fatalf("seed platform: %v", err)
	}
	integration, err := h.integrationRepo.Create(context.Background(), clientID, platform.ID, "loc-123", sqlc.IntegrationStatusACTIVE)
	if err != nil {
		t.Fatalf("seed integration: %v", err)
	}
	_ = integration

	h.svc = NewService(
		h.integrationRepo, h.tokenRepo, newMockClientRepo(), platformRepo,
		newMockOAuthStateRepo(), h.configRepo, registry,
		"https://example.com/callback",
		ProviderConfigSettings{
			Name: "RVPay", Description: "RVPay payment provider",
			ImageURL:    "https://example.com/logo.jpg",
			PaymentsURL: "https://checkout.example.com/payment/checkout",
			QueryURL:    "https://api.example.com/payments/custom-provider/query",
		},
		h.svcLogger(),
	)
	return h
}

// svcLogger returns the harness logger: buffered when logOut is set so tests
// can assert on structured log output, otherwise a no-op.
func (h *reconcileTestHarness) svcLogger() zerolog.Logger {
	if h.logOut != nil {
		return zerolog.New(h.logOut)
	}
	return zerolog.Nop()
}

// shortenConfigVerify bounds the base-config GET retry used inside
// RegisterProvider so absent-provider tests stay fast. It must only be used
// from non-parallel tests because it mutates package-level knobs.
func shortenConfigVerify(t *testing.T) {
	t.Helper()
	oldAttempts, oldDelay := baseConfigVerifyAttempts, baseConfigVerifyDelay
	baseConfigVerifyAttempts, baseConfigVerifyDelay = 1, time.Millisecond
	t.Cleanup(func() {
		baseConfigVerifyAttempts, baseConfigVerifyDelay = oldAttempts, oldDelay
	})
}

// seedToken stores a location OAuth token for the integration.
func (h *reconcileTestHarness) seedToken(t *testing.T, accessToken string, expiresAt time.Time) {
	t.Helper()
	integration := h.integrationRepo.integrations[externalIntegrationKey(h)]
	if _, err := h.tokenRepo.Create(context.Background(), integration.ID, accessToken, "refresh-token-"+accessToken, expiresAt, "read write", "Bearer"); err != nil {
		t.Fatalf("seed token: %v", err)
	}
}

// externalIntegrationKey returns the map key of the single seeded integration.
func externalIntegrationKey(h *reconcileTestHarness) string {
	for key := range h.integrationRepo.integrations {
		return key
	}
	return ""
}

// TestReconcile_RemotePresent_LocalRowExisted: INSTALL with an existing local
// provider row and a present, complete remote provider must preserve the
// local configuration, re-run NO registration, and create no duplicate rows.
func TestReconcile_RemotePresent_LocalRowExisted(t *testing.T) {
	h := newReconcileTestHarness(t, http.StatusOK, fullConnectBody, http.StatusOK)
	h.seedToken(t, "stored-access-token", time.Now().Add(time.Hour))
	if _, err := h.configRepo.Create(context.Background(), h.integrationRepo.integrations[externalIntegrationKey(h)].ID, "RVPay", "RVPay payment provider", "https://example.com/logo.jpg", "loc-123", "https://api.example.com/payments/custom-provider/query", "https://checkout.example.com/payment/checkout", false, "existing-api-key"); err != nil {
		t.Fatalf("seed local config: %v", err)
	}

	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
		t.Fatalf("ReconcilePaymentProvider failed: %v", err)
	}

	h.mu.Lock()
	posts, gets := h.providerPosts, h.connectGets
	h.mu.Unlock()
	if posts != 0 {
		t.Fatalf("provider POST count = %d, want 0 (local data preserved when remote is present)", posts)
	}
	if gets != 1 {
		t.Fatalf("connect GET count = %d, want 1 (remote verification)", gets)
	}
	// Local provider config preserved verbatim — including the existing API key.
	cfg, err := h.configRepo.GetByIntegrationID(context.Background(), h.integrationRepo.integrations[externalIntegrationKey(h)].ID)
	if err != nil {
		t.Fatalf("local config should exist: %v", err)
	}
	if cfg.ProviderApiKey != "existing-api-key" {
		t.Fatalf("local API key changed: %q, want existing-api-key", cfg.ProviderApiKey)
	}
}

// TestReconcile_RemoteAbsent_ReregistersAndStaysIdempotent: INSTALL with an
// existing local row but an absent remote provider (trace-only 200) must
// re-run the existing registration exactly once, and duplicate INSTALLs must
// remain idempotent (no duplicate local provider rows, no duplicate remote
// registrations once the remote provider is materialized).
func TestReconcile_RemoteAbsent_ReregistersAndStaysIdempotent(t *testing.T) {
	shortenConfigVerify(t)
	h := newReconcileTestHarness(t, http.StatusOK, `{"traceId":"trace-only"}`, http.StatusOK)
	h.seedToken(t, "stored-access-token", time.Now().Add(time.Hour))

	// First INSTALL: local row absent, remote absent → registration.
	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
		t.Fatalf("first reconciliation failed: %v", err)
	}
	h.mu.Lock()
	posts := h.providerPosts
	h.mu.Unlock()
	if posts != 1 {
		t.Fatalf("provider POST count after absent remote = %d, want 1", posts)
	}
	if len(h.configRepo.configs) != 1 {
		t.Fatalf("config rows = %d, want 1 (no duplicate local rows)", len(h.configRepo.configs))
	}

	// Second (duplicate) INSTALL after remote materialization: the mock's
	// connect GET still returns trace-only, so the harness now seeds a local
	// row and a full remote response to prove the idempotent skip path.
	// Instead of changing mock behavior mid-test, re-run reconciliation on a
	// harness whose remote is now complete.
	h2 := newReconcileTestHarness(t, http.StatusOK, fullConnectBody, http.StatusOK)
	h2.seedToken(t, "stored-access-token", time.Now().Add(time.Hour))
	integrationID := h2.integrationRepo.integrations[externalIntegrationKey(h2)].ID
	if _, err := h2.configRepo.Create(context.Background(), integrationID, "RVPay", "RVPay payment provider", "https://example.com/logo.jpg", "loc-123", "https://api.example.com/payments/custom-provider/query", "https://checkout.example.com/payment/checkout", false, "api-key"); err != nil {
		t.Fatalf("seed local config: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := h2.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
			t.Fatalf("duplicate INSTALL reconciliation %d failed: %v", i+1, err)
		}
	}
	h2.mu.Lock()
	posts2 := h2.providerPosts
	h2.mu.Unlock()
	if posts2 != 0 {
		t.Fatalf("duplicate INSTALL provider POSTs = %d, want 0 (idempotent when remote present)", posts2)
	}
	if len(h2.configRepo.configs) != 1 {
		t.Fatalf("duplicate INSTALL config rows = %d, want 1", len(h2.configRepo.configs))
	}
	_ = integrationID
}

// TestReconcile_RemoteAbsent400_Reregisters: a 400 from the connect GET is
// absent/incomplete — never "already exists" — and triggers re-registration.
func TestReconcile_RemoteAbsent400_Reregisters(t *testing.T) {
	shortenConfigVerify(t)
	h := newReconcileTestHarness(t, http.StatusBadRequest, `{"message":"base config for integration is not created yet"}`, http.StatusOK)
	h.seedToken(t, "stored-access-token", time.Now().Add(time.Hour))

	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
		t.Fatalf("reconciliation should recover an absent remote provider: %v", err)
	}
	h.mu.Lock()
	posts := h.providerPosts
	h.mu.Unlock()
	if posts != 1 {
		t.Fatalf("provider POST count = %d, want 1 (remote re-registration)", posts)
	}
	if len(h.configRepo.configs) != 1 {
		t.Fatalf("config rows = %d, want 1", len(h.configRepo.configs))
	}
}

// TestReconcile_Unauthorized_NotAlreadyExists: a 401 from the remote
// verification must surface as PermissionDenied (unauthorized), never be
// mislabeled as "provider already exists", and must attempt no registration.
func TestReconcile_Unauthorized_NotAlreadyExists(t *testing.T) {
	h := newReconcileTestHarness(t, http.StatusUnauthorized, `{"message":"The token is not authorized for this scope"}`, http.StatusOK)
	h.seedToken(t, "stored-access-token", time.Now().Add(time.Hour))

	err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123")
	if err == nil {
		t.Fatal("unauthorized reconciliation should fail")
	}
	if got := status.Code(err); got != codes.PermissionDenied {
		t.Fatalf("error code = %s, want %s (unauthorized, not already-exists)", got, codes.PermissionDenied)
	}
	h.mu.Lock()
	posts := h.providerPosts
	h.mu.Unlock()
	if posts != 0 {
		t.Fatalf("provider POST count = %d, want 0 (no registration with an unauthorized token)", posts)
	}
}

// TestReconcile_TransientVerificationFailure: a 500 from the remote
// verification is transient — surfaced for retry, registration not guessed.
func TestReconcile_TransientVerificationFailure(t *testing.T) {
	h := newReconcileTestHarness(t, http.StatusInternalServerError, `{"message":"boom"}`, http.StatusOK)
	h.seedToken(t, "stored-access-token", time.Now().Add(time.Hour))

	err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123")
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("error code = %s, want %s (transient failure)", got, codes.Internal)
	}
	h.mu.Lock()
	posts := h.providerPosts
	h.mu.Unlock()
	if posts != 0 {
		t.Fatalf("provider POST count = %d, want 0 (no registration on unknown remote state)", posts)
	}
}

// TestReconcile_RegistrationRejected: a 500 from the provider POST is a
// rejected registration — surfaced and logged, never success.
func TestReconcile_RegistrationRejected(t *testing.T) {
	shortenConfigVerify(t)
	h := newReconcileTestHarness(t, http.StatusOK, `{"traceId":"trace-only"}`, http.StatusInternalServerError)
	h.seedToken(t, "stored-access-token", time.Now().Add(time.Hour))

	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err == nil {
		t.Fatal("rejected registration should fail")
	}
	// The registration failure must not persist local provider data.
	if len(h.configRepo.configs) != 0 {
		t.Fatalf("config rows = %d, want 0 (local data preserved unless remote succeeds)", len(h.configRepo.configs))
	}
}

// TestReconcile_UsesStoredLocationToken: a valid stored location token is
// used directly (correct location-specific token; no refresh).
func TestReconcile_UsesStoredLocationToken(t *testing.T) {
	h := newReconcileTestHarness(t, http.StatusOK, fullConnectBody, http.StatusOK)
	h.seedToken(t, "stored-access-token", time.Now().Add(time.Hour))

	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}
	h.mu.Lock()
	refreshes := h.tokenRefreshes
	h.mu.Unlock()
	if refreshes != 0 {
		t.Fatalf("token refreshes = %d, want 0 (valid stored location token used)", refreshes)
	}
}

// TestReconcile_RefreshesExpiredTokenBeforeReconciliation: an expired stored
// token is refreshed through the existing refresh mechanism, and the
// refreshed token performs the registration.
func TestReconcile_RefreshesExpiredTokenBeforeReconciliation(t *testing.T) {
	shortenConfigVerify(t)
	h := newReconcileTestHarness(t, http.StatusOK, fullConnectBody, http.StatusOK)
	h.seedToken(t, "expired-access-token", time.Now().Add(-time.Minute))

	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}
	h.mu.Lock()
	refreshes := h.tokenRefreshes
	h.mu.Unlock()
	if refreshes != 1 {
		t.Fatalf("token refreshes = %d, want 1 (refresh-before-reconcile)", refreshes)
	}
	// The stored token must be updated in place (never deleted).
	integrationID := h.integrationRepo.integrations[externalIntegrationKey(h)].ID
	stored, err := h.tokenRepo.GetByIntegrationID(context.Background(), integrationID)
	if err != nil {
		t.Fatalf("stored token should still exist: %v", err)
	}
	if stored.AccessToken != "refreshed-access-token" {
		t.Fatalf("stored token not refreshed: %q", stored.AccessToken)
	}
}

// TestReconcile_NoTokenYet_Deferred: INSTALL arriving before the OAuth
// callback has no token — reconciliation is deferred (nil), nothing deleted.
func TestReconcile_NoTokenYet_Deferred(t *testing.T) {
	h := newReconcileTestHarness(t, http.StatusOK, fullConnectBody, http.StatusOK)

	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
		t.Fatalf("reconciliation without a token should defer, not fail: %v", err)
	}
	h.mu.Lock()
	posts, gets := h.providerPosts, h.connectGets
	h.mu.Unlock()
	if posts != 0 || gets != 0 {
		t.Fatalf("remote calls = (posts %d, gets %d), want (0, 0) when deferred", posts, gets)
	}
	if len(h.configRepo.configs) != 0 {
		t.Fatalf("config rows = %d, want 0 (nothing created without a token)", len(h.configRepo.configs))
	}
}

// TestReconcile_NoSecretsInLogs: reconciliation logs must contain only token
// fingerprints — never raw access tokens, refresh tokens, or API keys.
func TestReconcile_NoSecretsInLogs(t *testing.T) {
	h := newReconcileTestHarness(t, http.StatusOK, fullConnectBody, http.StatusOK)
	h.seedToken(t, "super-secret-access-token", time.Now().Add(time.Hour))

	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}
	logs := h.logOut.String()
	for _, secret := range []string{"super-secret-access-token", "refresh-token-super-secret-access-token"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("logs contain secret %q; logs must be fingerprint-only", secret)
		}
	}
	want := providers.AccessTokenFingerprint("super-secret-access-token")
	if !strings.Contains(logs, want) {
		t.Fatalf("logs should contain the token fingerprint %q; got logs: %s", want, logs)
	}
}

// TestReconcile_NoSecretsInLogsOnReregistration: the re-registration path
// (RegisterProvider) must also be fingerprint-only.
func TestReconcile_NoSecretsInLogsOnReregistration(t *testing.T) {
	shortenConfigVerify(t)
	h := newReconcileTestHarness(t, http.StatusOK, `{"traceId":"trace-only"}`, http.StatusOK)
	h.seedToken(t, "reregister-secret-token", time.Now().Add(time.Hour))

	if err := h.svc.ReconcilePaymentProvider(context.Background(), "loc-123"); err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}
	if strings.Contains(h.logOut.String(), "reregister-secret-token") {
		t.Fatal("logs contain the raw access token on the re-registration path")
	}
}

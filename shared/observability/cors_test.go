package observability

import (
	"github.com/rs/zerolog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// corsTestHandler records whether it was reached and always answers 200 OK.
type corsTestHandler struct {
	reached bool
}

func (h *corsTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.reached = true
	w.WriteHeader(http.StatusOK)
}

func TestCORS_AllowedOriginReceivesAllowOrigin(t *testing.T) {
	handler := &corsTestHandler{}
	srv := httptest.NewServer(CORS(zerolog.Nop(), []string{"http://localhost:3000", "https://admindashboard.rvpay.xyz"}, handler))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/public/clients/sub-accounts?page=1&pageSize=20", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:3000")
	}
	if got := resp.Header.Get("Vary"); !containsToken(got, "Origin") {
		t.Errorf("Vary = %q, want it to contain Origin", got)
	}
	if !handler.reached {
		t.Error("allowed-origin request did not reach the wrapped handler")
	}
}

func TestCORS_DisallowedOriginGetsNoCORSHeaders(t *testing.T) {
	handler := &corsTestHandler{}
	srv := httptest.NewServer(CORS(zerolog.Nop(), []string{"http://localhost:3000"}, handler))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/public/clients/sub-accounts", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	// The request still executes (CORS is a browser read policy, not auth),
	// but no CORS authorization may be granted to the unapproved origin.
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for unapproved origin", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("Access-Control-Allow-Methods = %q, want empty for unapproved origin", got)
	}
	if !handler.reached {
		t.Error("non-preflight request should still reach the wrapped handler")
	}
}
func TestCORS_PreflightAnsweredAtTransportLayer(t *testing.T) {
	handler := &corsTestHandler{}
	srv := httptest.NewServer(CORS(zerolog.Nop(), []string{"http://localhost:3000"}, handler))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/v1/public/deposits", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:3000")
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, "GET, POST, OPTIONS")
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, "Content-Type")
	}
	// Critical: preflight must NOT enter business logic / repository /
	// payment processing. The wrapped handler represents all of those.
	if handler.reached {
		t.Error("preflight OPTIONS reached the wrapped handler; it must be answered at the transport layer")
	}
}

func TestCORS_PreflightFromUnapprovedOriginNotAuthorized(t *testing.T) {
	handler := &corsTestHandler{}
	srv := httptest.NewServer(CORS(zerolog.Nop(), []string{"http://localhost:3000"}, handler))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/v1/public/deposits", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for unapproved preflight", got)
	}
}

func TestCORS_AllowedOriginPostCarriesAllowOrigin(t *testing.T) {
	handler := &corsTestHandler{}
	srv := httptest.NewServer(CORS(zerolog.Nop(), []string{"https://admindashboard.rvpay.xyz"}, handler))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/public/deposits", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "https://admindashboard.rvpay.xyz")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://admindashboard.rvpay.xyz" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://admindashboard.rvpay.xyz")
	}
	if !handler.reached {
		t.Error("POST did not reach the wrapped handler")
	}
}

func TestCORS_RequestWithoutOriginPassesThrough(t *testing.T) {
	handler := &corsTestHandler{}
	srv := httptest.NewServer(CORS(zerolog.Nop(), []string{"http://localhost:3000"}, handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty when no Origin header is sent", got)
	}
	if !handler.reached {
		t.Error("same-origin/no-origin request should reach the wrapped handler")
	}
}

func TestParseAllowedOrigins(t *testing.T) {
	got := ParseAllowedOrigins("https://admindashboard.rvpay.xyz, http://localhost:3000 ,,")
	want := []string{"https://admindashboard.rvpay.xyz", "http://localhost:3000"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("origins[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// containsToken reports whether a comma-separated header value contains the
// given token.
func containsToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.TrimSpace(part) == token {
			return true
		}
	}
	return false
}

// Regression tests for the runtime CORS configuration resolution: the running
// services previously loaded a stale HTTP_CORS_ALLOWED_ORIGINS value without
// the local Dashboard origin, so browsers got 200 responses they could not
// read. ResolveAllowedOrigins must always include the local dev origin and
// fall back to the documented defaults when the configuration is empty.
func TestResolveAllowedOrigins_EmptyFallsBackToDefaults(t *testing.T) {
	got := ResolveAllowedOrigins("")
	want := DefaultAllowedOrigins()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("origins[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveAllowedOrigins_AlwaysIncludesLocalDevOrigin(t *testing.T) {
	// A stale config listing only the deployed origin must still gain the
	// local development origin.
	got := ResolveAllowedOrigins("https://admindashboard.rvpay.xyz")
	found := false
	for _, origin := range got {
		if origin == LocalDevOrigin {
			found = true
		}
	}
	if !found {
		t.Errorf("ResolveAllowedOrigins = %v, want it to include %q", got, LocalDevOrigin)
	}
	if len(got) != 2 || got[0] != "https://admindashboard.rvpay.xyz" {
		t.Errorf("ResolveAllowedOrigins = %v, want configured origin first", got)
	}
}

func TestResolveAllowedOrigins_DoesNotDuplicateLocalDevOrigin(t *testing.T) {
	got := ResolveAllowedOrigins("http://localhost:3000,https://admindashboard.rvpay.xyz")
	count := 0
	for _, origin := range got {
		if origin == LocalDevOrigin {
			count++
		}
	}
	if count != 1 {
		t.Errorf("LocalDevOrigin appears %d times in %v, want exactly 1", count, got)
	}
}

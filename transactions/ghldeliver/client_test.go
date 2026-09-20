package ghldeliver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHTTPPostSuccess verifies a 2xx response marks success and carries the
// agreed transport headers.
func TestHTTPPostSuccess(t *testing.T) {
	t.Parallel()

	var gotEventID, gotEventType, gotContentType, gotAccept, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEventID = r.Header.Get("X-RVPay-Event-Id")
		gotEventType = r.Header.Get("X-RVPay-Event")
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	poster := NewHTTPPoster(ts.URL, DefaultTimeout)
	if err := poster.Post(context.Background(), "evt-123", []byte(`{"event":"rvpay.payment.completed"}`)); err != nil {
		t.Fatalf("Post failed: %v", err)
	}

	if gotEventID != "evt-123" {
		t.Errorf("X-RVPay-Event-Id = %q, want evt-123", gotEventID)
	}
	if gotEventType != EventType {
		t.Errorf("X-RVPay-Event = %q, want %q", gotEventType, EventType)
	}
	if gotContentType != "application/json" || gotAccept != "application/json" {
		t.Errorf("content-type/accept = %q/%q, want application/json", gotContentType, gotAccept)
	}
	if !strings.HasPrefix(gotUA, "RVPay/") {
		t.Errorf("user-agent = %q, want the RVPay/<version> convention", gotUA)
	}
}

// TestHTTPPostClassifications verifies the retry classification and that
// error messages never embed the configured webhook URL.
func TestHTTPPostClassifications(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		status     int
		wantKind   FailureKind
		wantStatus int
	}{
		{name: "permanent 4xx", status: http.StatusBadRequest, wantKind: FailurePermanent, wantStatus: 400},
		{name: "permanent 404", status: http.StatusNotFound, wantKind: FailurePermanent, wantStatus: 404},
		{name: "retryable 408", status: http.StatusRequestTimeout, wantKind: FailureTransient, wantStatus: 408},
		{name: "retryable 429", status: http.StatusTooManyRequests, wantKind: FailureTransient, wantStatus: 429},
		{name: "retryable 500", status: http.StatusInternalServerError, wantKind: FailureTransient, wantStatus: 500},
		{name: "retryable 503", status: http.StatusServiceUnavailable, wantKind: FailureTransient, wantStatus: 503},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer ts.Close()

			poster := NewHTTPPoster(ts.URL, DefaultTimeout)
			err := poster.Post(context.Background(), "evt-1", []byte(`{}`))
			if err == nil {
				t.Fatalf("expected an error for HTTP %d", tc.status)
			}
			var deliveryErr *DeliveryError
			if !errors.As(err, &deliveryErr) {
				t.Fatalf("expected *DeliveryError, got %T: %v", err, err)
			}
			if deliveryErr.Kind != tc.wantKind {
				t.Errorf("kind = %v, want %v", deliveryErr.Kind, tc.wantKind)
			}
			if deliveryErr.HTTPStatus != tc.wantStatus {
				t.Errorf("http status = %d, want %d", deliveryErr.HTTPStatus, tc.wantStatus)
			}
			// The configured webhook URL is secret configuration; it must
			// never surface in error messages or logs.
			if strings.Contains(err.Error(), ts.URL) {
				t.Errorf("error message leaks the webhook URL: %q", err.Error())
			}
		})
	}
}

// TestHTTPPostNetworkError verifies a connection failure is transient and
// URL-free.
func TestHTTPPostNetworkError(t *testing.T) {
	t.Parallel()

	// Nothing listens on this port: the dial fails.
	poster := NewHTTPPoster("http://127.0.0.1:1/nope", 2*time.Second)
	err := poster.Post(context.Background(), "evt-1", []byte(`{}`))
	if err == nil {
		t.Fatal("expected a transient network error")
	}
	var deliveryErr *DeliveryError
	if !errors.As(err, &deliveryErr) || deliveryErr.Kind != FailureTransient {
		t.Fatalf("expected a transient DeliveryError, got: %v", err)
	}
	if strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Errorf("error message leaks the webhook URL: %q", err.Error())
	}
}

// TestHTTPPostTimeout verifies a bounded timeout is classified transient and
// does not leak the URL.
func TestHTTPPostTimeout(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer ts.Close()

	poster := NewHTTPPoster(ts.URL, 100*time.Millisecond)
	err := poster.Post(context.Background(), "evt-1", []byte(`{}`))
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	var deliveryErr *DeliveryError
	if !errors.As(err, &deliveryErr) || deliveryErr.Kind != FailureTransient {
		t.Fatalf("expected a transient DeliveryError, got: %v", err)
	}
	if strings.Contains(err.Error(), ts.URL) {
		t.Errorf("error message leaks the webhook URL: %q", err.Error())
	}
}

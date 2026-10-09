package ghldeliver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// UserAgent identifies RVPay to the HighLevel inbound webhook endpoint.
const UserAgent = "RVPay/1.0"

// DefaultTimeout bounds a single delivery attempt.
const DefaultTimeout = 10 * time.Second

// Classification of a delivery failure. Transient failures are retried
// according to the bounded retry policy; permanent failures are recorded and
// never retried, so a HighLevel configuration problem cannot become an
// infinite background retry loop.
type FailureKind int

const (
	// FailureTransient covers timeouts, network/connection errors and HTTP
	// 5xx responses.
	FailureTransient FailureKind = iota
	// FailurePermanent covers HTTP 4xx responses other than 408/429.
	FailurePermanent
)

// DeliveryError carries the failure classification. Its message is safe to
// log: it never embeds the configured webhook URL (net/http url.Error
// messages contain the full URL and are therefore never forwarded).
type DeliveryError struct {
	Kind       FailureKind
	HTTPStatus int
	msg        string
}

func (e *DeliveryError) Error() string { return e.msg }

// transientError and permanentError build URL-free error messages.
func transientError(format string, args ...any) error {
	return &DeliveryError{Kind: FailureTransient, msg: fmt.Sprintf(format, args...)}
}

// transientHTTPError is a transient failure caused by a retryable HTTP
// status; the status is retained for observability logs.
func transientHTTPError(status int, msg string) error {
	return &DeliveryError{Kind: FailureTransient, HTTPStatus: status, msg: msg}
}

func permanentError(status int, format string, args ...any) error {
	return &DeliveryError{Kind: FailurePermanent, HTTPStatus: status, msg: fmt.Sprintf(format, args...)}
}

// ValidateInboundWebhookURL reports whether the configured value is a usable
// HTTPS URL. The value itself is never returned or logged; callers log only
// the HIGHLEVEL_INBOUND_WEBHOOK_URL variable name when validation fails.
func ValidateInboundWebhookURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// Poster delivers one event payload to the configured HighLevel inbound
// webhook URL. The payload bytes are the immutable outbox snapshot; every
// retry reuses the identical bytes, event id and idempotency key.
type Poster interface {
	Post(ctx context.Context, eventID string, payload []byte) error
}

// HTTPPoster posts the event JSON to HIGHLEVEL_INBOUND_WEBHOOK_URL.
type HTTPPoster struct {
	url        string
	httpClient *http.Client
}

// NewHTTPPoster builds a poster for the configured inbound webhook URL. The
// caller is expected to have validated it with ValidateInboundWebhookURL.
func NewHTTPPoster(rawURL string, timeout time.Duration) *HTTPPoster {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &HTTPPoster{
		url:        rawURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Post sends one delivery attempt with the agreed transport contract:
// Content-Type/Accept application/json, a RVPay User-Agent, and the event
// type/id headers. The configured URL and the payload are never logged, and
// transport errors are stripped of the URL that net/http embeds in them.
func (p *HTTPPoster) Post(ctx context.Context, eventID string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytesReader(payload))
	if err != nil {
		// A construction failure would embed the URL; keep it out.
		return transientError("highlevel webhook request could not be constructed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-RVPay-Event", EventType)
	req.Header.Set("X-RVPay-Event-Id", eventID)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		// err may be a *url.Error whose message contains the full webhook
		// URL. Classify it and return a URL-free message instead. Context
		// deadline errors are timeouts; everything else is a network error.
		if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, context.DeadlineExceeded) {
			return transientError("highlevel webhook delivery timed out")
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) && errors.Is(urlErr.Err, context.DeadlineExceeded) {
			return transientError("highlevel webhook delivery timed out")
		}
		return transientError("highlevel webhook delivery failed (network error)")
	}
	defer func() { _ = resp.Body.Close() }()
	// The response body is drained (bounded) so the connection can be reused,
	// but its content is never read into logs: the endpoint is a workflow
	// trigger, not an API contract.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests:
		return transientHTTPError(resp.StatusCode, "highlevel webhook delivery retryable HTTP status "+strconv.Itoa(resp.StatusCode))
	case resp.StatusCode >= 500:
		return transientHTTPError(resp.StatusCode, "highlevel webhook delivery server error HTTP "+strconv.Itoa(resp.StatusCode))
	default:
		return permanentError(resp.StatusCode, "%s", "highlevel webhook delivery rejected (HTTP "+strconv.Itoa(resp.StatusCode)+")")
	}
}

// bytesReader is a small helper keeping the imports of the poster focused.
func bytesReader(b []byte) io.Reader {
	return &byteSliceReader{b: b}
}

// byteSliceReader is a minimal io.Reader over a byte slice.
type byteSliceReader struct {
	b []byte
	i int
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}

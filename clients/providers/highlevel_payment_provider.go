package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HighLevelPaymentProviderClient implements PaymentProviderClient for
// HighLevel's Custom Payment Provider API. It makes authenticated outbound
// calls to HighLevel using the installed location's OAuth access token.
//
// The access token is used only in the Authorization header and is never
// logged or included in returned errors.
type HighLevelPaymentProviderClient struct {
	// baseURL is the HighLevel API base URL (e.g. https://services.leadconnectorhq.com).
	baseURL string
	// httpClient is the shared HTTP client used for outbound calls.
	httpClient *http.Client
}

// NewHighLevelPaymentProviderClient creates a new HighLevel payment provider
// client. baseURL is the HighLevel API base URL; it must not include a
// trailing slash. httpClient may be nil, in which case a default client with
// a 10-second timeout is used.
func NewHighLevelPaymentProviderClient(baseURL string, httpClient *http.Client) *HighLevelPaymentProviderClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &HighLevelPaymentProviderClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
	}
}

// CreateProviderAssociation registers the Custom Payment Provider for the
// supplied HighLevel location. Per the HighLevel v3 contract, `locationId`
// is a required QUERY parameter and the provider metadata
// (name, description, paymentsUrl, queryUrl, imageUrl,
// supportsSubscriptionSchedule) is sent in the JSON body. This call is what
// makes RVPay appear and work on HighLevel's Payments > Integrations page.
//
// POST /payments/custom-provider/provider?locationId=<id>
func (c *HighLevelPaymentProviderClient) CreateProviderAssociation(ctx context.Context, accessToken string, cfg ProviderConfig) error {
	if strings.TrimSpace(accessToken) == "" {
		return ErrMissingAccessToken
	}
	if strings.TrimSpace(cfg.LocationID) == "" {
		return ErrMissingLocationID
	}

	// locationId is a required query parameter per the v3 create-integration
	// contract. It is NOT part of the JSON body.
	q := url.Values{}
	q.Set("locationId", cfg.LocationID)
	path := "/payments/custom-provider/provider" + "?" + q.Encode()

	// The provider metadata is sent in the JSON body. This is what registers
	// RVPay as the Custom Payment Provider for the location and what HighLevel
	// displays on the Payments > Integrations page.
	body := map[string]interface{}{
		"name":                         cfg.Name,
		"description":                  cfg.Description,
		"paymentsUrl":                  cfg.PaymentsURL,
		"queryUrl":                     cfg.QueryURL,
		"imageUrl":                     cfg.ImageURL,
		"supportsSubscriptionSchedule": cfg.SupportsSubscriptionSchedule,
	}

	var respBody map[string]interface{}
	if err := c.doJSON(ctx, http.MethodPost, path, accessToken, body, &respBody); err != nil {
		return err
	}

	return nil
}

// CreateProviderConfigs pushes the RVPay live/test processing keys
// (apiKey + publishableKey) to HighLevel for the supplied location. Per the
// HighLevel v3 contract, `locationId` is a required QUERY parameter and the
// credentials are sent in the JSON body. This is what lets the location
// transact through RVPay once the provider association exists.
//
// POST /payments/custom-provider/connect?locationId=<id>
// Body: {live:{apiKey,publishableKey,liveMode}, test:{apiKey,publishableKey,liveMode}}
func (c *HighLevelPaymentProviderClient) CreateProviderConfigs(ctx context.Context, accessToken, locationID string, creds ProviderCredentials) error {
	// Identical behavior to the diagnostics variant; the diagnostics are
	// simply discarded here so the existing contract is unchanged.
	_, err := c.CreateProviderConfigsWithDiagnostics(ctx, accessToken, locationID, creds)
	return err
}

// CreateProviderConfigsWithDiagnostics performs the same credential push as
// CreateProviderConfigs and additionally returns diagnostic details of the
// actual HighLevel HTTP response (HTTP status, sanitized response body,
// HighLevel traceId when present). The diagnostics never contain credentials
// or the access token; error semantics are identical.
func (c *HighLevelPaymentProviderClient) CreateProviderConfigsWithDiagnostics(ctx context.Context, accessToken, locationID string, creds ProviderCredentials) (*HighLevelCallDiagnostics, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, ErrMissingAccessToken
	}
	if strings.TrimSpace(locationID) == "" {
		return nil, ErrMissingLocationID
	}

	// locationId is a required query parameter per the v3 contract. It is
	// NOT part of the JSON body.
	q := url.Values{}
	q.Set("locationId", locationID)
	path := "/payments/custom-provider/connect" + "?" + q.Encode()

	// The live/test processing keys are sent in the JSON body. liveMode
	// reflects the environment each key set belongs to.
	body := map[string]interface{}{
		"live": map[string]interface{}{
			"apiKey":         creds.Live.APIKey,
			"publishableKey": creds.Live.PublishableKey,
			"liveMode":       true,
		},
		"test": map[string]interface{}{
			"apiKey":         creds.Test.APIKey,
			"publishableKey": creds.Test.PublishableKey,
			"liveMode":       false,
		},
	}

	var respBody map[string]interface{}
	return c.doJSONDiag(ctx, http.MethodPost, path, accessToken, body, &respBody)
}

// UpdateProviderCapabilities enables the RVPay Custom Payment Provider
// capabilities for the supplied HighLevel location. Per the HighLevel v3
// contract, `locationId` is sent in the JSON body (NOT as a query parameter)
// and `supportsSubscriptionSchedules` is false because RVPay supports
// one-time payments only. No companyId is sent. This is what lets the
// location use RVPay as its Custom Payment Provider once registered.
//
// PUT /payments/custom-provider/capabilities
// Body: {locationId, supportsSubscriptionSchedules:false}
func (c *HighLevelPaymentProviderClient) UpdateProviderCapabilities(ctx context.Context, accessToken, locationID string) error {
	if strings.TrimSpace(accessToken) == "" {
		return ErrMissingAccessToken
	}
	if strings.TrimSpace(locationID) == "" {
		return ErrMissingLocationID
	}

	path := "/payments/custom-provider/capabilities"

	// locationId is a required JSON body field per the v3 capabilities
	// contract. supportsSubscriptionSchedules is false: RVPay supports
	// one-time payments only. companyId is not sent.
	body := map[string]interface{}{
		"locationId":                    locationID,
		"supportsSubscriptionSchedules": false,
	}

	var respBody map[string]interface{}
	if err := c.doJSON(ctx, http.MethodPut, path, accessToken, body, &respBody); err != nil {
		return err
	}

	return nil
}

// FetchProviderConfig fetches the provider configuration for a location.
//
// GET /payments/custom-provider/connect
func (c *HighLevelPaymentProviderClient) FetchProviderConfig(ctx context.Context, accessToken, locationID string) (*ProviderConfig, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, ErrMissingAccessToken
	}
	if strings.TrimSpace(locationID) == "" {
		return nil, ErrMissingLocationID
	}

	path := "/payments/custom-provider/connect"
	q := url.Values{}
	q.Set("locationId", locationID)
	fullPath := path + "?" + q.Encode()

	var respBody struct {
		Name                         string `json:"name"`
		Description                  string `json:"description"`
		ImageURL                     string `json:"imageUrl"`
		LocationID                   string `json:"locationId"`
		QueryURL                     string `json:"queryUrl"`
		PaymentsURL                  string `json:"paymentsUrl"`
		SupportsSubscriptionSchedule bool   `json:"supportsSubscriptionSchedule"`
	}

	if err := c.doJSON(ctx, http.MethodGet, fullPath, accessToken, nil, &respBody); err != nil {
		return nil, err
	}

	return &ProviderConfig{
		Name:                         respBody.Name,
		Description:                  respBody.Description,
		ImageURL:                     respBody.ImageURL,
		LocationID:                   respBody.LocationID,
		QueryURL:                     respBody.QueryURL,
		PaymentsURL:                  respBody.PaymentsURL,
		SupportsSubscriptionSchedule: respBody.SupportsSubscriptionSchedule,
	}, nil
}

// DisconnectProvider disconnects the provider configuration for a location.
//
// DELETE /payments/custom-provider/connect
func (c *HighLevelPaymentProviderClient) DisconnectProvider(ctx context.Context, accessToken, locationID string) error {
	if strings.TrimSpace(accessToken) == "" {
		return ErrMissingAccessToken
	}
	if strings.TrimSpace(locationID) == "" {
		return ErrMissingLocationID
	}

	path := "/payments/custom-provider/connect"
	q := url.Values{}
	q.Set("locationId", locationID)
	fullPath := path + "?" + q.Encode()

	var respBody map[string]interface{}
	if err := c.doJSON(ctx, http.MethodDelete, fullPath, accessToken, nil, &respBody); err != nil {
		return err
	}

	return nil
}

// doJSON performs an authenticated JSON request to the HighLevel API. It
// handles 2xx, 400, 401, and 422 responses and returns typed/domain errors.
// The access token is never logged or included in returned errors.
func (c *HighLevelPaymentProviderClient) doJSON(ctx context.Context, method, path, accessToken string, body, out interface{}) error {
	_, err := c.doJSONDiag(ctx, method, path, accessToken, body, out)
	return err
}

// doJSONDiag performs the same authenticated JSON request as doJSON and
// additionally returns diagnostic details of the actual HighLevel HTTP
// response (HTTP status, sanitized response body, HighLevel traceId when
// present) for both success and error outcomes. The diagnostics never
// contain credentials or the access token; error behavior is identical.
func (c *HighLevelPaymentProviderClient) doJSONDiag(ctx context.Context, method, path, accessToken string, body, out interface{}) (*HighLevelCallDiagnostics, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Version", "v3")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if out != nil && len(respData) > 0 {
			if err := json.Unmarshal(respData, out); err != nil {
				return nil, fmt.Errorf("parse response body: %w", err)
			}
		}
		return &HighLevelCallDiagnostics{
			StatusCode: resp.StatusCode,
			Body:       sanitizeErrorBody(respData),
			TraceID:    traceIDFromBody(respData),
		}, nil
	case resp.StatusCode == http.StatusBadRequest:
		return nil, wrapHighLevelAPIError(resp.StatusCode, respData, fmt.Errorf("%w: %s", ErrBadRequest, sanitizeErrorBody(respData)))
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, wrapHighLevelAPIError(resp.StatusCode, respData, fmt.Errorf("%w: %s", ErrUnauthorized, sanitizeErrorBody(respData)))
	case resp.StatusCode == http.StatusUnprocessableEntity:
		return nil, wrapHighLevelAPIError(resp.StatusCode, respData, fmt.Errorf("%w: %s", ErrUnprocessableEntity, sanitizeErrorBody(respData)))
	default:
		return nil, wrapHighLevelAPIError(resp.StatusCode, respData, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, sanitizeErrorBody(respData)))
	}
}

// HighLevelAPIError carries diagnostic details of a non-2xx HighLevel
// response (HTTP status, the sanitized response body, and the HighLevel
// traceId when present) alongside the original typed error. It does NOT
// change the error message or sentinel classification: Error and Unwrap
// delegate to the wrapped error so errors.Is/As behavior and the existing
// message format are preserved. The body is already sanitized
// (credential-redacted) and never contains the access token.
type HighLevelAPIError struct {
	// StatusCode is the HTTP status returned by HighLevel.
	StatusCode int
	// Body is the sanitized (credential-redacted) HighLevel response body.
	Body string
	// TraceID is the HighLevel traceId from the response body, if present.
	TraceID string
	err     error
}

// Error returns the original error message unchanged.
func (e *HighLevelAPIError) Error() string { return e.err.Error() }

// Unwrap exposes the original typed error so errors.Is/As classification
// (e.g. providers.ErrBadRequest) keeps working.
func (e *HighLevelAPIError) Unwrap() error { return e.err }

// wrapHighLevelAPIError attaches diagnostic response details to err without
// altering its message or sentinel classification.
func wrapHighLevelAPIError(statusCode int, body []byte, err error) error {
	return &HighLevelAPIError{
		StatusCode: statusCode,
		Body:       sanitizeErrorBody(body),
		TraceID:    traceIDFromBody(body),
		err:        err,
	}
}

// traceIDFromBody extracts the HighLevel traceId from a response body, if
// present. It never returns credential material — only the traceId field.
func traceIDFromBody(body []byte) string {
	var raw struct {
		TraceID string `json:"traceId"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	return raw.TraceID
}

// credentialFields are JSON field names whose values must be redacted from
// error messages to prevent credential leakage.
var credentialFields = []string{
	"access_token",
	"refresh_token",
	"client_secret",
	"apiKey",
	"api_key",
}

// sanitizeErrorBody truncates and sanitizes an error response body so that
// sensitive data (including access tokens, refresh tokens, and API keys) is
// never leaked into errors. It attempts to parse the body as JSON and redact
// known credential fields. If parsing fails, it falls back to length-limited
// plain-text truncation.
func sanitizeErrorBody(body []byte) string {
	if len(body) == 0 {
		return "empty response body"
	}

	// Attempt to redact credential fields from JSON bodies.
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err == nil {
		for _, field := range credentialFields {
			if _, ok := raw[field]; ok {
				raw[field] = "[REDACTED]"
			}
		}
		sanitized, err := json.Marshal(raw)
		if err == nil {
			body = sanitized
		}
	}

	// Truncate to a reasonable length to avoid leaking large payloads.
	if len(body) > 512 {
		body = body[:512]
	}
	return string(body)
}

// Sentinel errors returned by the HighLevel payment provider client.
var (
	// ErrMissingAccessToken is returned when no access token is provided.
	ErrMissingAccessToken = errors.New("access token is required")
	// ErrMissingLocationID is returned when no location ID is provided.
	ErrMissingLocationID = errors.New("location ID is required")
	// ErrBadRequest is returned when HighLevel responds with 400.
	ErrBadRequest = errors.New("highlevel: bad request")
	// ErrUnauthorized is returned when HighLevel responds with 401.
	ErrUnauthorized = errors.New("highlevel: unauthorized")
	// ErrUnprocessableEntity is returned when HighLevel responds with 422.
	ErrUnprocessableEntity = errors.New("highlevel: unprocessable entity")
)

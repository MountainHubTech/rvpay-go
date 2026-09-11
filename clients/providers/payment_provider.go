package providers

import "context"

// PaymentProviderClient defines the outbound HighLevel Custom Payment Provider
// API operations. These are outbound integrations performed by the Clients
// service on behalf of an installed location; they are NOT exposed as RVPay
// public endpoints.
//
// Each operation requires the installed location's OAuth access token. The
// access token is used only to authenticate the outbound call and is never
// logged or returned in errors.
//
// GHL v3 distinction:
//   - The association (POST /payments/custom-provider/provider) registers the
//     provider NAME/URLs/LOGO metadata for a location and is what makes RVPay
//     appear and work on HighLevel's Payments > Integrations page.
//   - FetchProviderConfig (GET /payments/custom-provider/connect) retrieves the
//     existing provider configuration for a location (same provider metadata
//     shape).
//   - CreateProviderConfigs (POST /payments/custom-provider/connect) pushes
//     RVPay's live/test processing keys (apiKey + publishableKey) to HighLevel
//     so the location can transact through RVPay.
type PaymentProviderClient interface {
	// CreateProviderAssociation registers the RVPay Custom Payment Provider for
	// the supplied HighLevel location, sending the provider metadata.
	//
	// POST /payments/custom-provider/provider?locationId=<id>
	// Body: {name, description, paymentsUrl, queryUrl, imageUrl, supportsSubscriptionSchedule}
	CreateProviderAssociation(ctx context.Context, accessToken string, cfg ProviderConfig) error

	// CreateProviderConfigs pushes the RVPay live/test processing keys
	// (apiKey + publishableKey) to HighLevel for the supplied location.
	//
	// POST /payments/custom-provider/connect?locationId=<id>
	// Body: {live:{apiKey,publishableKey,liveMode}, test:{apiKey,publishableKey,liveMode}}
	CreateProviderConfigs(ctx context.Context, accessToken, locationID string, creds ProviderCredentials) error

	// FetchProviderConfig fetches the existing provider configuration for a
	// location, returning the real provider metadata registered with HighLevel.
	//
	// GET /payments/custom-provider/connect?locationId=<id>
	FetchProviderConfig(ctx context.Context, accessToken, locationID string) (*ProviderConfig, error)

	// DisconnectProvider disconnects the provider configuration for a location.
	//
	// DELETE /payments/custom-provider/connect?locationId=<id>
	DisconnectProvider(ctx context.Context, accessToken, locationID string) error

	// UpdateOrderStatus pushes a PawaPay-authoritative terminal payment result
	// to GoHighLevel for a location's order so the order leaves "pending".
	// accessToken is the installed location's OAuth access token; it is used
	// only in the Authorization header and is never logged or returned in
	// errors. status must be GhlOrderStatusCompleted or GhlOrderStatusFailed;
	// any other value is a caller error. amount is the deposit amount in minor
	// currency units (e.g., cents) recorded against the GHL order. The concrete
	// v3 operation is the order payment record endpoint; the exact GHL path/shape
	// is isolated to the HighLevelPaymentProviderClient implementation and
	// requires full-deployment verification (see agents/
	// ghl-v3-payment-status-synchronization.md §8).
	UpdateOrderStatus(ctx context.Context, accessToken, locationID, orderID string, status GhlOrderStatus, amount int64) error

	// CreateProviderConfigsWithDiagnostics performs the same credential push
	// as CreateProviderConfigs and additionally returns diagnostic details
	// of the actual HighLevel HTTP response (status, sanitized body, traceId)
	// for logging. The returned diagnostics never contain credentials or the
	// access token. Error semantics are identical to CreateProviderConfigs.
	CreateProviderConfigsWithDiagnostics(ctx context.Context, accessToken, locationID string, creds ProviderCredentials) (*HighLevelCallDiagnostics, error)

	// UpdateProviderCapabilities enables the RVPay Custom Payment Provider
	// capabilities for the supplied HighLevel location. Per the HighLevel v3
	// contract, locationId is sent in the JSON body and RVPay does not
	// support subscription schedules.
	//
	// PUT /payments/custom-provider/capabilities
	// Body: {locationId, supportsSubscriptionSchedules:false}
	UpdateProviderCapabilities(ctx context.Context, accessToken, locationID string) error
}

// HighLevelCallDiagnostics captures diagnostic details of a completed
// HighLevel HTTP response for logging. The Body is always sanitized
// (credential-redacted); diagnostics never contain credentials or the OAuth
// access token.
type HighLevelCallDiagnostics struct {
	// StatusCode is the HTTP status returned by HighLevel.
	StatusCode int
	// Body is the sanitized (credential-redacted) HighLevel response body.
	Body string
	// TraceID is the HighLevel traceId from the response body, if present.
	TraceID string
}

// GhlOrderStatus is the target GoHighLevel payment status for a server-side
// order status synchronization. Only terminal PawaPay results may drive an
// update; pending/processing deposits never land here.
type GhlOrderStatus string

const (
	// GhlOrderStatusCompleted marks the GHL order/payment as captured/paid.
	GhlOrderStatusCompleted GhlOrderStatus = "completed"
	// GhlOrderStatusFailed marks the GHL order/payment as failed/declined.
	GhlOrderStatusFailed GhlOrderStatus = "failed"
)

// ProviderConfig is the provider configuration sent to HighLevel. It is built
// from RVPay configuration and the correct location; it is never hard-coded.
type ProviderConfig struct {
	// Name is the display name of the payment provider.
	Name string
	// Description is the description of the payment provider.
	Description string
	// ImageURL is the publicly accessible image URL of the payment provider.
	ImageURL string
	// LocationID is the HighLevel location ID for the installed account.
	LocationID string
	// QueryURL is the backend query URL supplied to HighLevel.
	QueryURL string
	// PaymentsURL is the frontend checkout URL supplied to HighLevel.
	PaymentsURL string
	// SupportsSubscriptionSchedule indicates whether the provider supports
	// subscription scheduling. RVPay currently supports one-time payments
	// only, so this is always false.
	SupportsSubscriptionSchedule bool
}

// ProviderModeCredentials holds the RVPay processing keys pushed to HighLevel
// for a single mode (live or test). All values come from environment
// configuration; none are hard-coded.
type ProviderModeCredentials struct {
	// APIKey is the RVPay API key HighLevel uses to authenticate requests.
	APIKey string
	// PublishableKey is the RVPay publishable key used on the frontend.
	PublishableKey string
	// LiveMode indicates whether these keys are for the live environment.
	LiveMode bool
}

// ProviderCredentials holds the live and test credentials pushed to HighLevel
// via POST /payments/custom-provider/connect.
type ProviderCredentials struct {
	Live ProviderModeCredentials
	Test ProviderModeCredentials
}

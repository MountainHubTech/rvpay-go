package providers

import "context"

// PaymentProviderReconciler is the narrow boundary the HighLevel webhook
// dispatcher uses to reconcile the REMOTE Custom Payment Provider state for an
// installed GHL location with the local RVPay state.
//
// A local payment_provider_configs row proves nothing about the remote
// provider: GHL removes the custom payment-provider association during
// uninstall while RVPay retains its local rows. The implementation (owned by
// the OAuth service, which already holds the token lifecycle and the
// RegisterProvider sequence) must:
//
//   - resolve the integration by the exact locationId;
//   - load the stored location OAuth token, refreshing it through the existing
//     refresh mechanism when required;
//   - verify the remote provider state and re-run the existing registration
//     sequence only when the remote provider is absent or incomplete;
//   - be idempotent and safe for duplicate INSTALL events;
//   - never delete tokens, integrations, or local provider configuration.
//
// The interface is optional for the dispatcher: when no reconciler is wired,
// INSTALL keeps its previous behavior (create/find the local config row only).
type PaymentProviderReconciler interface {
	// ReconcilePaymentProvider verifies and, when necessary, restores the
	// remote HighLevel Custom Payment Provider for the supplied location.
	// locationID is the exact GHL locationId from the webhook event. It
	// returns a typed error distinguishing unauthorized tokens, absent
	// providers, transient failures, and rejected registrations; it never
	// treats every remote error as "already exists".
	ReconcilePaymentProvider(ctx context.Context, locationID string) error
}

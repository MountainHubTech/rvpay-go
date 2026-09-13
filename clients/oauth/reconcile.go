package oauth

import (
	"context"
	"errors"

	"github.com/MountainHubTech/rvpay-go/clients/db/repo"
	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/MountainHubTech/rvpay-go/clients/providers"
	"github.com/google/uuid"
)

// ReconcilePaymentProvider verifies — and when necessary restores — the
// remote HighLevel Custom Payment Provider for an installed location.
//
// A local payment_provider_configs row proves nothing about the remote
// state: GHL removes the custom payment-provider association during
// uninstall while RVPay retains its local integration/provider rows, and the
// OAuth callback's provider registration is best-effort only. INSTALL
// processing reconciles the remote provider through this operation instead
// of assuming local existence implies remote existence.
//
// Design (smallest safe change; no duplicated registration logic):
//
//  1. The integration is resolved by the exact locationId (deterministic
//     external_account_id mapping, config-locationId fallback only).
//  2. The stored location token is loaded via the existing accessTokenForSync
//     helper and refreshed at most once through the existing refresh path.
//  3. Remote state is verified with the existing FetchProviderConfig; only
//     meaningful metadata counts as present (a 200 with a trace-only body
//     does NOT prove existence).
//  4. When absent or incomplete, the existing RegisterProvider sequence is
//     re-run (idempotent, retry-safe, no duplicate local rows). Local data
//     is preserved unless the remote operation succeeds.
//  5. Logging is fingerprint-only; no token or credential material.
//
// Error classification (never collapses into "already exists"):
//   - ErrOAuthTokenNotFound → deferred (nil): INSTALL may arrive before the
//     OAuth callback; the callback will exchange and register.
//   - ErrReconciliationUnauthorized → 401 after refresh; reauthorization
//     required. Never mislabeled as "already exists".
//   - ErrReconciliationFailed → transient/malformed verification failure;
//     safe to retry via the INSTALL webhook error path.
//   - RegisterProvider's typed errors → registration rejected.
//
// Default-provider status: GHL's documented Custom Payment Provider API does
// not expose an operation to set a location's default payment provider; it
// is selected manually in the HighLevel UI (Payments > Integrations) per
// sub-account. This operation logs default_provider_status accordingly and
// never claims default status from a successful association.
func (s *Service) ReconcilePaymentProvider(ctx context.Context, locationID string) error {
	if s.configRepo == nil {
		return ErrProviderConfigRepoNotConfigured
	}
	if locationID == "" {
		return ErrMissingLocationID
	}

	s.logger.Info().Str("location_id", locationID).Msg("GHL provider reconciliation started")

	// Step 1: resolve the integration by the exact locationId.
	integration, err := s.integrationsRepo.GetByExternalAccountID(ctx, locationID)
	if errors.Is(err, repo.ErrNotFound) {
		config, cfgErr := s.configRepo.GetByLocationID(ctx, locationID)
		if cfgErr == nil {
			integration, err = s.integrationsRepo.GetByID(ctx, config.IntegrationID)
		} else if !errors.Is(cfgErr, repo.ErrNotFound) {
			return translateError(cfgErr)
		}
	}
	if errors.Is(err, repo.ErrNotFound) {
		s.logger.Info().Str("location_id", locationID).
			Msg("GHL provider reconciliation deferred: no integration for location")
		return ErrIntegrationNotFound
	}
	if err != nil {
		return translateError(err)
	}

	return s.reconcileIntegration(ctx, integration, locationID)
}

// reconcileIntegration loads the stored location token and performs the
// remote verification and, when required, re-registration.
func (s *Service) reconcileIntegration(ctx context.Context, integration sqlc.Integration, locationID string) error {
	// Step 2: load the stored location token (refresh-once when expired)
	// through the existing accessTokenForSync helper.
	token, err := s.accessTokenForSync(ctx, integration.ID)
	if errors.Is(err, ErrOAuthTokenNotFound) {
		// INSTALL can arrive before the OAuth callback has exchanged the
		// code. Reconciliation is deferred to the OAuth callback, which
		// exchanges the token and registers the provider. This is not a
		// failure: no token, integration, or config row is deleted.
		s.logger.Info().
			Str("integration_id", integration.ID.String()).
			Str("location_id", locationID).
			Str("token_source", "none").
			Bool("reconciliation_completed", false).
			Msg("GHL provider reconciliation deferred: no OAuth token yet; OAuth callback will register the provider")
		return nil
	}
	if err != nil {
		// Includes refresh failure (ErrTokenRefreshFailed): unauthorized or
		// expired-beyond-refresh tokens must not be retried blindly.
		s.logger.Warn().
			Err(err).
			Str("integration_id", integration.ID.String()).
			Str("location_id", locationID).
			Str("token_source", "refresh").
			Msg("GHL provider reconciliation failed: stored location token unavailable")
		return err
	}

	s.logger.Info().
		Str("integration_id", integration.ID.String()).
		Str("location_id", locationID).
		Str("token_source", token.Source).
		Str("access_token_fingerprint", providers.AccessTokenFingerprint(token.AccessToken)).
		Bool("token_refreshed", token.Refreshed).
		Msg("GHL provider reconciliation resolved location token")

	// Resolve the payment provider client through the existing registry.
	platform, err := s.platformsRepo.GetByID(ctx, integration.PlatformID)
	if errors.Is(err, repo.ErrNotFound) {
		return ErrPlatformNotFound
	}
	if err != nil {
		return translateError(err)
	}
	provider, ok := s.registry.Get(platform.Slug)
	if !ok {
		return ErrProviderNotSupported
	}
	paymentClient := provider.PaymentProvider()
	if paymentClient == nil {
		return ErrPaymentProviderNotSupported
	}

	// Step 3: verify the remote provider state. HTTP 200 alone is not proof:
	// HighLevel can answer 200 with a trace-only body while the provider
	// configuration has not been materialized.
	remotePresent := false
	fetched, verifyErr := paymentClient.FetchProviderConfig(ctx, token.AccessToken, locationID)
	switch {
	case verifyErr == nil && fetched.Name != "" && fetched.QueryURL != "" && fetched.PaymentsURL != "":
		remotePresent = true
		s.logger.Info().
			Str("integration_id", integration.ID.String()).
			Str("location_id", locationID).
			Str("remote_verification_result", "provider_present_and_complete").
			Str("default_provider_status", "unknown").
			Msg("GHL provider reconciliation verified remote provider; default provider is selected manually in the HighLevel UI")
	case errors.Is(verifyErr, providers.ErrUnauthorized):
		// Never mislabel an authorization failure as "already exists".
		s.logger.Warn().
			Err(verifyErr).
			Str("integration_id", integration.ID.String()).
			Str("location_id", locationID).
			Str("remote_verification_result", "unauthorized").
			Str("default_provider_status", "unknown").
			Msg("GHL provider reconciliation unauthorized; reauthorization of the location is required")
		return ErrReconciliationUnauthorized
	case verifyErr == nil:
		// 200 with an empty/trace-only body: base configuration not
		// materialized — treat as absent/incomplete.
		s.logger.Info().
			Str("integration_id", integration.ID.String()).
			Str("location_id", locationID).
			Str("remote_verification_result", "provider_absent_or_incomplete").
			Str("default_provider_status", "unknown").
			Msg("GHL provider reconciliation found incomplete remote provider; re-running registration")
	default:
		var apiErr *providers.HighLevelAPIError
		classification := "transient_or_unexpected"
		if errors.As(verifyErr, &apiErr) {
			if errors.Is(verifyErr, providers.ErrBadRequest) || errors.Is(verifyErr, providers.ErrUnprocessableEntity) {
				classification = "provider_absent_or_incomplete"
			}
		}
		if classification != "provider_absent_or_incomplete" {
			// Do not re-register on an unknown/transient verification
			// failure: surface it so the INSTALL webhook is retried instead
			// of guessing the remote state.
			s.logger.Warn().
				Err(verifyErr).
				Str("integration_id", integration.ID.String()).
				Str("location_id", locationID).
				Str("remote_verification_result", classification).
				Str("default_provider_status", "unknown").
				Msg("GHL provider reconciliation could not verify remote provider; retry required")
			return ErrReconciliationFailed
		}
		s.logger.Info().
			Err(verifyErr).
			Str("integration_id", integration.ID.String()).
			Str("location_id", locationID).
			Str("remote_verification_result", classification).
			Str("default_provider_status", "unknown").
			Msg("GHL provider reconciliation found no remote provider; re-running registration")
	}

	return s.reregisterProvider(ctx, integration.ID, locationID, token, remotePresent)
}

// reregisterProvider decides between preserving the local row and re-running
// the existing RegisterProvider sequence, and logs the result.
func (s *Service) reregisterProvider(ctx context.Context, integrationID uuid.UUID, locationID string, token syncAccessToken, remotePresent bool) error {
	// Local row existence is checked only to report state; it never short-
	// circuits re-registration when the remote provider is absent.
	_, localErr := s.configRepo.GetByIntegrationID(ctx, integrationID)
	haveLocal := localErr == nil
	if localErr != nil && !errors.Is(localErr, repo.ErrNotFound) {
		return translateError(localErr)
	}

	if remotePresent && haveLocal {
		s.logger.Info().
			Str("integration_id", integrationID.String()).
			Str("location_id", locationID).
			Bool("local_provider_row_existed", true).
			Bool("remote_provider_present", true).
			Bool("registration_attempted", false).
			Bool("reconciliation_completed", true).
			Str("default_provider_status", "unknown").
			Msg("GHL provider reconciliation completed: remote provider present, local configuration preserved")
		return nil
	}

	s.logger.Info().
		Str("integration_id", integrationID.String()).
		Str("location_id", locationID).
		Bool("local_provider_row_existed", haveLocal).
		Bool("remote_provider_present", remotePresent).
		Bool("registration_attempted", true).
		Msg("GHL provider reconciliation re-running provider registration")

	// Re-run the EXISTING registration sequence (idempotent, retry-safe,
	// reuses/updates the existing local row — no duplicates).
	if regErr := s.RegisterProvider(ctx, integrationID, locationID, token.AccessToken); regErr != nil {
		// A 401 anywhere in the registration sequence surfaces as
		// unauthorized, not as "provider already exists".
		var apiErr *providers.HighLevelAPIError
		if errors.As(regErr, &apiErr) && errors.Is(regErr, providers.ErrUnauthorized) {
			s.logger.Warn().
				Err(regErr).
				Str("integration_id", integrationID.String()).
				Str("location_id", locationID).
				Str("registration_result", "unauthorized").
				Bool("reconciliation_completed", false).
				Msg("GHL provider reconciliation registration unauthorized; reauthorization required")
			return ErrReconciliationUnauthorized
		}
		s.logger.Warn().
			Err(regErr).
			Str("integration_id", integrationID.String()).
			Str("location_id", locationID).
			Str("registration_result", "failed").
			Bool("reconciliation_completed", false).
			Msg("GHL provider reconciliation registration failed; later reconciliation required")
		return regErr
	}

	s.logger.Info().
		Str("integration_id", integrationID.String()).
		Str("location_id", locationID).
		Str("registration_result", "succeeded").
		Bool("reconciliation_completed", true).
		Str("default_provider_status", "manual_ui_step").
		Msg("GHL provider reconciliation completed; default provider must be selected in the HighLevel UI")

	return nil
}

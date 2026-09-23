package oauth

import (
	"context"
	"errors"
	"strings"

	"github.com/MountainHubTech/rvpay-go/clients/db/repo"
	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
)

// This file implements the account (client) display-name enrichment: the
// authoritative HighLevel location/sub-account name is fetched through the
// EXISTING HighLevel provider client and persisted in clients.display_name.
//
// It deliberately reuses every existing building block instead of introducing a
// parallel auth or lifecycle:
//
//	integrations.external_account_id (GHL locationId)
//	  ↓   resolveIntegrationForLocation (existing deterministic mapping)
//	stored location access token
//	  ↓   accessTokenForSync (existing refresh-once path)
//	GET /locations/{locationId}  (existing shared v3 client)
//	  ↓   location.name  (never the locationId)
//	clients.display_name
//	  ↓   UpdateDisplayName (guarded: only fills a missing name)
//	Admin Dashboard sub-account name
//
// clients.client_name ("highlevel-<locationId>") is NOT changed: it is the
// correlation identifier the dashboard sends as the Transactions sub-account
// filter.
//
// Requirements: the location token must carry the locations.readonly scope. A
// token granted before that scope was requested fails with
// providers.ErrUnauthorized / providers.ErrForbidden, which means the location
// must be reauthorized (reinstalled); the existing display name is preserved in
// that case and the reason is reported to the caller (and by the backfill CLI).

// SyncClientDisplayName fetches the authoritative HighLevel location name for a
// locationId and persists it as the owning client's display name.
//
// The operation is idempotent and safe to re-run: a client that already has a
// display name is left untouched (the guarded update matches 0 rows, which is
// reported as success, not failure). Nothing is fabricated — an empty location
// name yields ErrLocationNameUnavailable and no write. Tokens, credentials and
// raw HighLevel response bodies are never logged; errors carry no credentials.
func (s *Service) SyncClientDisplayName(ctx context.Context, locationID string) error {
	locationID = strings.TrimSpace(locationID)
	if locationID == "" {
		return ErrMissingLocationID
	}

	integration, err := s.resolveIntegrationForLocation(ctx, locationID)
	if err != nil {
		return err
	}

	return s.syncClientDisplayName(ctx, integration, locationID)
}

// resolveIntegrationForLocation resolves the installed integration for a GHL
// location using the same deterministic resolution order as provider
// reconciliation:
//
//  1. integrations.external_account_id = locationId (authoritative mapping).
//  2. payment_provider_configs.location_id = locationId.
func (s *Service) resolveIntegrationForLocation(ctx context.Context, locationID string) (sqlc.Integration, error) {
	integration, err := s.integrationsRepo.GetByExternalAccountID(ctx, locationID)
	if errors.Is(err, repo.ErrNotFound) && s.configRepo != nil {
		config, cfgErr := s.configRepo.GetByLocationID(ctx, locationID)
		if cfgErr == nil {
			integration, err = s.integrationsRepo.GetByID(ctx, config.IntegrationID)
		} else if !errors.Is(cfgErr, repo.ErrNotFound) {
			return sqlc.Integration{}, translateError(cfgErr)
		}
	}
	if errors.Is(err, repo.ErrNotFound) {
		s.logger.Info().
			Str("location_id", locationID).
			Str("operation", "SyncClientDisplayName").
			Bool("display_name_updated", false).
			Msg("client display name enrichment deferred: no integration for location")
		return sqlc.Integration{}, ErrIntegrationNotFound
	}
	if err != nil {
		return sqlc.Integration{}, translateError(err)
	}
	return integration, nil
}

// syncClientDisplayName performs the enrichment for an already-resolved
// integration. It is used by the OAuth callback and reconciliation paths, which
// already hold the integration and must not re-resolve it.
func (s *Service) syncClientDisplayName(ctx context.Context, integration sqlc.Integration, locationID string) error {
	platform, err := s.platformsRepo.GetByID(ctx, integration.PlatformID)
	if errors.Is(err, repo.ErrNotFound) {
		return ErrPlatformNotFound
	}
	if err != nil {
		return translateError(err)
	}
	if !platform.Enabled {
		return ErrPlatformDisabled
	}

	provider, ok := s.registry.Get(platform.Slug)
	if !ok {
		return ErrProviderNotSupported
	}

	paymentClient := provider.PaymentProvider()
	if paymentClient == nil {
		return ErrPaymentProviderNotSupported
	}

	// Existing token handling: the stored location token is used as-is and
	// refreshed at most once through the existing refresh path when expired.
	token, err := s.accessTokenForSync(ctx, integration.ID)
	if err != nil {
		return err
	}

	location, err := paymentClient.FetchLocation(ctx, token.AccessToken, locationID)
	if err != nil {
		// The existing display name is preserved: a failed fetch (401/403
		// missing scope, 404, 429, 5xx, timeout) is reported, never written.
		s.logger.Warn().
			Err(err).
			Str("integration_id", integration.ID.String()).
			Str("client_id", integration.ClientID.String()).
			Str("location_id", locationID).
			Str("token_source", token.Source).
			Bool("display_name_updated", false).
			Msg("HighLevel location name fetch failed; existing client display name preserved")
		return err
	}

	name := strings.TrimSpace(location.Name)
	if name == "" {
		s.logger.Warn().
			Str("integration_id", integration.ID.String()).
			Str("client_id", integration.ClientID.String()).
			Str("location_id", locationID).
			Bool("display_name_updated", false).
			Msg("HighLevel returned no location name; existing client display name preserved")
		return ErrLocationNameUnavailable
	}

	updated, err := s.clientsRepo.UpdateDisplayName(ctx, integration.ClientID, name)
	if errors.Is(err, repo.ErrNotFound) {
		// Either the client no longer exists or it already carries a display
		// name (the guarded UPDATE matched no row). Both are safe no-ops.
		s.logger.Info().
			Str("integration_id", integration.ID.String()).
			Str("client_id", integration.ClientID.String()).
			Str("location_id", locationID).
			Bool("display_name_updated", false).
			Msg("client display name already resolved; nothing to update")
		return nil
	}
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("integration_id", integration.ID.String()).
			Str("client_id", integration.ClientID.String()).
			Str("location_id", locationID).
			Bool("display_name_updated", false).
			Msg("could not persist client display name")
		return translateError(err)
	}

	s.logger.Info().
		Str("integration_id", integration.ID.String()).
		Str("client_id", updated.ID.String()).
		Str("location_id", locationID).
		Str("token_source", token.Source).
		Bool("display_name_updated", true).
		Msg("HighLevel location name persisted as client display name")

	return nil
}

// enrichClientDisplayName runs the display-name enrichment without failing the
// caller. The OAuth installation / provider reconciliation result is never
// changed by a name-enrichment problem (it is a display concern), and the client
// keeps its existing name until the operation succeeds on a later install,
// reconciliation, or explicit backfill run.
func (s *Service) enrichClientDisplayName(ctx context.Context, integration sqlc.Integration, locationID string) {
	if err := s.syncClientDisplayName(ctx, integration, locationID); err != nil {
		s.logger.Warn().
			Err(err).
			Str("integration_id", integration.ID.String()).
			Str("location_id", locationID).
			Str("operation", "SyncClientDisplayName").
			Msg("client display name enrichment deferred; installation/reconciliation result unchanged")
	}
}

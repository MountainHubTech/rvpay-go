package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/I-Frostbyte/rvpay-go/clients/db/repo"
	"github.com/I-Frostbyte/rvpay-go/clients/db/sqlc"
	"github.com/I-Frostbyte/rvpay-go/clients/providers"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ProviderConfigSettings holds the configuration used to build the HighLevel
// Custom Payment Provider configuration. All values come from environment
// configuration; none are hard-coded.
type ProviderConfigSettings struct {
	// Name is the display name of the payment provider.
	Name string
	// Description is the description of the payment provider.
	Description string
	// ImageURL is the publicly accessible image URL of the payment provider.
	ImageURL string
	// PaymentsURL is the frontend checkout URL supplied to HighLevel.
	PaymentsURL string
	// QueryURL is the backend query URL supplied to HighLevel.
	QueryURL string
	// LiveAPIKey is the RVPay live API key pushed to HighLevel as the
	// provider config's live apiKey (HIGHLEVEL_LIVE_API_KEY).
	LiveAPIKey string
	// LivePublishableKey is the RVPay live publishable key pushed to
	// HighLevel (HIGHLEVEL_LIVE_PUBLISHABLE_KEY).
	LivePublishableKey string
	// TestAPIKey is the RVPay test API key pushed to HighLevel as the
	// provider config's test apiKey (HIGHLEVEL_TEST_API_KEY).
	TestAPIKey string
	// TestPublishableKey is the RVPay test publishable key pushed to
	// HighLevel (HIGHLEVEL_TEST_PUBLISHABLE_KEY).
	TestPublishableKey string
}

// Service manages OAuth flows for provider integrations.
type Service struct {
	integrationsRepo repo.IntegrationRepo
	oauthRepo        repo.OAuthTokenRepo
	clientsRepo      repo.ClientRepo
	platformsRepo    repo.PlatformRepo
	oauthStateRepo   repo.OAuthStateRepo
	configRepo       repo.PaymentProviderConfigRepo
	registry         providers.ProviderRegistry
	redirectURI      string
	stateTTL         time.Duration
	providerConfig   ProviderConfigSettings
	logger           zerolog.Logger
}

// NewService creates a new OAuth service. redirectURI is the configured
// callback URL used for the OAuth authorization and token exchange; it must
// come from configuration (HIGHLEVEL_REDIRECT_URI), never be hard-coded.
// oauthStateRepo persists OAuth state so the callback can securely recover
// the client/platform context and resist CSRF/replay attacks.
//
// configRepo persists the HighLevel Custom Payment Provider configuration
// created during the registration lifecycle. It may be nil if the provider
// does not support Custom Payment Provider operations.
//
// providerConfig holds the configuration used to build the provider
// configuration sent to HighLevel. It is only used when the provider supports
// the payment provider capability.
func NewService(
	integrationsRepo repo.IntegrationRepo,
	oauthRepo repo.OAuthTokenRepo,
	clientsRepo repo.ClientRepo,
	platformsRepo repo.PlatformRepo,
	oauthStateRepo repo.OAuthStateRepo,
	configRepo repo.PaymentProviderConfigRepo,
	registry providers.ProviderRegistry,
	redirectURI string,
	providerConfig ProviderConfigSettings,
	logger zerolog.Logger,
) *Service {
	return &Service{
		integrationsRepo: integrationsRepo,
		oauthRepo:        oauthRepo,
		clientsRepo:      clientsRepo,
		platformsRepo:    platformsRepo,
		oauthStateRepo:   oauthStateRepo,
		configRepo:       configRepo,
		registry:         registry,
		redirectURI:      redirectURI,
		stateTTL:         10 * time.Minute,
		providerConfig:   providerConfig,
		logger:           logger,
	}
}

// AuthorizationURL generates the OAuth authorization URL for a client and platform.
func (s *Service) AuthorizationURL(ctx context.Context, clientID, platformID uuid.UUID, state string) (string, error) {
	platform, err := s.platformsRepo.GetByID(ctx, platformID)
	if err == repo.ErrNotFound {
		return "", ErrPlatformNotFound
	}
	if err != nil {
		return "", translateError(err)
	}

	if !platform.Enabled {
		return "", ErrPlatformDisabled
	}

	provider, ok := s.registry.Get(platform.Slug)
	if !ok {
		return "", ErrProviderNotSupported
	}

	authURL, err := provider.OAuthProvider().GenerateAuthorizationURL(ctx, state, s.redirectURI)
	if err != nil {
		return "", err
	}

	s.logger.Info().Str("client_id", clientID.String()).Str("platform_id", platformID.String()).Str("provider", provider.ID()).Msg("OAuth authorization URL generated")

	return authURL, nil
}

// BeginAuthorization starts an OAuth authorization flow for a client and
// platform. It generates a cryptographically random state, persists it with
// the client/platform context and an expiry, and returns the provider
// authorization URL. The returned state must be validated by HandleCallback
// when the provider redirects the user back.
func (s *Service) BeginAuthorization(ctx context.Context, clientID, platformID uuid.UUID) (authURL, state string, err error) {
	platform, err := s.platformsRepo.GetByID(ctx, platformID)
	if err == repo.ErrNotFound {
		return "", "", ErrPlatformNotFound
	}
	if err != nil {
		return "", "", translateError(err)
	}

	if !platform.Enabled {
		return "", "", ErrPlatformDisabled
	}

	client, err := s.clientsRepo.GetByID(ctx, clientID)
	if err == repo.ErrNotFound {
		return "", "", ErrClientNotFound
	}
	if err != nil {
		return "", "", translateError(err)
	}

	if client.Status != sqlc.ClientStatusACTIVE {
		return "", "", ErrClientInactive
	}

	provider, ok := s.registry.Get(platform.Slug)
	if !ok {
		return "", "", ErrProviderNotSupported
	}

	state, err = providers.GenerateState()
	if err != nil {
		return "", "", err
	}

	_, err = s.oauthStateRepo.Create(ctx, state, clientID, platformID, time.Now().Add(s.stateTTL))
	if err != nil {
		return "", "", translateError(err)
	}

	authURL, err = provider.OAuthProvider().GenerateAuthorizationURL(ctx, state, s.redirectURI)
	if err != nil {
		return "", "", err
	}

	s.logger.Info().Str("client_id", clientID.String()).Str("platform_id", platformID.String()).Str("provider", provider.ID()).Msg("OAuth authorization flow started")

	return authURL, state, nil
}

// HandleCallback processes an OAuth callback from a provider.
//
// The `code` is always required. The `state` is optional:
//
//   - When `state` is present, the existing state-based flow is preserved: the
//     state is validated (must exist, be unexpired, and be unconsumed),
//     atomically consumed to prevent replay, and used to recover the
//     client/platform context.
//   - When `state` is absent (HighLevel Marketplace OAuth does not return a
//     state), the authorization code is exchanged first to obtain the GHL
//     locationId. The existing HighLevel platform (slug "highlevel") is
//     resolved, and the tenant client plus the client's integration to the
//     platform are created in the database during this one-time installation
//     (idempotent — reused if already present). The integration is mapped to
//     the locationId via external_account_id and the callback continues.
func (s *Service) HandleCallback(ctx context.Context, code, state string) (*CallbackResult, error) {
	s.logger.Info().Msg("\n HandleCallback method initiated... \n")

	if code == "" {
		return nil, ErrMissingCode
	}

	if state != "" {
		// Atomically consume the state. ConsumeOAuthState only succeeds when the
		// state exists, is not already consumed, and has not expired. This both
		// validates the state and prevents replay attacks in a single operation.
		s.logger.Info().Msg("\n State exists and is being consumed... \n")
		record, err := s.oauthStateRepo.Consume(ctx, state)
		if err == repo.ErrNotFound {
			// Distinguish expired/consumed from unknown for clearer errors.
			existing, getErr := s.oauthStateRepo.GetByState(ctx, state)
			if getErr == nil {
				if existing.ConsumedAt.Valid {
					return nil, ErrStateConsumed
				}
				return nil, ErrStateExpired
			}
			return nil, ErrInvalidState
		}
		if err != nil {
			return nil, translateError(err)
		}

		return s.ProcessCallback(ctx, record.ClientID, record.PlatformID, code, state)
	}

	// No state: resolve the client/platform context from the GHL locationId.
	// Exchange the authorization code first to obtain the locationId, then
	// resolve locationId -> integration via the deterministic mapping.
	s.logger.Info().Msg("\n State doesn't exist, resolving client/platform context... \n ")
	if s.configRepo == nil {
		return nil, ErrProviderConfigRepoNotConfigured
	}

	provider, ok := s.registry.Get("highlevel")
	if !ok {
		return nil, ErrProviderNotSupported
	}

	tokenResp, err := provider.OAuthProvider().ExchangeCode(ctx, code, s.redirectURI)
	if err != nil {
		s.logger.Error().Err(err).Msg("OAuth token exchange failed during stateless callback")
		return nil, ErrTokenExchangeFailed
	}
	if tokenResp.LocationID == "" {
		return nil, ErrMissingLocationID
	}

	s.logger.Info().Msgf("\n Location ID: %v \n", tokenResp.LocationID)

	// Only the HighLevel platform is expected to already exist. Resolve it by
	// slug (never create or modify platform records). The installation creates
	// the client and the client's integration to that platform in the database
	// during this OAuth callback; neither is assumed to exist beforehand.
	platform, err := s.platformsRepo.GetBySlug(ctx, "highlevel")
	if err == repo.ErrNotFound {
		return nil, ErrPlatformNotFound
	}
	if err != nil {
		return nil, translateError(err)
	}

	// Derive a deterministic tenant client name from the GHL sub-account so a
	// repeat callback reuses the same client. Create it ACTIVE when missing so
	// the shared callback processing can complete the installation.
	clientName := "highlevel-" + tokenResp.LocationID
	client, err := s.clientsRepo.GetByName(ctx, clientName)
	if err == repo.ErrNotFound {
		client, err = s.clientsRepo.Create(ctx, clientName, sqlc.ClientStatusACTIVE)
	}
	if err == repo.ErrDuplicate {
		// A concurrent install created the client; reuse it.
		client, err = s.clientsRepo.GetByName(ctx, clientName)
	}
	if err != nil {
		return nil, translateError(err)
	}

	// Create the client's integration to the platform during installation,
	// mapped to the GHL locationId. Reuse it if a prior install already
	// created it. Status CREATED lets the shared callback activate the
	// integration and persist the OAuth token.
	integration, err := s.integrationsRepo.GetByClientAndPlatform(ctx, client.ID, platform.ID)
	if err == repo.ErrNotFound {
		integration, err = s.integrationsRepo.Create(ctx, client.ID, platform.ID, tokenResp.LocationID, sqlc.IntegrationStatusCREATED)
		if err == repo.ErrDuplicate {
			integration, err = s.integrationsRepo.GetByClientAndPlatform(ctx, client.ID, platform.ID)
		}
	}
	if err != nil {
		return nil, translateError(err)
	}

	// Continue with the already-exchanged token response. The authorization
	// code was exchanged exactly once above; it must not be exchanged again by
	// downstream processing.
	return s.processCallbackWithToken(ctx, integration.ClientID, integration.PlatformID, provider, tokenResp)
}

// CallbackResult represents the result of an OAuth callback.
type CallbackResult struct {
	IntegrationID  uuid.UUID
	ClientID       uuid.UUID
	PlatformID     uuid.UUID
	AccessToken    string
	RefreshToken   string
	ExpiresAt      time.Time
	Scope          string
	ProviderUserID string
	// ProviderRegistered indicates whether the HighLevel Custom Payment
	// Provider registration completed successfully during the OAuth callback.
	// It is false when the provider does not support payment provider
	// operations or when registration failed (in which case the integration
	// remains installed and RegisterProvider can be retried).
	ProviderRegistered bool
	// ProviderRegistrationError is set when provider registration failed but
	// the OAuth installation itself succeeded. The integration remains
	// installed; the error is informational so the caller can decide whether
	// to surface it or retry registration.
	ProviderRegistrationError error
}

// ProcessCallback processes the OAuth callback and creates the integration.
// After the integration and OAuth token are persisted, it triggers the
// HighLevel Custom Payment Provider registration lifecycle (provider
// association and configuration) when the provider supports it.
//
// If an integration already exists for the client/platform with status
// CREATED (a pre-provisioned integration awaiting OAuth completion), it is
// reused and activated rather than returning ErrIntegrationAlreadyExists.
// This supports the HighLevel Marketplace first-install flow where the
// integration is pre-created before the OAuth callback arrives.
func (s *Service) ProcessCallback(ctx context.Context, clientID, platformID uuid.UUID, code, state string) (*CallbackResult, error) {
	platform, err := s.platformsRepo.GetByID(ctx, platformID)
	if err == repo.ErrNotFound {
		return nil, ErrPlatformNotFound
	}
	if err != nil {
		return nil, translateError(err)
	}

	if !platform.Enabled {
		return nil, ErrPlatformDisabled
	}

	provider, ok := s.registry.Get(platform.Slug)
	if !ok {
		return nil, ErrProviderNotSupported
	}

	client, err := s.clientsRepo.GetByID(ctx, clientID)
	if err == repo.ErrNotFound {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, translateError(err)
	}

	if client.Status != sqlc.ClientStatusACTIVE {
		return nil, ErrClientInactive
	}

	// Exchange the authorization code exactly once. Downstream processing
	// consumes the already-exchanged token response and never re-exchanges the
	// raw authorization code, so a flow that already exchanged the code (e.g.
	// the stateless HighLevel Marketplace callback) cannot cause a second
	// exchange.
	tokenResp, err := provider.OAuthProvider().ExchangeCode(ctx, code, s.redirectURI)
	if err != nil {
		s.logger.Error().Err(err).Str("client_id", clientID.String()).Str("platform_id", platformID.String()).Msg("OAuth token exchange failed")
		return nil, ErrTokenExchangeFailed
	}

	return s.processCallbackWithToken(ctx, clientID, platformID, provider, tokenResp)
}

// processCallbackWithToken continues an OAuth callback after the authorization
// code has been exchanged once for a token response. Both the state-based flow
// and the stateless HighLevel Marketplace flow converge here with the token
// response from their single exchange, so the authorization code is never
// exchanged twice.
//
// It re-validates the active integration's client, resolves the provider user
// info, reuses (CREATED) or creates the integration, persists the OAuth token,
// and triggers the HighLevel Custom Payment Provider registration lifecycle.
func (s *Service) processCallbackWithToken(ctx context.Context, clientID, platformID uuid.UUID, provider providers.Provider, tokenResp *providers.TokenResponse) (*CallbackResult, error) {
	s.logger.Info().Msg("ProcessCallbackWithToken Initiated...")
	// Re-validate the client for the stateless Marketplace flow, where the tenant
	// may have been provisioned by the INSTALL webhook earlier. The state-based
	// flow already validated the client before the exchange, so this is a
	// harmless redundant check that keeps both paths converging here correct.
	client, err := s.clientsRepo.GetByID(ctx, clientID)
	if err == repo.ErrNotFound {
		return nil, ErrClientNotFound
	}
	if err != nil {
		return nil, translateError(err)
	}

	if client.Status != sqlc.ClientStatusACTIVE {
		return nil, ErrClientInactive
	}

	s.logger.Info().Msgf("\n Client that was created or already existed already: %v \n", client)

	// providerUserID, err := provider.OAuthProvider().GetUserInfo(ctx, tokenResp.AccessToken)
	// if err != nil {
	// 	s.logger.Error().Err(err).Str("client_id", clientID.String()).Str("platform_id", platformID.String()).Msg("OAuth user info retrieval failed")
	// 	return nil, ErrUserInfoFailed
	// }

	// Determine the integration to use. If an integration already exists for
	// this client/platform:
	//   - CREATED: reuse it (pre-provisioned integration awaiting OAuth
	//     completion). Activate it and continue the token/registration flow.
	//   - otherwise: return ErrIntegrationAlreadyExists (genuine conflict).
	var integration sqlc.Integration
	existing, err := s.integrationsRepo.GetByClientAndPlatform(ctx, clientID, platformID)
	if err == nil {
		if existing.Status != sqlc.IntegrationStatusCREATED {
			// // Temporarily create the provider association here.
			// // Get rid of this if block after local testing is done.
			// if provider.HasCapability(providers.CapabilityPaymentProvider) {
			// 	regErr := s.RegisterProvider(ctx, existing.ID, tokenResp.LocationID, tokenResp.AccessToken)
			// 	if regErr != nil {
			// 		s.logger.Warn().Err(regErr).Str("integration_id", existing.ID.String()).Str("location_id", tokenResp.LocationID).Msg("HighLevel provider registration failed; integration remains installed")
			// 		// result.ProviderRegistrationError = regErr
			// 	} else {
			// 		// result.ProviderRegistered = true
			// 		s.logger.Info().Msg("Provider successfully registered")
			// 	}
			// }
			return nil, ErrIntegrationAlreadyExists
		}
		// Reuse the pre-provisioned CREATED integration. Activate it. The
		// external_account_id is set to the GHL locationId when the provider
		// registration persists the payment_provider_configs record; the
		// locationId is the deterministic GHL sub-account identifier.
		integration, err = s.integrationsRepo.UpdateStatus(ctx, existing.ID, sqlc.IntegrationStatusACTIVE)
		if err != nil {
			return nil, translateError(err)
		}
		s.logger.Info().Str("integration_id", integration.ID.String()).Str("client_id", clientID.String()).Str("platform_id", platformID.String()).Msg("reused pre-provisioned CREATED integration")
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, translateError(err)
	} else {
		// No integration exists for this client/platform yet. Create it now,
		// mapped to the HighLevel location id, and activate it so the OAuth
		// token persistence and provider registration have a complete
		// integration to target. This mirrors the idempotent provisioning done
		// by the stateless Marketplace callback.
		integration, err = s.integrationsRepo.Create(ctx, clientID, platformID, tokenResp.LocationID, sqlc.IntegrationStatusCREATED)
		if err == repo.ErrDuplicate {
			// A concurrent callback already created the integration; reuse it.
			integration, err = s.integrationsRepo.GetByClientAndPlatform(ctx, clientID, platformID)
		}
		if err != nil {
			return nil, translateError(err)
		}
		integration, err = s.integrationsRepo.UpdateStatus(ctx, integration.ID, sqlc.IntegrationStatusACTIVE)
		if err != nil {
			return nil, translateError(err)
		}
	}
	// else {
	// 	integration, err = s.integrationsRepo.Create(ctx, clientID, platformID, providerUserID, sqlc.IntegrationStatusACTIVE)
	// 	if err != nil {
	// 		return nil, translateError(err)
	// 	}
	// }

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	_, err = s.oauthRepo.Create(ctx, integration.ID, tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt, tokenResp.Scope, tokenResp.TokenType)
	if err != nil {
		s.logger.Error().Err(err).Str("integration_id", integration.ID.String()).Msg("OAuth token persistence failed")
		return nil, translateError(err)
	}

	result := &CallbackResult{
		IntegrationID: integration.ID,
		ClientID:      clientID,
		PlatformID:    platformID,
		AccessToken:   tokenResp.AccessToken,
		RefreshToken:  tokenResp.RefreshToken,
		ExpiresAt:     expiresAt,
		Scope:         tokenResp.Scope,
		// ProviderUserID: providerUserID,
	}

	// Trigger the HighLevel Custom Payment Provider registration lifecycle.
	// The OAuth installation is already persisted; registration is a
	// best-effort follow-up that must not roll back the installation. The
	// location ID used for registration is the actual HighLevel locationId
	// from the OAuth token response, not the HighLevel user ID.
	if provider.HasCapability(providers.CapabilityPaymentProvider) {
		regErr := s.RegisterProvider(ctx, integration.ID, tokenResp.LocationID, tokenResp.AccessToken)
		if regErr != nil {
			s.logger.Warn().Err(regErr).Str("integration_id", integration.ID.String()).Str("location_id", tokenResp.LocationID).Msg("HighLevel provider registration failed; integration remains installed")
			result.ProviderRegistrationError = regErr
		} else {
			result.ProviderRegistered = true
		}
	}

	s.logger.Info().Str("integration_id", integration.ID.String()).Str("client_id", clientID.String()).Str("platform_id", platformID.String()).Bool("provider_registered", result.ProviderRegistered).Msg("OAuth callback processed successfully")

	// .Str("provider_user_id", providerUserID)

	return result, nil
}

// baseConfigVerifyAttempts and baseConfigVerifyDelay bound the
// eventual-consistency verification retry around the GET
// /payments/custom-provider/connect only. They are package-level variables so
// tests can shorten the retry without changing behavior.
var (
	baseConfigVerifyAttempts = 5
	baseConfigVerifyDelay    = 500 * time.Millisecond
)

// RegisterProvider registers RVPay as the HighLevel Custom Payment Provider
// for an installed location and persists the local provider configuration.
// It:
//
//  1. Registers the provider association
//     (POST /payments/custom-provider/provider?locationId=<id>) with the
//     provider metadata in the body. This is what makes RVPay appear and work
//     on HighLevel's Payments > Integrations page.
//  2. Confirms the base configuration exists
//     (GET /payments/custom-provider/connect?locationId=<id>) with a small
//     bounded verification retry for eventual consistency. The credential
//     POST is never sent until the base configuration has been confirmed;
//     otherwise HighLevel answers 422 "Base config ... not created yet".
//  3. Pushes the live/test processing keys
//     (POST /payments/custom-provider/connect?locationId=<id>) only after
//     the base configuration is confirmed to exist.
//  4. Persists the local provider configuration, reusing an existing API key
//     when a valid local config already exists. Remote metadata fetched in
//     step 2 is preferred over configured defaults.
//
// The operation is idempotent: if the provider is already associated, the
// existing configuration is fetched and persisted instead of creating a
// duplicate. It does not classify every 400/422 as "already exists"; an
// association failure is confirmed via the fetch before being treated as
// idempotent. Registration failures return a typed error; the integration
// remains installed and the operation can be retried safely.
func (s *Service) RegisterProvider(ctx context.Context, integrationID uuid.UUID, locationID, accessToken string) error {
	s.logger.Info().Msg("RegisterProvider initiated...")

	s.logger.Info().Str("location_id", locationID).Msg("location id for client")

	s.logger.Info().Str("access_token", accessToken).Msg("access token for client")

	s.logger.Info().Msg("Checking provider configuration repository...")
	if s.configRepo == nil {
		return ErrProviderConfigRepoNotConfigured
	}
	if locationID == "" {
		return ErrMissingLocationID
	}
	if accessToken == "" {
		return ErrMissingAccessToken
	}

	integration, err := s.integrationsRepo.GetByID(ctx, integrationID)
	if err == repo.ErrNotFound {
		return ErrIntegrationNotFound
	}
	if err != nil {
		return translateError(err)
	}

	platform, err := s.platformsRepo.GetByID(ctx, integration.PlatformID)
	if err == repo.ErrNotFound {
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

	// Build the provider metadata sent to HighLevel from RVPay configuration.
	// Nothing is hard-coded; all values come from environment configuration.
	metadata := providers.ProviderConfig{
		Name:                         s.providerConfig.Name,
		Description:                  s.providerConfig.Description,
		ImageURL:                     s.providerConfig.ImageURL,
		LocationID:                   locationID,
		QueryURL:                     s.providerConfig.QueryURL,
		PaymentsURL:                  s.providerConfig.PaymentsURL,
		SupportsSubscriptionSchedule: false, // RVPay supports one-time payments only.
	}

	// Step 1: Register the provider association. This is the correct v3 step
	// for metadata registration: locationId is a required query parameter and
	// the metadata is sent in the body. We do not treat every 400/422 as
	// "already exists"; we confirm via the fetch in Step 2 and only treat it as
	// idempotent when a real provider configuration is returned.
	err = paymentClient.CreateProviderAssociation(ctx, accessToken, metadata)
	if err != nil {
		if !errors.Is(err, providers.ErrBadRequest) && !errors.Is(err, providers.ErrUnprocessableEntity) {
			s.logger.Error().
				Err(err).
				Str("integration_id", integrationID.String()).
				Str("location_id", locationID).
				Msg("HighLevel provider association failed")

			return ErrProviderAssociationFailed
		}
		s.logger.Info().Str("integration_id", integrationID.String()).Str("location_id", locationID).Msg("provider association may already exist; confirming via fetch")
	}

	// Step 2: Confirm the base configuration exists before pushing any
	// credentials. Per the HighLevel v3 contract, the credential POST to
	// /payments/custom-provider/connect fails with HTTP 422 ("Base config for
	// integration is not created yet") if the base configuration has not been
	// created by the provider association yet. HighLevel may create it
	// asynchronously, so the GET below is retried a small, bounded number of
	// times. Credentials are NEVER sent until the GET confirms the base
	// configuration exists.
	baseConfigConfirmed := false
	for attempt := 1; attempt <= baseConfigVerifyAttempts; attempt++ {
		existing, fetchErr := paymentClient.FetchProviderConfig(ctx, accessToken, locationID)
		if fetchErr == nil {
			// HTTP success alone is NOT proof that the base configuration
			// exists: HighLevel can answer 200 with a trace-only body
			// ({"traceId":"..."}) while the base provider configuration has
			// not been materialized yet. The base config is only considered
			// confirmed when the GET returns meaningful provider metadata.
			if existing.Name != "" && existing.QueryURL != "" && existing.PaymentsURL != "" {
				if existing.LocationID == "" {
					existing.LocationID = locationID
				}
				metadata = *existing
				baseConfigConfirmed = true
				break
			}
			// Success but empty/trace-only configuration: treat as "base
			// config not materialized yet", do NOT confirm and do NOT send
			// credentials; fall through to the bounded GET retry.
			s.logger.Warn().
				Int("attempt", attempt).
				Str("integration_id", integrationID.String()).
				Str("location_id", locationID).
				Msg("base config verification returned empty configuration; retrying verification fetch")
		} else {
			// An unauthorized/expired token must not be retried or treated as
			// "not ready yet": the credential POST is skipped and the error is
			// logged. Any other error (including HighLevel 400/422 for a base
			// configuration that does not exist yet) is retried on the GET only.
			if errors.Is(fetchErr, providers.ErrUnauthorized) {
				s.logger.Warn().
					Err(fetchErr).
					Str("integration_id", integrationID.String()).
					Str("location_id", locationID).
					Msg("base config verification unauthorized; skipping provider config creation")
				break
			}
			s.logger.Warn().
				Err(fetchErr).
				Int("attempt", attempt).
				Str("integration_id", integrationID.String()).
				Str("location_id", locationID).
				Msg("base configuration not confirmed yet; retrying verification fetch")
			if attempt < baseConfigVerifyAttempts {
				select {
				case <-ctx.Done():
					s.logger.Warn().Str("integration_id", integrationID.String()).Str("location_id", locationID).Msg("base config verification cancelled; skipping provider config creation")
					return ctx.Err()
				case <-time.After(baseConfigVerifyDelay):
				}
			}
		}
	}

	// Step 2b: Push the RVPay live/test processing keys to HighLevel
	// (POST /payments/custom-provider/connect?locationId=<id>) — but ONLY
	// after the base configuration has been confirmed to exist via the GET
	// above. The keys come exclusively from environment configuration. The
	// call is best-effort: a failure is logged and does not abort the
	// registration or the installation, and it can be retried on the next
	// registration.
	if !baseConfigConfirmed {
		s.logger.Warn().
			Str("integration_id", integrationID.String()).
			Str("location_id", locationID).
			Msg("base configuration could not be confirmed; skipping provider config creation")
	} else if s.providerConfig.LiveAPIKey != "" || s.providerConfig.LivePublishableKey != "" ||
		s.providerConfig.TestAPIKey != "" || s.providerConfig.TestPublishableKey != "" {
		creds := providers.ProviderCredentials{
			Live: providers.ProviderModeCredentials{
				APIKey:         s.providerConfig.LiveAPIKey,
				PublishableKey: s.providerConfig.LivePublishableKey,
				LiveMode:       true,
			},
			Test: providers.ProviderModeCredentials{
				APIKey:         s.providerConfig.TestAPIKey,
				PublishableKey: s.providerConfig.TestPublishableKey,
				LiveMode:       false,
			},
		}
		if cfgErr := s.CreateProviderConfigs(ctx, paymentClient, accessToken, locationID, creds); cfgErr != nil {
			s.logger.Warn().
				Err(cfgErr).
				Str("integration_id", integrationID.String()).
				Str("location_id", locationID).
				Msg("HighLevel provider config creation failed; association remains registered")
		}
	} else {
		s.logger.Warn().Str("integration_id", integrationID.String()).Str("location_id", locationID).Msg("no live/test provider keys configured; skipping provider config creation")
	}

	// Step 3: Persist the local provider configuration, reusing an existing API
	// key when a valid local config already exists. The provider API key is a
	// generated random value used to authenticate HighLevel query requests; it
	// is distinct from the OAuth access token and the pawaPay API key. It is
	// only regenerated when no local config with a non-empty key exists.
	existingLocal, getLocalErr := s.configRepo.GetByIntegrationID(ctx, integrationID)
	haveLocal := getLocalErr == nil
	if getLocalErr != nil && getLocalErr != repo.ErrNotFound {
		return translateError(getLocalErr)
	}

	apiKey := ""
	if haveLocal && existingLocal.ProviderApiKey != "" {
		apiKey = existingLocal.ProviderApiKey
	}
	if apiKey == "" {
		apiKey, err = generateAPIKey()
		if err != nil {
			return ErrAPIKeyGenerationFailed
		}
	}

	if haveLocal {
		_, err = s.configRepo.Update(ctx, integrationID, metadata.Name, metadata.Description, metadata.ImageURL, metadata.LocationID, metadata.QueryURL, metadata.PaymentsURL, metadata.SupportsSubscriptionSchedule, apiKey)
	} else {
		_, err = s.configRepo.Create(ctx, integrationID, metadata.Name, metadata.Description, metadata.ImageURL, metadata.LocationID, metadata.QueryURL, metadata.PaymentsURL, metadata.SupportsSubscriptionSchedule, apiKey)
	}
	if err == repo.ErrDuplicate {
		// A concurrent registration created the local config; update it instead.
		_, err = s.configRepo.Update(ctx, integrationID, metadata.Name, metadata.Description, metadata.ImageURL, metadata.LocationID, metadata.QueryURL, metadata.PaymentsURL, metadata.SupportsSubscriptionSchedule, apiKey)
	}
	if err != nil {
		return translateError(err)
	}

	s.logger.Info().Str("integration_id", integrationID.String()).Str("location_id", locationID).Msg("HighLevel provider registration completed")

	return nil
}

// CreateProviderConfigs pushes the RVPay live/test processing keys to
// HighLevel for an installed location using the supplied payment provider
// client. It validates that the access token and location ID are present and
// that at least one credential value is configured; otherwise it returns a
// typed error. It performs the outbound
// POST /payments/custom-provider/connect?locationId=<id> call.
func (s *Service) CreateProviderConfigs(ctx context.Context, paymentClient providers.PaymentProviderClient, accessToken, locationID string, creds providers.ProviderCredentials) error {
	s.logger.Info().Msg("CreateProviderConfigs initiated...")

	s.logger.Info().Str("location_id", locationID).Msg("location id for client")

	s.logger.Info().Str("access_token", accessToken).Msg("access token for client")

	s.logger.Info().Msgf("Payment Client: %v", paymentClient)

	s.logger.Info().Msgf("Provider Credentials: %v", creds)

	if paymentClient == nil {
		return ErrPaymentProviderNotSupported
	}
	if accessToken == "" {
		return ErrMissingAccessToken
	}
	if locationID == "" {
		return ErrMissingLocationID
	}
	if creds.Live.APIKey == "" && creds.Live.PublishableKey == "" &&
		creds.Test.APIKey == "" && creds.Test.PublishableKey == "" {
		return ErrProviderCredentialsNotConfigured
	}

	return paymentClient.CreateProviderConfigs(ctx, accessToken, locationID, creds)
}

// RefreshAccessToken refreshes an OAuth access token for an integration.
func (s *Service) RefreshAccessToken(ctx context.Context, integrationID uuid.UUID) error {
	integration, err := s.integrationsRepo.GetByID(ctx, integrationID)
	if err == repo.ErrNotFound {
		return ErrIntegrationNotFound
	}
	if err != nil {
		return translateError(err)
	}

	if integration.Status != sqlc.IntegrationStatusACTIVE {
		return ErrIntegrationNotActive
	}

	oauthToken, err := s.oauthRepo.GetByIntegrationID(ctx, integrationID)
	if err == repo.ErrNotFound {
		return ErrOAuthTokenNotFound
	}
	if err != nil {
		return translateError(err)
	}

	platform, err := s.platformsRepo.GetByID(ctx, integration.PlatformID)
	if err == repo.ErrNotFound {
		return ErrPlatformNotFound
	}
	if err != nil {
		return translateError(err)
	}

	provider, ok := s.registry.Get(platform.Slug)
	if !ok {
		return ErrProviderNotSupported
	}

	tokenResp, err := provider.OAuthProvider().RefreshToken(ctx, oauthToken.RefreshToken)
	if err != nil {
		s.logger.Error().Err(err).Str("integration_id", integrationID.String()).Msg("OAuth token refresh failed")
		return ErrTokenRefreshFailed
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	_, err = s.oauthRepo.Update(ctx, oauthToken.ID, tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt, tokenResp.Scope, tokenResp.TokenType)
	if err != nil {
		return translateError(err)
	}

	s.logger.Info().Str("integration_id", integrationID.String()).Msg("OAuth token refreshed")

	return nil
}

// ValidateToken validates an OAuth access token for an integration.
func (s *Service) ValidateToken(ctx context.Context, integrationID uuid.UUID) (bool, error) {
	oauthToken, err := s.oauthRepo.GetByIntegrationID(ctx, integrationID)
	if err == repo.ErrNotFound {
		return false, ErrOAuthTokenNotFound
	}
	if err != nil {
		return false, translateError(err)
	}

	if time.Now().After(oauthToken.ExpiresAt) {
		return false, nil
	}

	integration, err := s.integrationsRepo.GetByID(ctx, integrationID)
	if err == repo.ErrNotFound {
		return false, ErrIntegrationNotFound
	}
	if err != nil {
		return false, translateError(err)
	}

	if integration.Status != sqlc.IntegrationStatusACTIVE {
		return false, ErrIntegrationNotActive
	}

	platform, err := s.platformsRepo.GetByID(ctx, integration.PlatformID)
	if err == repo.ErrNotFound {
		return false, ErrPlatformNotFound
	}
	if err != nil {
		return false, translateError(err)
	}

	provider, ok := s.registry.Get(platform.Slug)
	if !ok {
		return false, ErrProviderNotSupported
	}

	valid, err := provider.OAuthProvider().ValidateToken(ctx, oauthToken.AccessToken)
	if err != nil {
		return false, err
	}

	return valid, nil
}

// generateAPIKey generates a cryptographically random API key used to
// authenticate HighLevel payment query requests. It is distinct from the
// OAuth access token and the pawaPay API key.
func generateAPIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

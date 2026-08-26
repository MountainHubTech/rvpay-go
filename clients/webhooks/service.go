package webhooks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/I-Frostbyte/rvpay-go/clients/db/repo"
	"github.com/I-Frostbyte/rvpay-go/clients/db/sqlc"
	"github.com/I-Frostbyte/rvpay-go/clients/providers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

// Service manages webhook lifecycle for provider integrations.
type Service struct {
	integrationsRepo          repo.IntegrationRepo
	clientsRepo               repo.ClientRepo
	webhookRepo               repo.WebhookSubscriptionRepo
	webhookEventRepo          repo.WebhookEventRepo
	platformsRepo             repo.PlatformRepo
	paymentProviderConfigRepo repo.PaymentProviderConfigRepo
	registry                  providers.ProviderRegistry
	dispatcher                providers.WebhookDispatcher
	logger                    zerolog.Logger
}

// NewService creates a new webhook service. dispatcher is the provider
// webhook event dispatcher used to process normalized events (e.g. the
// HighLevel INSTALL/UNINSTALL lifecycle). It may be nil; when nil, events are
// persisted but not dispatched.
//
// clientsRepo and platformsRepo are used by the HighLevel INSTALL flow to
// provision the RVPay tenant (client + integration) when the GHL location is
// not yet mapped. platformsRepo must resolve the already-existing HighLevel
// platform; this service never creates platform records.
func NewService(
	integrationsRepo repo.IntegrationRepo,
	clientsRepo repo.ClientRepo,
	webhookRepo repo.WebhookSubscriptionRepo,
	webhookEventRepo repo.WebhookEventRepo,
	platformsRepo repo.PlatformRepo,
	paymentProviderConfigRepo repo.PaymentProviderConfigRepo,
	registry providers.ProviderRegistry,
	dispatcher providers.WebhookDispatcher,
	logger zerolog.Logger,
) *Service {
	return &Service{
		integrationsRepo:          integrationsRepo,
		clientsRepo:               clientsRepo,
		webhookRepo:               webhookRepo,
		webhookEventRepo:          webhookEventRepo,
		platformsRepo:             platformsRepo,
		paymentProviderConfigRepo: paymentProviderConfigRepo,
		registry:                  registry,
		dispatcher:                dispatcher,
		logger:                    logger,
	}
}

// RegisterWebhook registers a webhook subscription for an integration.
func (s *Service) RegisterWebhook(ctx context.Context, integrationID uuid.UUID, callbackURL string) error {
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

	webhookProvider := provider.WebhookProvider()
	if webhookProvider == nil {
		return ErrProviderNotSupported
	}

	err = webhookProvider.RegisterWebhook(ctx, integrationID.String(), callbackURL)
	if err != nil {
		return err
	}

	_, err = s.webhookRepo.Create(ctx, integrationID, callbackURL, "highlevel", sqlc.WebhookSubscriptionStatusACTIVE)
	if err != nil {
		return translateError(err)
	}

	s.logger.Info().Str("integration_id", integrationID.String()).Str("callback_url", callbackURL).Msg("Webhook registered")

	return nil
}

// UnregisterWebhook removes a webhook subscription for an integration.
func (s *Service) UnregisterWebhook(ctx context.Context, integrationID uuid.UUID) error {
	webhook, err := s.webhookRepo.GetByIntegrationIDAndEndpoint(ctx, integrationID, "")
	if err == repo.ErrNotFound {
		return ErrWebhookNotFound
	}
	if err != nil {
		return translateError(err)
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

	webhookProvider := provider.WebhookProvider()
	if webhookProvider == nil {
		return ErrProviderNotSupported
	}

	err = webhookProvider.UnregisterWebhook(ctx, integrationID.String())
	if err != nil {
		return err
	}

	err = s.webhookRepo.Delete(ctx, webhook.ID)
	if err != nil {
		return translateError(err)
	}

	s.logger.Info().Str("integration_id", integrationID.String()).Msg("Webhook unregistered")

	return nil
}

// ProcessWebhook processes an incoming webhook request.
func (s *Service) ProcessWebhook(ctx context.Context, providerID string, headers map[string]string, body []byte) error {
	provider, ok := s.registry.Get(providerID)
	if !ok {
		return ErrProviderNotSupported
	}

	webhookProvider := provider.WebhookProvider()
	if webhookProvider == nil {
		return ErrProviderNotSupported
	}

	err := webhookProvider.VerifyRequest(ctx, headers, body)
	if err != nil {
		s.logger.Error().Err(err).Str("provider", providerID).Msg("Webhook verification failed")
		return ErrInvalidSignature
	}

	event, err := webhookProvider.ParseEvent(ctx, body)
	if err != nil {
		s.logger.Error().Err(err).Str("provider", providerID).Msg("Webhook payload parsing failed")
		return ErrInvalidPayload
	}

	// For HighLevel webhooks, the GHL appId is NOT an RVPay UUID. Resolve the
	// actual RVPay integration UUID deterministically from the GHL locationId.
	//
	// For INSTALL events, the pre-provisioned integration mapping
	// (integration.external_account_id = GHL locationId) is authoritative and
	// must be resolved BEFORE the payment_provider_configs record exists. The
	// INSTALL handler is dispatched first so it can create the
	// payment_provider_configs record idempotently; the webhook subscription
	// requirement is relaxed for INSTALL because a first installation has no
	// subscription yet (it is registered after OAuth completes). For
	// non-INSTALL events, the existing behavior is preserved: resolve via the
	// payment_provider_configs table, require the webhook subscription, then
	// dispatch.
	var integrationID uuid.UUID
	isInstall := providerID == "highlevel" && (event.EventType == "INSTALL" || event.EventType == "integration.installed")
	if providerID == "highlevel" && event.LocationID != "" {
		if isInstall {
			// First-install flow: resolve the tenant (client + integration) for
			// this GHL sub-account. The pre-provisioned integration mapping
			// (integrations.external_account_id = GHL locationId) is
			// authoritative; fall back to the payment_provider_configs mapping;
			// if neither exists, provision the tenant so the INSTALL flow can
			// proceed. This is idempotent: a repeated INSTALL reuses existing
			// records.
			integrationID, err = s.resolveOrProvisionTenant(ctx, event.LocationID)
			if err != nil {
				return err
			}
		} else {
			config, err := s.paymentProviderConfigRepo.GetByLocationID(ctx, event.LocationID)
			if err == repo.ErrNotFound {
				return ErrIntegrationNotFound
			}
			if err != nil {
				return translateError(err)
			}
			integrationID = config.IntegrationID
		}
	} else {
		integrationID, err = uuid.Parse(event.IntegrationID)
		if err != nil {
			return ErrInvalidPayload
		}
	}

	// Idempotency: record the event atomically. The unique constraint on
	// (integration_id, provider_event_id) plus ON CONFLICT DO NOTHING makes
	// duplicate deliveries race-safe: a concurrent retry of the same event
	// will not insert a second row and is treated as a duplicate.
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		s.logger.Error().Err(err).Str("event_id", event.ProviderEventID).Msg("Webhook payload marshal failed")
		return ErrInvalidPayload
	}

	_, err = s.webhookEventRepo.Create(ctx, integrationID, event.ProviderEventID, event.EventType, payload)
	if err == repo.ErrDuplicate {
		s.logger.Info().Str("event_id", event.ProviderEventID).Str("integration_id", integrationID.String()).Msg("Duplicate webhook event ignored")
		return ErrDuplicateEvent
	}
	if err != nil {
		return translateError(err)
	}

	// For INSTALL events, dispatch the INSTALL handler BEFORE the webhook
	// subscription check. The INSTALL handler creates/finds the
	// payment_provider_configs record idempotently. A first installation has
	// no webhook subscription yet (it is registered after OAuth completes), so
	// the INSTALL event must not fail merely because the subscription or the
	// config does not exist yet.
	if isInstall {
		if s.dispatcher != nil {
			err = s.dispatcher.Dispatch(ctx, event)
			if err != nil {
				s.logger.Error().Err(err).Str("event_id", event.ProviderEventID).Msg("Webhook event dispatch failed")
				return err
			}
		}
		s.logger.Info().Str("provider", providerID).Str("event_type", event.EventType).Str("event_id", event.ProviderEventID).Msg("Webhook processed successfully")
		return nil
	}

	webhook, err := s.webhookRepo.GetByIntegrationIDAndEndpoint(ctx, integrationID, "")
	if err == repo.ErrNotFound {
		return ErrWebhookNotFound
	}
	if err != nil {
		return translateError(err)
	}

	s.logger.Info().Str("event_id", event.ProviderEventID).Str("integration_id", integrationID.String()).Msg("Webhook subscription exists, processing event")

	lastDelivery := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	_, err = s.webhookRepo.UpdateLastDelivery(ctx, webhook.ID, lastDelivery)
	if err != nil {
		return translateError(err)
	}

	if s.dispatcher != nil {
		err = s.dispatcher.Dispatch(ctx, event)
		if err != nil {
			s.logger.Error().Err(err).Str("event_id", event.ProviderEventID).Msg("Webhook event dispatch failed")
			return err
		}
	}

	s.logger.Info().Str("provider", providerID).Str("event_type", event.EventType).Str("event_id", event.ProviderEventID).Msg("Webhook processed successfully")

	return nil
}

// resolveOrProvisionTenant resolves the RVPay integration for a HighLevel
// INSTALL location. Resolution order:
//  1. integrations.external_account_id = locationId (authoritative
//     provisioning mapping established by a prior INSTALL).
//  2. payment_provider_configs.location_id = locationId (integration already
//     active with a registered payment provider config).
//  3. Otherwise, provision the tenant: reuse the client for this GHL
//     sub-account (create when missing) and create the integration mapped to
//     the locationId. Only the INSTALL event triggers provisioning.
//
// It never creates or modifies platform records: the HighLevel platform is
// resolved by slug and must already exist.
func (s *Service) resolveOrProvisionTenant(ctx context.Context, locationID string) (uuid.UUID, error) {
	// Authoritative provisioning mapping.
	integration, err := s.integrationsRepo.GetByExternalAccountID(ctx, locationID)
	if err == nil {
		return integration.ID, nil
	}
	if err != repo.ErrNotFound {
		return uuid.Nil, translateError(err)
	}

	// Fall back to the payment provider config mapping.
	config, configErr := s.paymentProviderConfigRepo.GetByLocationID(ctx, locationID)
	if configErr == nil {
		return config.IntegrationID, nil
	}
	if configErr != repo.ErrNotFound {
		return uuid.Nil, translateError(configErr)
	}

	return s.provisionTenant(ctx, locationID)
}

// provisionTenant creates the RVPay client and integration for a HighLevel
// sub-account identified by its locationId, if they do not already exist. It
// is idempotent: an existing client/integration is reused rather than
// duplicated.
func (s *Service) provisionTenant(ctx context.Context, locationID string) (uuid.UUID, error) {
	if s.clientsRepo == nil {
		return uuid.Nil, ErrIntegrationNotFound
	}

	// Resolve the existing HighLevel platform by slug. Never create/modify
	// platform records.
	platform, err := s.platformsRepo.GetBySlug(ctx, "highlevel")
	if err == repo.ErrNotFound {
		return uuid.Nil, ErrPlatformNotFound
	}
	if err != nil {
		return uuid.Nil, translateError(err)
	}

	// Derive a deterministic tenant client name from the GHL sub-account so a
	// repeated INSTALL reuses the same client.
	clientName := "highlevel-" + locationID

	// Reuse the client when it already exists; otherwise create it ACTIVE so
	// the later OAuth callback can reuse it.
	client, err := s.clientsRepo.GetByName(ctx, clientName)
	if err == repo.ErrNotFound {
		client, err = s.clientsRepo.Create(ctx, clientName, sqlc.ClientStatusACTIVE)
	}
	if err == repo.ErrDuplicate {
		// A concurrent INSTALL created the client; reuse it.
		client, err = s.clientsRepo.GetByName(ctx, clientName)
	}
	if err != nil {
		if err == repo.ErrNotFound {
			return uuid.Nil, ErrClientNotFound
		}
		return uuid.Nil, translateError(err)
	}

	// Reuse the integration when it already exists (idempotency); otherwise
	// create it mapped to the locationId with status CREATED, which the OAuth
	// callback activates later.
	existing, existingErr := s.integrationsRepo.GetByClientAndPlatform(ctx, client.ID, platform.ID)
	if existingErr == nil {
		return existing.ID, nil
	}
	if existingErr != repo.ErrNotFound {
		return uuid.Nil, translateError(existingErr)
	}

	integration, err := s.integrationsRepo.Create(ctx, client.ID, platform.ID, locationID, sqlc.IntegrationStatusCREATED)
	if err == repo.ErrDuplicate {
		// A concurrent INSTALL created the integration; reuse it.
		integration, err = s.integrationsRepo.GetByExternalAccountID(ctx, locationID)
	}
	if err != nil {
		if err == repo.ErrNotFound {
			return uuid.Nil, ErrIntegrationNotFound
		}
		return uuid.Nil, translateError(err)
	}

	s.logger.Info().Str("location_id", locationID).Str("client_id", client.ID.String()).Str("integration_id", integration.ID.String()).Msg("HighLevel INSTALL provisioned client and integration")

	return integration.ID, nil
}

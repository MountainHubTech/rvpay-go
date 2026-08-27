package webhooks

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"github.com/I-Frostbyte/rvpay-go/clients/db/repo"
	"github.com/I-Frostbyte/rvpay-go/clients/db/sqlc"
	"github.com/I-Frostbyte/rvpay-go/clients/providers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// testWebhookEd25519KeyPair generates a fresh Ed25519 key pair and returns the
// PEM-encoded public key and the raw private key for signing test payloads.
func testWebhookEd25519KeyPair(t *testing.T) (publicKeyPEM string, privateKey ed25519.PrivateKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	publicKeyPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}))

	return publicKeyPEM, priv
}

// signWebhookBody signs the raw body with the private key and returns the
// base64 encoded signature for the X-GHL-Signature header.
func signWebhookBody(t *testing.T, priv ed25519.PrivateKey, body []byte) string {
	t.Helper()
	sig := ed25519.Sign(priv, body)
	return base64.StdEncoding.EncodeToString(sig)
}

// mockWebhookIntegrationRepo is a minimal IntegrationRepo test double
type mockWebhookIntegrationRepo struct {
	integrations map[string]sqlc.Integration
}

func newMockWebhookIntegrationRepo() *mockWebhookIntegrationRepo {
	return &mockWebhookIntegrationRepo{
		integrations: make(map[string]sqlc.Integration),
	}
}

func (m *mockWebhookIntegrationRepo) Create(ctx context.Context, clientID, platformID uuid.UUID, externalAccountID string, status sqlc.IntegrationStatus) (sqlc.Integration, error) {
	// Simulate the unique (client_id, platform_id) constraint.
	for _, i := range m.integrations {
		if i.ClientID == clientID && i.PlatformID == platformID {
			return sqlc.Integration{}, repo.ErrDuplicate
		}
	}
	integration := sqlc.Integration{
		ID:                uuid.New(),
		ClientID:          clientID,
		PlatformID:        platformID,
		ExternalAccountID: externalAccountID,
		Status:            status,
	}
	m.integrations[integration.ID.String()] = integration
	return integration, nil
}

func (m *mockWebhookIntegrationRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.Integration, error) {
	i, ok := m.integrations[id.String()]
	if !ok {
		return sqlc.Integration{}, repo.ErrNotFound
	}
	return i, nil
}

func (m *mockWebhookIntegrationRepo) GetByClientAndPlatform(ctx context.Context, clientID, platformID uuid.UUID) (sqlc.Integration, error) {
	for _, i := range m.integrations {
		if i.ClientID == clientID && i.PlatformID == platformID {
			return i, nil
		}
	}
	return sqlc.Integration{}, repo.ErrNotFound
}

func (m *mockWebhookIntegrationRepo) GetByExternalAccountID(ctx context.Context, externalAccountID string) (sqlc.Integration, error) {
	for _, i := range m.integrations {
		if i.ExternalAccountID == externalAccountID {
			return i, nil
		}
	}
	return sqlc.Integration{}, repo.ErrNotFound
}

func (m *mockWebhookIntegrationRepo) ListByClient(ctx context.Context, clientID uuid.UUID, limit, offset int32) ([]sqlc.Integration, error) {
	return nil, nil
}

func (m *mockWebhookIntegrationRepo) ListByPlatform(ctx context.Context, platformID uuid.UUID, limit, offset int32) ([]sqlc.Integration, error) {
	return nil, nil
}

func (m *mockWebhookIntegrationRepo) ListActiveByClient(ctx context.Context, clientID uuid.UUID, limit, offset int32) ([]sqlc.Integration, error) {
	return nil, nil
}

func (m *mockWebhookIntegrationRepo) CountByClient(ctx context.Context, clientID uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *mockWebhookIntegrationRepo) ExistsByClientAndPlatform(ctx context.Context, clientID, platformID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockWebhookIntegrationRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.IntegrationStatus) (sqlc.Integration, error) {
	return sqlc.Integration{}, nil
}

func (m *mockWebhookIntegrationRepo) UpdateLastSyncAt(ctx context.Context, id uuid.UUID, lastSyncAt pgtype.Timestamptz) (sqlc.Integration, error) {
	return sqlc.Integration{}, nil
}

func (m *mockWebhookIntegrationRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

// mockPaymentProviderConfigRepo is a minimal PaymentProviderConfigRepo test double
type mockPaymentProviderConfigRepo struct {
	configs map[string]sqlc.PaymentProviderConfig
}

func newMockPaymentProviderConfigRepo() *mockPaymentProviderConfigRepo {
	return &mockPaymentProviderConfigRepo{
		configs: make(map[string]sqlc.PaymentProviderConfig),
	}
}

func (m *mockPaymentProviderConfigRepo) Create(ctx context.Context, integrationID uuid.UUID, providerName, providerDescription, providerImageURL, locationID, queryURL, paymentsURL string, supportsSubscriptionSchedule bool, providerAPIKey string) (sqlc.PaymentProviderConfig, error) {
	// Idempotent: if a config already exists for this integration, return ErrDuplicate.
	for _, c := range m.configs {
		if c.IntegrationID == integrationID {
			return sqlc.PaymentProviderConfig{}, repo.ErrDuplicate
		}
	}
	config := sqlc.PaymentProviderConfig{
		ID:                           uuid.New(),
		IntegrationID:                integrationID,
		ProviderName:                 providerName,
		ProviderDescription:          providerDescription,
		ProviderImageUrl:             providerImageURL,
		LocationID:                   locationID,
		QueryUrl:                     queryURL,
		PaymentsUrl:                  paymentsURL,
		SupportsSubscriptionSchedule: supportsSubscriptionSchedule,
		ProviderApiKey:               providerAPIKey,
	}
	m.configs[config.ID.String()] = config
	return config, nil
}

func (m *mockPaymentProviderConfigRepo) GetByIntegrationID(ctx context.Context, integrationID uuid.UUID) (sqlc.PaymentProviderConfig, error) {
	return sqlc.PaymentProviderConfig{}, repo.ErrNotFound
}

func (m *mockPaymentProviderConfigRepo) GetByLocationID(ctx context.Context, locationID string) (sqlc.PaymentProviderConfig, error) {
	for _, c := range m.configs {
		if c.LocationID == locationID {
			return c, nil
		}
	}
	return sqlc.PaymentProviderConfig{}, repo.ErrNotFound
}

func (m *mockPaymentProviderConfigRepo) GetByAPIKey(ctx context.Context, apiKey string) (sqlc.PaymentProviderConfig, error) {
	return sqlc.PaymentProviderConfig{}, repo.ErrNotFound
}

func (m *mockPaymentProviderConfigRepo) Update(ctx context.Context, integrationID uuid.UUID, providerName, providerDescription, providerImageURL, locationID, queryURL, paymentsURL string, supportsSubscriptionSchedule bool, providerAPIKey string) (sqlc.PaymentProviderConfig, error) {
	return sqlc.PaymentProviderConfig{}, nil
}

func (m *mockPaymentProviderConfigRepo) Delete(ctx context.Context, integrationID uuid.UUID) error {
	return nil
}

// mockWebhookClientRepo is a minimal ClientRepo test double for the INSTALL
// tenant-provisioning flow.
type mockWebhookClientRepo struct {
	clients map[string]sqlc.Client
}

func newMockWebhookClientRepo() *mockWebhookClientRepo {
	cr := &mockWebhookClientRepo{
		clients: make(map[string]sqlc.Client),
	}
	return cr
}

func (m *mockWebhookClientRepo) Create(ctx context.Context, name string, status sqlc.ClientStatus) (sqlc.Client, error) {
	for _, c := range m.clients {
		if c.ClientName == name {
			// Simulate the unique-name collision; return the existing record so
			// callers can reuse it idempotently.
			return c, repo.ErrDuplicate
		}
	}
	client := sqlc.Client{
		ID:         uuid.New(),
		ClientName: name,
		Status:     status,
	}
	m.clients[client.ID.String()] = client
	return client, nil
}

func (m *mockWebhookClientRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.Client, error) {
	client, ok := m.clients[id.String()]
	if !ok {
		return sqlc.Client{}, repo.ErrNotFound
	}
	return client, nil
}

func (m *mockWebhookClientRepo) GetByName(ctx context.Context, name string) (sqlc.Client, error) {
	for _, c := range m.clients {
		if c.ClientName == name {
			return c, nil
		}
	}
	return sqlc.Client{}, repo.ErrNotFound
}

func (m *mockWebhookClientRepo) List(ctx context.Context, limit, offset int32) ([]sqlc.Client, error) {
	return nil, nil
}

func (m *mockWebhookClientRepo) ListActive(ctx context.Context, limit, offset int32) ([]sqlc.Client, error) {
	return nil, nil
}

func (m *mockWebhookClientRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.clients)), nil
}

func (m *mockWebhookClientRepo) ExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	_, ok := m.clients[id.String()]
	return ok, nil
}

func (m *mockWebhookClientRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.ClientStatus) (sqlc.Client, error) {
	client, ok := m.clients[id.String()]
	if !ok {
		return sqlc.Client{}, repo.ErrNotFound
	}
	client.Status = status
	m.clients[id.String()] = client
	return client, nil
}

func (m *mockWebhookClientRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.clients[id.String()]; !ok {
		return repo.ErrNotFound
	}
	delete(m.clients, id.String())
	return nil
}

// mockWebhookPlatformRepo is a minimal PlatformRepo test double
type mockWebhookPlatformRepo struct {
	platforms map[string]sqlc.Platform
}

func newMockWebhookPlatformRepo() *mockWebhookPlatformRepo {
	return &mockWebhookPlatformRepo{
		platforms: make(map[string]sqlc.Platform),
	}
}

func (m *mockWebhookPlatformRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.Platform, error) {
	p, ok := m.platforms[id.String()]
	if !ok {
		return sqlc.Platform{}, repo.ErrNotFound
	}
	return p, nil
}

func (m *mockWebhookPlatformRepo) Create(ctx context.Context, name, displayName, slug string, enabled, oauthCapable, webhookCapable bool) (sqlc.Platform, error) {
	return sqlc.Platform{}, nil
}

func (m *mockWebhookPlatformRepo) GetByName(ctx context.Context, name string) (sqlc.Platform, error) {
	return sqlc.Platform{}, repo.ErrNotFound
}

func (m *mockWebhookPlatformRepo) GetBySlug(ctx context.Context, slug string) (sqlc.Platform, error) {
	for _, p := range m.platforms {
		if p.Slug == slug {
			return p, nil
		}
	}
	return sqlc.Platform{}, repo.ErrNotFound
}

func (m *mockWebhookPlatformRepo) List(ctx context.Context, limit, offset int32) ([]sqlc.Platform, error) {
	return nil, nil
}

func (m *mockWebhookPlatformRepo) ListEnabled(ctx context.Context, limit, offset int32) ([]sqlc.Platform, error) {
	return nil, nil
}

func (m *mockWebhookPlatformRepo) Count(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockWebhookPlatformRepo) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	return false, nil
}

func (m *mockWebhookPlatformRepo) Update(ctx context.Context, id uuid.UUID, name, displayName, slug string, enabled, oauthCapable, webhookCapable bool) (sqlc.Platform, error) {
	return sqlc.Platform{}, nil
}

func (m *mockWebhookPlatformRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

// mockWebhookEventRepo is a WebhookEventRepo test double
type mockWebhookEventRepo struct {
	events map[string]sqlc.WebhookEvent
}

func newMockWebhookEventRepo() *mockWebhookEventRepo {
	return &mockWebhookEventRepo{
		events: make(map[string]sqlc.WebhookEvent),
	}
}

func (m *mockWebhookEventRepo) Create(ctx context.Context, integrationID uuid.UUID, providerEventID, eventType string, payload []byte) (sqlc.WebhookEvent, error) {
	key := integrationID.String() + ":" + providerEventID
	if _, ok := m.events[key]; ok {
		return sqlc.WebhookEvent{}, repo.ErrDuplicate
	}
	event := sqlc.WebhookEvent{
		ID:              uuid.New(),
		IntegrationID:   integrationID,
		ProviderEventID: providerEventID,
		EventType:       eventType,
		Payload:         payload,
	}
	m.events[key] = event
	return event, nil
}

func (m *mockWebhookEventRepo) GetByIntegrationAndProvider(ctx context.Context, integrationID uuid.UUID, providerEventID string) (sqlc.WebhookEvent, error) {
	key := integrationID.String() + ":" + providerEventID
	event, ok := m.events[key]
	if !ok {
		return sqlc.WebhookEvent{}, repo.ErrNotFound
	}
	return event, nil
}

// mockWebhookRepo is a WebhookSubscriptionRepo test double
type mockWebhookRepo struct {
	subscriptions map[string]sqlc.WebhookSubscription
}

func newMockWebhookRepo() *mockWebhookRepo {
	return &mockWebhookRepo{
		subscriptions: make(map[string]sqlc.WebhookSubscription),
	}
}

func (m *mockWebhookRepo) Create(ctx context.Context, integrationID uuid.UUID, endpoint, secret string, status sqlc.WebhookSubscriptionStatus) (sqlc.WebhookSubscription, error) {
	sub := sqlc.WebhookSubscription{
		ID:            uuid.New(),
		IntegrationID: integrationID,
		Endpoint:      endpoint,
		Secret:        secret,
		Status:        status,
	}
	m.subscriptions[sub.ID.String()] = sub
	return sub, nil
}

func (m *mockWebhookRepo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.WebhookSubscription, error) {
	sub, ok := m.subscriptions[id.String()]
	if !ok {
		return sqlc.WebhookSubscription{}, repo.ErrNotFound
	}
	return sub, nil
}

func (m *mockWebhookRepo) GetByIntegrationIDAndEndpoint(ctx context.Context, integrationID uuid.UUID, endpoint string) (sqlc.WebhookSubscription, error) {
	for _, sub := range m.subscriptions {
		if sub.IntegrationID == integrationID {
			return sub, nil
		}
	}
	return sqlc.WebhookSubscription{}, repo.ErrNotFound
}

func (m *mockWebhookRepo) ListByIntegrationID(ctx context.Context, integrationID uuid.UUID, limit, offset int32) ([]sqlc.WebhookSubscription, error) {
	return nil, nil
}

func (m *mockWebhookRepo) ListActiveByIntegrationID(ctx context.Context, integrationID uuid.UUID) ([]sqlc.WebhookSubscription, error) {
	return nil, nil
}

func (m *mockWebhookRepo) CountByIntegrationID(ctx context.Context, integrationID uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *mockWebhookRepo) Exists(ctx context.Context, integrationID uuid.UUID, endpoint string) (bool, error) {
	return false, nil
}

func (m *mockWebhookRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status sqlc.WebhookSubscriptionStatus) (sqlc.WebhookSubscription, error) {
	sub, ok := m.subscriptions[id.String()]
	if !ok {
		return sqlc.WebhookSubscription{}, repo.ErrNotFound
	}
	sub.Status = status
	m.subscriptions[id.String()] = sub
	return sub, nil
}

func (m *mockWebhookRepo) UpdateLastDelivery(ctx context.Context, id uuid.UUID, lastDelivery pgtype.Timestamptz) (sqlc.WebhookSubscription, error) {
	sub, ok := m.subscriptions[id.String()]
	if !ok {
		return sqlc.WebhookSubscription{}, repo.ErrNotFound
	}
	sub.LastDelivery = lastDelivery
	m.subscriptions[id.String()] = sub
	return sub, nil
}

func (m *mockWebhookRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.subscriptions[id.String()]; !ok {
		return repo.ErrNotFound
	}
	delete(m.subscriptions, id.String())
	return nil
}

func TestProcessWebhookUnknownProvider(t *testing.T) {
	t.Parallel()

	registry := providers.NewProviderRegistry()

	svc := NewService(
		newMockWebhookIntegrationRepo(),
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		newMockPaymentProviderConfigRepo(),
		registry,
		nil, // no dispatcher in this test
		zerolog.Nop(),
	)

	err := svc.ProcessWebhook(context.Background(), "unknown", map[string]string{}, []byte(`{}`))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestProcessWebhookInvalidSignature(t *testing.T) {
	t.Parallel()

	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", "test-webhook-secret", nil, zerolog.Nop()))

	svc := NewService(
		newMockWebhookIntegrationRepo(),
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		newMockPaymentProviderConfigRepo(),
		registry,
		nil, // no dispatcher in this test
		zerolog.Nop(),
	)

	headers := map[string]string{}
	err := svc.ProcessWebhook(context.Background(), "highlevel", headers, []byte(`{"eventId":"test"}`))
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestRegisterWebhookIntegrationNotFound(t *testing.T) {
	t.Parallel()

	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", "test-webhook-secret", nil, zerolog.Nop()))

	svc := NewService(
		newMockWebhookIntegrationRepo(),
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		newMockPaymentProviderConfigRepo(),
		registry,
		nil, // no dispatcher in this test
		zerolog.Nop(),
	)

	err := svc.RegisterWebhook(context.Background(), uuid.New(), "https://example.com/callback")
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.NotFound)
	}
}

func TestUnregisterWebhookNotFound(t *testing.T) {
	t.Parallel()

	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", "test-webhook-secret", nil, zerolog.Nop()))

	svc := NewService(
		newMockWebhookIntegrationRepo(),
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		newMockPaymentProviderConfigRepo(),
		registry,
		nil, // no dispatcher in this test
		zerolog.Nop(),
	)

	err := svc.UnregisterWebhook(context.Background(), uuid.New())
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.NotFound)
	}
}

// mockWebhookDispatcher is a WebhookDispatcher test double that records the
// dispatched events and can be configured to fail.
type mockWebhookDispatcher struct {
	events []*providers.WebhookEvent
	err    error
}

func (m *mockWebhookDispatcher) Dispatch(ctx context.Context, event *providers.WebhookEvent) error {
	m.events = append(m.events, event)
	return m.err
}

// newTestWebhookService builds a webhook Service wired to in-memory mocks and
// a HighLevel provider with a valid Ed25519 key for signature verification.
// It returns the service, the mocks for assertions, and the private key for
// signing test payloads.
func newTestWebhookService(t *testing.T, dispatcher providers.WebhookDispatcher) (*Service, *mockWebhookIntegrationRepo, *mockWebhookRepo, *mockWebhookEventRepo, *mockPaymentProviderConfigRepo, ed25519.PrivateKey) {
	t.Helper()

	integrationRepo := newMockWebhookIntegrationRepo()
	webhookRepo := newMockWebhookRepo()
	eventRepo := newMockWebhookEventRepo()
	configRepo := newMockPaymentProviderConfigRepo()

	publicKeyPEM, priv := testWebhookEd25519KeyPair(t)

	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", publicKeyPEM, nil, zerolog.Nop()))

	svc := NewService(
		integrationRepo,
		newMockWebhookClientRepo(),
		webhookRepo,
		eventRepo,
		newMockWebhookPlatformRepo(),
		configRepo,
		registry,
		dispatcher,
		zerolog.Nop(),
	)

	return svc, integrationRepo, webhookRepo, eventRepo, configRepo, priv
}

func TestProcessWebhook_InstallResolvesPreProvisionedIntegration(t *testing.T) {
	t.Parallel()

	dispatcher := &mockWebhookDispatcher{}
	svc, integrationRepo, _, eventRepo, _, priv := newTestWebhookService(t, dispatcher)

	// Pre-provisioned integration with external_account_id = GHL locationId.
	clientID := uuid.New()
	platformID := uuid.New()
	integrationID := uuid.New()
	integrationRepo.integrations[integrationID.String()] = sqlc.Integration{
		ID:                integrationID,
		ClientID:          clientID,
		PlatformID:        platformID,
		ExternalAccountID: "GHL_LOCATION_A",
		Status:            sqlc.IntegrationStatusCREATED,
	}

	// GHL INSTALL webhook payload.
	body := []byte(`{"type":"INSTALL","appId":"app-1","locationId":"GHL_LOCATION_A","companyId":"company-1","timestamp":"2026-08-17T09:06:59.366Z","webhookId":"evt-install-1"}`)
	headers := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body),
	}

	err := svc.ProcessWebhook(context.Background(), "highlevel", headers, body)
	if err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}

	// The INSTALL handler must have been dispatched.
	if len(dispatcher.events) != 1 {
		t.Fatalf("expected 1 dispatched event, got %d", len(dispatcher.events))
	}
	if dispatcher.events[0].EventType != "INSTALL" {
		t.Fatalf("dispatched event type = %s, want INSTALL", dispatcher.events[0].EventType)
	}
	if dispatcher.events[0].LocationID != "GHL_LOCATION_A" {
		t.Fatalf("dispatched event LocationID = %q, want GHL_LOCATION_A", dispatcher.events[0].LocationID)
	}

	// The event must be persisted for idempotency.
	if len(eventRepo.events) != 1 {
		t.Fatalf("expected 1 persisted event, got %d", len(eventRepo.events))
	}
}

func TestProcessWebhook_InstallCreatesConfig(t *testing.T) {
	t.Parallel()

	// Use the real HighLevel dispatcher so the INSTALL handler creates the
	// payment_provider_configs record.
	integrationRepo := newMockWebhookIntegrationRepo()
	configRepo := newMockPaymentProviderConfigRepo()
	dispatcher := providers.NewHighLevelWebhookDispatcher(
		&testWebhookLogger{},
		integrationRepo,
		configRepo,
		providers.ProviderConfigSettings{Name: "RVPay", QueryURL: "https://api.example.com/query", PaymentsURL: "https://checkout.example.com/pay"},
	)

	publicKeyPEM, priv := testWebhookEd25519KeyPair(t)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", publicKeyPEM, nil, zerolog.Nop()))

	svc := NewService(
		integrationRepo,
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		configRepo,
		registry,
		dispatcher,
		zerolog.Nop(),
	)

	// Pre-provisioned integration with external_account_id = GHL locationId.
	clientID := uuid.New()
	platformID := uuid.New()
	integrationID := uuid.New()
	integrationRepo.integrations[integrationID.String()] = sqlc.Integration{
		ID:                integrationID,
		ClientID:          clientID,
		PlatformID:        platformID,
		ExternalAccountID: "GHL_LOCATION_A",
		Status:            sqlc.IntegrationStatusCREATED,
	}

	body := []byte(`{"type":"INSTALL","appId":"app-1","locationId":"GHL_LOCATION_A","companyId":"company-1","timestamp":"2026-08-17T09:06:59.366Z","webhookId":"evt-install-1"}`)
	headers := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body),
	}

	err := svc.ProcessWebhook(context.Background(), "highlevel", headers, body)
	if err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}

	// The config must be created for the resolved integration.
	if len(configRepo.configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configRepo.configs))
	}
	for _, c := range configRepo.configs {
		if c.IntegrationID != integrationID {
			t.Fatalf("config IntegrationID = %v, want %v", c.IntegrationID, integrationID)
		}
		if c.LocationID != "GHL_LOCATION_A" {
			t.Fatalf("config LocationID = %q, want GHL_LOCATION_A", c.LocationID)
		}
	}
}

func TestProcessWebhook_InstallReusesExistingConfig(t *testing.T) {
	t.Parallel()

	integrationRepo := newMockWebhookIntegrationRepo()
	configRepo := newMockPaymentProviderConfigRepo()
	dispatcher := providers.NewHighLevelWebhookDispatcher(
		&testWebhookLogger{},
		integrationRepo,
		configRepo,
		providers.ProviderConfigSettings{},
	)

	publicKeyPEM, priv := testWebhookEd25519KeyPair(t)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", publicKeyPEM, nil, zerolog.Nop()))

	svc := NewService(
		integrationRepo,
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		configRepo,
		registry,
		dispatcher,
		zerolog.Nop(),
	)

	clientID := uuid.New()
	platformID := uuid.New()
	integrationID := uuid.New()
	integrationRepo.integrations[integrationID.String()] = sqlc.Integration{
		ID:                integrationID,
		ClientID:          clientID,
		PlatformID:        platformID,
		ExternalAccountID: "GHL_LOCATION_A",
		Status:            sqlc.IntegrationStatusCREATED,
	}
	// Pre-existing config for the integration.
	configRepo.configs["existing"] = sqlc.PaymentProviderConfig{
		ID:            uuid.New(),
		IntegrationID: integrationID,
		LocationID:    "GHL_LOCATION_A",
	}

	body := []byte(`{"type":"INSTALL","appId":"app-1","locationId":"GHL_LOCATION_A","companyId":"company-1","timestamp":"2026-08-17T09:06:59.366Z","webhookId":"evt-install-1"}`)
	headers := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body),
	}

	err := svc.ProcessWebhook(context.Background(), "highlevel", headers, body)
	if err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}

	// The existing config must be reused (not duplicated).
	if len(configRepo.configs) != 1 {
		t.Fatalf("expected 1 config (reused), got %d", len(configRepo.configs))
	}
}

func TestProcessWebhook_InstallMultipleClientsSelectsCorrect(t *testing.T) {
	t.Parallel()

	integrationRepo := newMockWebhookIntegrationRepo()
	configRepo := newMockPaymentProviderConfigRepo()
	dispatcher := providers.NewHighLevelWebhookDispatcher(
		&testWebhookLogger{},
		integrationRepo,
		configRepo,
		providers.ProviderConfigSettings{},
	)

	publicKeyPEM, priv := testWebhookEd25519KeyPair(t)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", publicKeyPEM, nil, zerolog.Nop()))

	svc := NewService(
		integrationRepo,
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		configRepo,
		registry,
		dispatcher,
		zerolog.Nop(),
	)

	// Client A -> Integration A (location LOCATION_A)
	clientA := uuid.New()
	platformID := uuid.New()
	integrationA := uuid.New()
	integrationRepo.integrations[integrationA.String()] = sqlc.Integration{
		ID:                integrationA,
		ClientID:          clientA,
		PlatformID:        platformID,
		ExternalAccountID: "LOCATION_A",
		Status:            sqlc.IntegrationStatusCREATED,
	}

	// Client B -> Integration B (location LOCATION_B)
	clientB := uuid.New()
	integrationB := uuid.New()
	integrationRepo.integrations[integrationB.String()] = sqlc.Integration{
		ID:                integrationB,
		ClientID:          clientB,
		PlatformID:        platformID,
		ExternalAccountID: "LOCATION_B",
		Status:            sqlc.IntegrationStatusCREATED,
	}

	// INSTALL for Client B's location must resolve to Integration B, not A.
	body := []byte(`{"type":"INSTALL","appId":"app-1","locationId":"LOCATION_B","companyId":"company-1","timestamp":"2026-08-17T09:06:59.366Z","webhookId":"evt-install-B"}`)
	headers := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body),
	}

	err := svc.ProcessWebhook(context.Background(), "highlevel", headers, body)
	if err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}

	if len(configRepo.configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configRepo.configs))
	}
	for _, c := range configRepo.configs {
		if c.IntegrationID != integrationB {
			t.Fatalf("config IntegrationID = %v, want %v (Client B's integration)", c.IntegrationID, integrationB)
		}
		if c.LocationID != "LOCATION_B" {
			t.Fatalf("config LocationID = %q, want LOCATION_B", c.LocationID)
		}
	}
}

func TestProcessWebhook_InstallMissingMappingFailsSafely(t *testing.T) {
	t.Parallel()

	integrationRepo := newMockWebhookIntegrationRepo()
	configRepo := newMockPaymentProviderConfigRepo()
	dispatcher := providers.NewHighLevelWebhookDispatcher(
		&testWebhookLogger{},
		integrationRepo,
		configRepo,
		providers.ProviderConfigSettings{},
	)

	publicKeyPEM, priv := testWebhookEd25519KeyPair(t)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", publicKeyPEM, nil, zerolog.Nop()))

	svc := NewService(
		integrationRepo,
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		configRepo,
		registry,
		dispatcher,
		zerolog.Nop(),
	)

	// No integration or config exists for this location.
	body := []byte(`{"type":"INSTALL","appId":"app-1","locationId":"UNKNOWN_LOC","companyId":"company-1","timestamp":"2026-08-17T09:06:59.366Z","webhookId":"evt-install-unknown"}`)
	headers := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body),
	}

	err := svc.ProcessWebhook(context.Background(), "highlevel", headers, body)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.NotFound)
	}

	// No config should be created.
	if len(configRepo.configs) != 0 {
		t.Fatalf("expected 0 configs, got %d", len(configRepo.configs))
	}
}

func TestProcessWebhook_NonInstallRequiresConfig(t *testing.T) {
	t.Parallel()

	// Non-INSTALL events must preserve the existing behavior: resolve via the
	// payment_provider_configs table. If the config does not exist, the event
	// fails with ErrIntegrationNotFound.
	integrationRepo := newMockWebhookIntegrationRepo()
	configRepo := newMockPaymentProviderConfigRepo()
	dispatcher := &mockWebhookDispatcher{}

	publicKeyPEM, priv := testWebhookEd25519KeyPair(t)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", publicKeyPEM, nil, zerolog.Nop()))

	svc := NewService(
		integrationRepo,
		newMockWebhookClientRepo(),
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		newMockWebhookPlatformRepo(),
		configRepo,
		registry,
		dispatcher,
		zerolog.Nop(),
	)

	// A non-INSTALL event (e.g. oauth.revoked) with a locationId but no config.
	body := []byte(`{"type":"oauth.revoked","appId":"app-1","locationId":"GHL_LOCATION_A","companyId":"company-1","timestamp":"2026-08-17T09:06:59.366Z","webhookId":"evt-revoked-1"}`)
	headers := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body),
	}

	err := svc.ProcessWebhook(context.Background(), "highlevel", headers, body)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status code = %s, want %s (non-INSTALL requires config)", status.Code(err), codes.NotFound)
	}

	// The dispatcher must NOT have been called.
	if len(dispatcher.events) != 0 {
		t.Fatalf("expected 0 dispatched events, got %d", len(dispatcher.events))
	}
}

func TestProcessWebhook_InstallProvisionsTenantAndConfig(t *testing.T) {
	t.Parallel()

	// A fresh INSTALL for an unmapped GHL location must provision the RVPay
	// tenant (client + integration) and then create the payment provider
	// config. No platform records are created; the HighLevel platform already
	// exists and is resolved by slug.
	integrationRepo := newMockWebhookIntegrationRepo()
	clientRepo := newMockWebhookClientRepo()
	configRepo := newMockPaymentProviderConfigRepo()
	platformRepo := newMockWebhookPlatformRepo()

	platformID := uuid.New()
	platformRepo.platforms[platformID.String()] = sqlc.Platform{ID: platformID, Name: "HighLevel", Slug: "highlevel", Enabled: true}

	dispatcher := providers.NewHighLevelWebhookDispatcher(
		&testWebhookLogger{},
		integrationRepo,
		configRepo,
		providers.ProviderConfigSettings{Name: "RVPay", QueryURL: "https://api.example.com/query", PaymentsURL: "https://checkout.example.com/pay"},
	)

	publicKeyPEM, priv := testWebhookEd25519KeyPair(t)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", publicKeyPEM, nil, zerolog.Nop()))

	svc := NewService(
		integrationRepo,
		clientRepo,
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		platformRepo,
		configRepo,
		registry,
		dispatcher,
		zerolog.Nop(),
	)

	body := []byte(`{"type":"INSTALL","appId":"app-1","locationId":"GHL_LOC_NEW","companyId":"company-1","timestamp":"2026-08-17T09:06:59.366Z","webhookId":"evt-install-new"}`)
	headers := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body),
	}

	err := svc.ProcessWebhook(context.Background(), "highlevel", headers, body)
	if err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}

	// A client and an integration must have been provisioned.
	if len(clientRepo.clients) != 1 {
		t.Fatalf("expected 1 provisioned client, got %d", len(clientRepo.clients))
	}
	if len(integrationRepo.integrations) != 1 {
		t.Fatalf("expected 1 provisioned integration, got %d", len(integrationRepo.integrations))
	}
	// The integration must map to the GHL locationId via external_account_id.
	for _, i := range integrationRepo.integrations {
		if i.ExternalAccountID != "GHL_LOC_NEW" {
			t.Fatalf("integration external_account_id = %q, want GHL_LOC_NEW", i.ExternalAccountID)
		}
	}
	// The payment provider config must be created for the provisioned integration.
	if len(configRepo.configs) != 1 {
		t.Fatalf("expected 1 provider config, got %d", len(configRepo.configs))
	}
}

func TestProcessWebhook_InstallRepeatedReusesRecords(t *testing.T) {
	t.Parallel()

	// A second INSTALL for the same GHL location must reuse the existing
	// client, integration, and payment provider config rather than duplicating
	// them.
	integrationRepo := newMockWebhookIntegrationRepo()
	clientRepo := newMockWebhookClientRepo()
	configRepo := newMockPaymentProviderConfigRepo()
	platformRepo := newMockWebhookPlatformRepo()

	platformID := uuid.New()
	platformRepo.platforms[platformID.String()] = sqlc.Platform{ID: platformID, Name: "HighLevel", Slug: "highlevel", Enabled: true}

	dispatcher := providers.NewHighLevelWebhookDispatcher(
		&testWebhookLogger{},
		integrationRepo,
		configRepo,
		providers.ProviderConfigSettings{Name: "RVPay", QueryURL: "https://api.example.com/query", PaymentsURL: "https://checkout.example.com/pay"},
	)

	publicKeyPEM, priv := testWebhookEd25519KeyPair(t)
	registry := providers.NewProviderRegistry()
	registry.Register(providers.NewHighLevelProvider("test-client", "test-secret", "https://example.com/callback", publicKeyPEM, nil, zerolog.Nop()))

	svc := NewService(
		integrationRepo,
		clientRepo,
		newMockWebhookRepo(),
		newMockWebhookEventRepo(),
		platformRepo,
		configRepo,
		registry,
		dispatcher,
		zerolog.Nop(),
	)

	// First INSTALL provisions the tenant.
	body := []byte(`{"type":"INSTALL","appId":"app-1","locationId":"GHL_LOC_A","companyId":"company-1","timestamp":"2026-08-17T09:06:59.366Z","webhookId":"evt-install-1"}`)
	headers := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body),
	}
	if err := svc.ProcessWebhook(context.Background(), "highlevel", headers, body); err != nil {
		t.Fatalf("first INSTALL failed: %v", err)
	}

	// A fresh (non-duplicate webhookId) INSTALL for the same location reuses
	// the tenant records.
	body2 := []byte(`{"type":"INSTALL","appId":"app-1","locationId":"GHL_LOC_A","companyId":"company-1","timestamp":"2026-08-17T09:07:01.000Z","webhookId":"evt-install-2"}`)
	headers2 := map[string]string{
		"X-GHL-Signature": signWebhookBody(t, priv, body2),
	}
	if err := svc.ProcessWebhook(context.Background(), "highlevel", headers2, body2); err != nil {
		t.Fatalf("second INSTALL failed: %v", err)
	}

	if len(clientRepo.clients) != 1 {
		t.Fatalf("expected 1 client (reused), got %d", len(clientRepo.clients))
	}
	if len(integrationRepo.integrations) != 1 {
		t.Fatalf("expected 1 integration (reused), got %d", len(integrationRepo.integrations))
	}
	if len(configRepo.configs) != 1 {
		t.Fatalf("expected 1 provider config (reused), got %d", len(configRepo.configs))
	}
}

// testWebhookLogger is a no-op Logger for dispatcher tests.
type testWebhookLogger struct{}

func (l *testWebhookLogger) Info(msg string, args ...interface{}) {}

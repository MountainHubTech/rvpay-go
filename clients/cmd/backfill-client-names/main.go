// Command backfill-client-names corrects existing RVPay client records whose
// Admin Dashboard account name is still an internal identifier
// ("highlevel-<locationId>") by resolving the authoritative HighLevel
// location/sub-account name and persisting it in clients.display_name.
//
// It is the "existing data" half of the account-name correction; new installs
// are corrected by the OAuth callback / provider reconciliation, which perform
// the same enrichment inline.
//
// Usage (from the clients service directory):
//
//	go run ./cmd/backfill-client-names             # correct every candidate
//	go run ./cmd/backfill-client-names -dry-run    # report candidates only
//
// Configuration is the normal Clients-service environment (.env / ECS task
// environment: DB_*, HIGHLEVEL_*). No migration is executed and no scope,
// secret or URL is hard-coded. The command is idempotent and safe to re-run:
// already-corrected clients leave the candidate set, a valid existing name is
// never overwritten, and every uncorrected record is reported with the reason
// (missing OAuth token, missing locations.readonly scope → 401/403, 404, 429,
// 5xx, timeout, empty name).
//
// Exit status: 0 when every candidate was corrected (or there was nothing to
// do); 1 when the run could not start or when records remain uncorrected — the
// remaining records and reasons are always reported first.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/MountainHubTech/rvpay-go/clients/backfill"
	"github.com/MountainHubTech/rvpay-go/clients/config"
	"github.com/MountainHubTech/rvpay-go/clients/db/repo"
	"github.com/MountainHubTech/rvpay-go/clients/oauth"
	"github.com/MountainHubTech/rvpay-go/clients/providers"
	commondatabase "github.com/MountainHubTech/rvpay-go/shared/database"
	commonlogger "github.com/MountainHubTech/rvpay-go/shared/logger"
	"github.com/rs/zerolog"
)

func main() {
	logger, err := commonlogger.New("", os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	dryRun := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-dry-run", "--dry-run":
			dryRun = true
		default:
			fmt.Fprintf(os.Stderr, "unknown argument %q (supported: -dry-run)\n", arg)
			os.Exit(1)
		}
	}

	report, err := run(context.Background(), logger, dryRun)
	if err != nil {
		logger.Err(err).Msg("client display name backfill failed to run")
		os.Exit(1)
	}

	logger.Info().
		Bool("dry_run", dryRun).
		Int("candidates", report.Candidates).
		Int("updated", report.Updated).
		Int("uncorrected", report.Failed).
		Msg("client display name backfill finished")

	if report.Failed > 0 && !dryRun {
		// Uncorrected records are a genuine operator action item (usually:
		// reinstall the location so its token carries locations.readonly), so
		// the command reports them as a non-zero exit status.
		os.Exit(1)
	}
}

// run wires the existing Clients-service dependencies and executes the
// backfill. It reuses the same repository/provider/OAuth wiring as the gRPC
// service so the correction behaves identically to the inline install and
// reconciliation enrichment path.
func run(ctx context.Context, logger zerolog.Logger, dryRun bool) (backfill.Report, error) {
	logger.Info().Bool("dry_run", dryRun).Msg("starting client display name backfill")

	cfg := config.Config{}
	if err := cfg.LoadConfig(); err != nil {
		logger.Err(err).Msg("failed to load config")
		return backfill.Report{}, err
	}

	logger, err := commonlogger.New(cfg.LogLevel, os.Stderr)
	if err != nil {
		return backfill.Report{}, fmt.Errorf("failed to parse log level: %w", err)
	}

	dbConnectionURL := commondatabase.PostgresURL(cfg.DB.DBUser, cfg.DB.DBPassword, cfg.DB.DBPort, cfg.DB.DBHost, cfg.DB.DBName, cfg.DB.TLSDisabled)
	db, err := commondatabase.Connect(ctx, dbConnectionURL)
	if err != nil {
		return backfill.Report{}, err
	}
	defer db.Close()
	logger.Info().Msg("connected to database for client display name backfill")

	// Migrations are never executed from this command: the schema (including
	// clients.display_name) is owned by the deployment migration pipeline.
	dbQuerier := repo.NewClientsRepo(db).Do()
	clientRepo := repo.NewClientRepo(dbQuerier)
	platformRepo := repo.NewPlatformRepo(dbQuerier)
	integrationRepo := repo.NewIntegrationRepo(dbQuerier)
	oauthTokenRepo := repo.NewOAuthTokenRepo(dbQuerier)
	oauthStateRepo := repo.NewOAuthStateRepo(dbQuerier)
	paymentProviderConfigRepo := repo.NewPaymentProviderConfigRepo(dbQuerier)

	// The HighLevel API base URL comes from configuration
	// (HIGHLEVEL_API_BASE_URL); it is never hard-coded.
	providerRegistry := providers.NewProviderRegistry()
	highLevelPaymentProvider := providers.NewHighLevelPaymentProviderClient(cfg.HighLevel.APIBaseURL, nil)
	highLevelProvider := providers.NewHighLevelProvider(cfg.HighLevel.ClientID, cfg.HighLevel.ClientSecret, cfg.HighLevel.RedirectURI, cfg.HighLevel.WebhookPublicKey, highLevelPaymentProvider, logger)
	providerRegistry.Register(highLevelProvider)

	// The OAuth service already owns the token handling (stored token,
	// refresh-once when expired) and the location lookup, so the backfill
	// reuses it instead of duplicating that lifecycle.
	oauthService := oauth.NewService(
		integrationRepo,
		oauthTokenRepo,
		clientRepo,
		platformRepo,
		oauthStateRepo,
		paymentProviderConfigRepo,
		providerRegistry,
		cfg.HighLevel.RedirectURI,
		oauth.ProviderConfigSettings{
			Name:               cfg.HighLevel.ProviderName,
			Description:        cfg.HighLevel.ProviderDescription,
			ImageURL:           cfg.HighLevel.ProviderImageURL,
			PaymentsURL:        cfg.HighLevel.PaymentURL,
			QueryURL:           cfg.HighLevel.QueryURL,
			LiveAPIKey:         cfg.HighLevel.LiveAPIKey,
			LivePublishableKey: cfg.HighLevel.LivePublishableKey,
			TestAPIKey:         cfg.HighLevel.TestAPIKey,
			TestPublishableKey: cfg.HighLevel.TestPublishableKey,
		},
		logger,
	)

	runner := backfill.NewClientNamesBackfill(clientRepo, oauthService, logger)
	return runner.Run(ctx, dryRun)
}

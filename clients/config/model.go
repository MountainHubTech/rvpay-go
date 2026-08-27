package config

import (
	"errors"
	"fmt"
	"os"

	// "strconv"

	"github.com/ardanlabs/conf/v3"
	"github.com/joho/godotenv"
)

type Config struct {
	LogLevel string `conf:"env:LOG_LEVEL,default:info"`

	ListenPort int `conf:"env:LISTEN_PORT,required"`

	DB DBConfig

	MigrationPath string `conf:"env:MIGRATION_PATH,required"`

	RunMigrations bool `conf:"env:RUN_MIGRATIONS,default:true"`

	HighLevel HighLevelConfig
}

// DBConfig holds database configuration.
type DBConfig struct {
	DBHost string `conf:"env:DB_HOST,required"`

	DBPort int `conf:"env:DB_PORT,required"`

	DBName string `conf:"env:DB_NAME,required"`

	DBUser string `conf:"env:DB_USER,required"`

	DBPassword string `conf:"env:DB_PASSWORD,required,mask"`

	TLSDisabled bool `conf:"env:DB_TLS_DISABLED"`
}

// HighLevelConfig holds HighLevel provider configuration.
type HighLevelConfig struct {
	ClientID string `conf:"env:HIGHLEVEL_CLIENT_ID,required"`

	ClientSecret string `conf:"env:HIGHLEVEL_CLIENT_SECRET,required,mask"`

	RedirectURI string `conf:"env:HIGHLEVEL_REDIRECT_URI,required"`

	// WebhookPublicKey is the PEM-encoded Ed25519 public key used to verify
	// HighLevel webhook signatures (HIGHLEVEL_WEBHOOK_PUBLIC_KEY). It is
	// public cryptographic material, not a private credential.
	WebhookPublicKey string `conf:"env:HIGHLEVEL_WEBHOOK_PUBLIC_KEY,required"`

	// PaymentURL is the frontend checkout URL supplied to HighLevel as the
	// payment provider's paymentsUrl (HIGHLEVEL_PAYMENT_URL). It is
	// configuration, never hard-coded.
	PaymentURL string `conf:"env:HIGHLEVEL_PAYMENT_URL,required"`

	// QueryURL is the backend query URL supplied to HighLevel as the payment
	// provider's queryUrl (HIGHLEVEL_QUERY_URL). It is configuration, never
	// hard-coded.
	QueryURL string `conf:"env:HIGHLEVEL_QUERY_URL,required"`

	// ProviderName is the display name of the payment provider
	// (HIGHLEVEL_PROVIDER_NAME).
	ProviderName string `conf:"env:HIGHLEVEL_PROVIDER_NAME,required"`

	// ProviderDescription is the description of the payment provider
	// (HIGHLEVEL_PROVIDER_DESCRIPTION).
	ProviderDescription string `conf:"env:HIGHLEVEL_PROVIDER_DESCRIPTION,required"`

	// ProviderImageURL is the image URL of the payment provider
	// (HIGHLEVEL_PROVIDER_IMAGE_URL).
	ProviderImageURL string `conf:"env:HIGHLEVEL_PROVIDER_IMAGE_URL,required"`

	// APIBaseURL is the HighLevel API base URL used for outbound Custom
	// Payment Provider calls (HIGHLEVEL_API_BASE_URL). It is configuration,
	// never hard-coded.
	APIBaseURL string `conf:"env:HIGHLEVEL_API_BASE_URL,required"`

	// PublicBaseURL is the publicly reachable base URL of the deployed
	// backend (PUBLIC_BASE_URL). It is used to derive the backend query URL
	// for the payment provider configuration. It is configuration, never
	// hard-coded.
	PublicBaseURL string `conf:"env:PUBLIC_BASE_URL,required"`
}


// LoadConfig reads configuration from file or environment variables.
func (c *Config) LoadConfig() error {
	if _, err := os.Stat(".env"); err == nil {
		err = godotenv.Load()
		if err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}
	}

	_, err := conf.Parse("", c)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			return err
		}

		return err
	}

	return nil
}

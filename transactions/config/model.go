package model

import (
	"errors"
	"fmt"
	"os"

	"github.com/ardanlabs/conf/v3"
	"github.com/joho/godotenv"
)

type Config struct {
	LogLevel string `conf:"env:LOG_LEVEL,default:info"`

	ListenPort    string `conf:"env:LISTEN_PORT,required"`
	MigrationPath string `conf:"env:MIGRATION_PATH,required"`

	RunMigrations bool `conf:"env:RUN_MIGRATIONS,default:true"`

	APIURL string `conf:"env:PAWAPAY_API_URL"`
	APIKey string `conf:"env:PAWAPAY_API_KEY"`

	// CORSAllowedOrigins is the comma-separated allowlist of browser origins
	// permitted to call the HTTP gateway cross-origin. The defaults are the
	// RVPay admin dashboard (which loads the payment checkout in an iframe
	// and calls the public deposit endpoint from the browser) and the local
	// development dashboard. Override via HTTP_CORS_ALLOWED_ORIGINS.
	// ClientsGrpcAddr is the gRPC address of the Clients service
	// (CLIENTS_GRPC_ADDR). The admin middleware delegates access-token
	// validation to the Clients service, which owns the users/access_tokens
	// state. It is configuration, never hard-coded.
	ClientsGrpcAddr string `conf:"env:CLIENTS_GRPC_ADDR"`

	CORSAllowedOrigins string `conf:"env:HTTP_CORS_ALLOWED_ORIGINS,default:https://admindashboard.rvpay.xyz,http://localhost:3000"`

	DB DBConfig
}

// DBConfig holds database configuration.
type DBConfig struct {
	DBUser     string `conf:"env:DB_USER,required"`
	DBPassword string `conf:"env:DB_PASSWORD,required"`
	DBHost     string `conf:"env:DB_HOST,required"`

	DBPort      uint   `conf:"env:DB_PORT,required"`
	DBName      string `conf:"env:DB_NAME,required"`
	TLSDisabled bool   `conf:"env:DB_TLS_DISABLED,default:false"`
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

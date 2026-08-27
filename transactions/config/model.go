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

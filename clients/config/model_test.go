package config

import (
	"os"
	"testing"
)

// envKeys lists every environment variable the Config struct reads.
var envKeys = []string{
	"LOG_LEVEL",
	"LISTEN_PORT",
	"RUN_MIGRATIONS",
	"MIGRATION_PATH",
	"DB_HOST",
	"DB_PORT",
	"DB_NAME",
	"DB_USER",
	"DB_PASSWORD",
	"DB_TLS_DISABLED",
	"HIGHLEVEL_CLIENT_ID",
	"HIGHLEVEL_CLIENT_SECRET",
	"HIGHLEVEL_REDIRECT_URL",
	"HIGHLEVEL_WEBHOOK_PUBLIC_KEY",
	"HIGHLEVEL_PAYMENT_URL",
	"HIGHLEVEL_QUERY_URL",
	"HIGHLEVEL_PROVIDER_NAME",
	"HIGHLEVEL_PROVIDER_DESCRIPTION",
	"HIGHLEVEL_PROVIDER_IMAGE_URL",
	"HIGHLEVEL_API_BASE_URL",
	"PUBLIC_BASE_URL",
}

func unsetAllEnv() {
	for _, k := range envKeys {
		os.Unsetenv(k)
	}
}

// setRequiredEnv populates every env var that is required by Config with a
// baseline value and unsets the rest. It returns a cleanup that clears all
// config env vars. Optional vars (LOG_LEVEL, RUN_MIGRATIONS,
// DB_TLS_DISABLED) may be overridden by the caller.
func setRequiredEnv() func() {
	unsetAllEnv()

	values := map[string]string{
		"LISTEN_PORT":                    "50051",
		"MIGRATION_PATH":                 "db/migrations",
		"DB_HOST":                        "localhost",
		"DB_PORT":                        "5432",
		"DB_NAME":                        "rvpay",
		"DB_USER":                        "postgres",
		"DB_PASSWORD":                    "postgres",
		"HIGHLEVEL_CLIENT_ID":            "test-client-id",
		"HIGHLEVEL_CLIENT_SECRET":        "test-client-secret",
		"HIGHLEVEL_REDIRECT_URL":         "https://example.com/callback",
		"HIGHLEVEL_WEBHOOK_PUBLIC_KEY":   "test-webhook-public-key",
		"HIGHLEVEL_PAYMENT_URL":          "https://example.com/payment",
		"HIGHLEVEL_QUERY_URL":            "https://example.com/query",
		"HIGHLEVEL_PROVIDER_NAME":        "RVPay",
		"HIGHLEVEL_PROVIDER_DESCRIPTION": "RVPay payment provider",
		"HIGHLEVEL_PROVIDER_IMAGE_URL":   "https://example.com/logo.jpg",
		"HIGHLEVEL_API_BASE_URL":         "https://services.leadconnectorhq.com",
		"PUBLIC_BASE_URL":                "https://example.com",
	}
	for k, v := range values {
		os.Setenv(k, v)
	}

	return unsetAllEnv
}

func TestLoadConfigDefaults(t *testing.T) {
	// Env-based tests share the process environment, so they must not run in
	// parallel.
	cleanup := setRequiredEnv()
	defer cleanup()
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("RUN_MIGRATIONS")
	os.Unsetenv("DB_TLS_DISABLED")

	cfg := Config{}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %s, want info (default)", cfg.LogLevel)
	}
	if cfg.ListenPort != 50051 {
		t.Fatalf("ListenPort = %d, want 50051", cfg.ListenPort)
	}
	if !cfg.RunMigrations {
		t.Fatal("RunMigrations should default to true")
	}
	if cfg.MigrationPath != "db/migrations" {
		t.Fatalf("MigrationPath = %s, want db/migrations", cfg.MigrationPath)
	}
	if cfg.DB.DBHost != "localhost" {
		t.Fatalf("DBHost = %s, want localhost", cfg.DB.DBHost)
	}
	if cfg.DB.DBPort != 5432 {
		t.Fatalf("DBPort = %d, want 5432", cfg.DB.DBPort)
	}
	if cfg.DB.DBName != "rvpay" {
		t.Fatalf("DBName = %s, want rvpay", cfg.DB.DBName)
	}
	if cfg.DB.DBUser != "postgres" {
		t.Fatalf("DBUser = %s, want postgres", cfg.DB.DBUser)
	}
	if cfg.DB.DBPassword != "postgres" {
		t.Fatalf("DBPassword = %s, want postgres", cfg.DB.DBPassword)
	}
	// DB_TLS_DISABLED has no default in the model, so it defaults to false.
	if cfg.DB.TLSDisabled {
		t.Fatal("TLSDisabled should default to false (no default in config)")
	}
}

func TestLoadConfigEnvironmentOverrides(t *testing.T) {
	cleanup := setRequiredEnv()
	defer cleanup()

	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LISTEN_PORT", "60000")
	os.Setenv("RUN_MIGRATIONS", "false")
	os.Setenv("MIGRATION_PATH", "test/migrations")
	os.Setenv("DB_HOST", "test-host")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_NAME", "test-db")
	os.Setenv("DB_USER", "test-user")
	os.Setenv("DB_PASSWORD", "test-pass")
	os.Setenv("DB_TLS_DISABLED", "true")
	os.Setenv("HIGHLEVEL_CLIENT_ID", "override-client-id")
	os.Setenv("HIGHLEVEL_REDIRECT_URL", "https://override.example.com/callback")

	cfg := Config{}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %s, want debug", cfg.LogLevel)
	}
	if cfg.ListenPort != 60000 {
		t.Fatalf("ListenPort = %d, want 60000", cfg.ListenPort)
	}
	if cfg.RunMigrations {
		t.Fatal("RunMigrations should be false")
	}
	if cfg.MigrationPath != "test/migrations" {
		t.Fatalf("MigrationPath = %s, want test/migrations", cfg.MigrationPath)
	}
	if cfg.DB.DBHost != "test-host" {
		t.Fatalf("DBHost = %s, want test-host", cfg.DB.DBHost)
	}
	if cfg.DB.DBPort != 5433 {
		t.Fatalf("DBPort = %d, want 5433", cfg.DB.DBPort)
	}
	if cfg.DB.DBName != "test-db" {
		t.Fatalf("DBName = %s, want test-db", cfg.DB.DBName)
	}
	if cfg.DB.DBUser != "test-user" {
		t.Fatalf("DBUser = %s, want test-user", cfg.DB.DBUser)
	}
	if cfg.DB.DBPassword != "test-pass" {
		t.Fatalf("DBPassword = %s, want test-pass", cfg.DB.DBPassword)
	}
	if !cfg.DB.TLSDisabled {
		t.Fatal("TLSDisabled should be true")
	}
	if cfg.HighLevel.ClientID != "override-client-id" {
		t.Fatalf("HighLevel.ClientID = %s, want override-client-id", cfg.HighLevel.ClientID)
	}
	if cfg.HighLevel.RedirectURI != "https://override.example.com/callback" {
		t.Fatalf("HighLevel.RedirectURI = %s, want override URL", cfg.HighLevel.RedirectURI)
	}
}

func TestLoadConfigMissingRequired(t *testing.T) {
	unsetAllEnv()

	cfg := Config{}
	if err := cfg.LoadConfig(); err == nil {
		t.Fatal("LoadConfig should fail when required environment variables are missing")
	}
}

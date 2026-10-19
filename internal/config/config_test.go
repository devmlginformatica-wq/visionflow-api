package config

import (
	"net/url"
	"testing"
)

func TestDSNEscapesCredentialsAndUsesConfiguredSSLMode(t *testing.T) {
	config := Config{
		DBHost:     "db.internal",
		DBPort:     "5432",
		DBUser:     "api user",
		DBPassword: "p@ss:/?#%word",
		DBName:     "vision flow",
		DBSSLMode:  "require",
	}

	parsed, err := url.Parse(config.DSN())
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	password, hasPassword := parsed.User.Password()
	if parsed.User.Username() != config.DBUser || !hasPassword || password != config.DBPassword {
		t.Fatal("DSN did not preserve escaped credentials")
	}
	if parsed.Host != "db.internal:5432" || parsed.Path != "/vision flow" {
		t.Fatalf("DSN host/path = %q/%q, want db.internal:5432 and /vision flow", parsed.Host, parsed.Path)
	}
	if got := parsed.Query().Get("sslmode"); got != "require" {
		t.Fatalf("DSN sslmode = %q, want require", got)
	}
}

func TestLoadSQLConsoleIsDisabledByDefault(t *testing.T) {
	t.Setenv("DB_USER", "test")
	t.Setenv("DB_PASSWORD", "test")
	t.Setenv("DB_NAME", "test")
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("SQL_CONSOLE_ENABLED", "")
	t.Setenv("APP_ENV", "")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.SQLConsoleEnabled {
		t.Fatal("SQLConsoleEnabled defaults to true, want disabled")
	}
	if config.AppEnv != "production" {
		t.Fatalf("AppEnv = %q, want production", config.AppEnv)
	}
}

func TestLoadRejectsInvalidSQLConsoleFlag(t *testing.T) {
	t.Setenv("DB_USER", "test")
	t.Setenv("DB_PASSWORD", "test")
	t.Setenv("DB_NAME", "test")
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("SQL_CONSOLE_ENABLED", "enabled")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an invalid SQL_CONSOLE_ENABLED value")
	}
}

func TestSQLConsoleFlagParse(t *testing.T) {
	t.Setenv("DB_USER", "test")
	t.Setenv("DB_PASSWORD", "test")
	t.Setenv("DB_NAME", "test")
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("SQL_CONSOLE_ENABLED", "true")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !config.SQLConsoleEnabled {
		t.Fatal("SQLConsoleEnabled = false, want true")
	}
}

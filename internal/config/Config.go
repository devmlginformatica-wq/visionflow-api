package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config agrega toda a configuração da aplicação.
type Config struct {
	Port              string
	AppEnv            string
	SQLConsoleEnabled bool

	// PostgreSQL
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	DBPoolMaxConns    int32
	DBPoolMinConns    int32
	DBPoolMaxLifetime time.Duration

	// JWT
	JWTSecret     string
	JWTExpiration time.Duration

	// CORS
	CORSAllowedOrigins []string
}

// Load carrega .env (se existir) e variáveis de ambiente.
// O .env é opcional em produção (usa env vars do sistema).
func Load() (*Config, error) {
	// .env é opcional — se não existir, segue com env vars do sistema.
	_ = godotenv.Load()

	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		AppEnv:             getEnv("APP_ENV", "production"),
		SQLConsoleEnabled:  false,
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", ""),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", ""),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		DBPoolMaxConns:     int32(getEnvInt("DB_POOL_MAX_CONNS", 10)),
		DBPoolMinConns:     int32(getEnvInt("DB_POOL_MIN_CONNS", 2)),
		DBPoolMaxLifetime:  getEnvDuration("DB_POOL_MAX_CONN_LIFETIME", 30*time.Minute),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTExpiration:      getEnvDuration("JWT_EXPIRATION", 8*time.Hour),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "")),
	}
	if value := os.Getenv("SQL_CONSOLE_ENABLED"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("SQL_CONSOLE_ENABLED deve ser true ou false")
		}
		cfg.SQLConsoleEnabled = enabled
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var missing []string
	if c.DBUser == "" {
		missing = append(missing, "DB_USER")
	}
	if c.DBPassword == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if c.DBName == "" {
		missing = append(missing, "DB_NAME")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("variáveis de ambiente obrigatórias ausentes: %s",
			strings.Join(missing, ", "))
	}
	return nil
}

// DSN retorna a string de conexão do Postgres (formato URL).
func (c *Config) DSN() string {
	connectionURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   net.JoinHostPort(c.DBHost, c.DBPort),
		Path:   "/" + c.DBName,
	}
	query := connectionURL.Query()
	query.Set("sslmode", c.DBSSLMode)
	connectionURL.RawQuery = query.Encode()
	return connectionURL.String()
}

// ─── Helpers ─────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

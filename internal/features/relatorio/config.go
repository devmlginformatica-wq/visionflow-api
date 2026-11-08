package relatorio

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLLMTimeoutSeconds = 30
	defaultReportLimit       = 1000
)

type LLMConfig struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
	Timeout  time.Duration
}

type DBIAConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

type RelatorioConfig struct {
	LimitDefault int
	LLM          LLMConfig
	DBIA         DBIAConfig
}

func loadRelatorioConfig() (RelatorioConfig, error) {
	limitDefault, err := parseLimitDefault()
	if err != nil {
		return RelatorioConfig{}, err
	}

	cfg := RelatorioConfig{
		LimitDefault: limitDefault,
		LLM: LLMConfig{
			Provider: strings.TrimSpace(os.Getenv("LLM_PROVIDER")),
			Model:    strings.TrimSpace(os.Getenv("LLM_MODEL")),
			BaseURL:  strings.TrimSpace(os.Getenv("LLM_BASE_URL")),
			APIKey:   strings.TrimSpace(os.Getenv("LLM_API_KEY")),
			Timeout:  parseTimeoutSeconds(os.Getenv("LLM_TIMEOUT_SECONDS"), defaultLLMTimeoutSeconds),
		},
		DBIA: DBIAConfig{
			Host:     strings.TrimSpace(os.Getenv("DB_IA_HOST")),
			Port:     strings.TrimSpace(os.Getenv("DB_IA_PORT")),
			Name:     strings.TrimSpace(os.Getenv("DB_IA_NAME")),
			User:     strings.TrimSpace(os.Getenv("DB_IA_USER")),
			Password: strings.TrimSpace(os.Getenv("DB_IA_PASSWORD")),
			SSLMode:  strings.TrimSpace(os.Getenv("DB_IA_SSLMODE")),
		},
	}
	if cfg.DBIA.SSLMode == "" {
		cfg.DBIA.SSLMode = "disable"
	}
	return cfg, nil
}

func parseLimitDefault() (int, error) {
	value := strings.TrimSpace(os.Getenv("RELATORIO_IA_LIMIT_DEFAULT"))
	if value == "" {
		return defaultReportLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("RELATORIO_IA_LIMIT_DEFAULT deve ser um inteiro válido: %w", err)
	}
	if limit < 0 {
		return 0, fmt.Errorf("RELATORIO_IA_LIMIT_DEFAULT não pode ser negativo")
	}
	return limit, nil
}

func parseTimeoutSeconds(raw string, fallback int) time.Duration {
	if strings.TrimSpace(raw) == "" {
		return time.Duration(fallback) * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return time.Duration(fallback) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func (c LLMConfig) ProviderName() string {
	return strings.ToLower(strings.TrimSpace(c.Provider))
}

func (c DBIAConfig) IsConfigured() bool {
	return c.Host != "" && c.Port != "" && c.Name != "" && c.User != "" && c.Password != "" && c.SSLMode != ""
}

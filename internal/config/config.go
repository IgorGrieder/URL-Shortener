package config

import (
	"fmt"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Server    ServerConfig
	Postgres  PostgresConfig
	Shortener ShortenerConfig
	Security  SecurityConfig
	OTel      OTelConfig
}

type AppConfig struct {
	Name     string
	Version  string
	Env      string
	LogLevel string
}

type ServerConfig struct {
	Port string
	Host string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type ShortenerConfig struct {
	BaseURL        string
	SlugLength     int
	RedirectStatus int // 301 or 302
}

type SecurityConfig struct {
	APIKeys []string
}

type OTelConfig struct {
	Endpoint string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	cfg := &Config{
		App: AppConfig{
			Name:     GetEnv("APP_NAME", "encurtador-url"),
			Version:  GetEnv("APP_VERSION", "0.1.0"),
			Env:      GetEnv("APP_ENV", "development"),
			LogLevel: GetEnv("LOG_LEVEL", "info"),
		},
		Server: ServerConfig{
			Port: GetEnv("APP_PORT", "8080"),
			Host: GetEnv("APP_HOST", "localhost"),
		},
		Postgres: PostgresConfig{
			Host:     GetEnv("DB_HOST", "localhost"),
			Port:     GetEnv("DB_PORT", "5432"),
			User:     GetEnv("DB_USER", "postgres"),
			Password: GetEnv("DB_PASSWORD", "postgres"),
			DBName:   GetEnv("DB_NAME", "encurtador"),
			SSLMode:  GetEnv("DB_SSL_MODE", "disable"),
		},
		Shortener: ShortenerConfig{
			BaseURL:        GetEnv("SHORTENER_BASE_URL", "http://localhost:8080"),
			SlugLength:     GetEnvInt("SLUG_LENGTH", 6),
			RedirectStatus: GetEnvInt("REDIRECT_STATUS", 302),
		},
		Security: SecurityConfig{
			APIKeys: SplitCSV(GetEnv("API_KEYS", "")),
		},
		OTel: OTelConfig{
			Endpoint: GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		},
	}

	if cfg.Shortener.RedirectStatus != 301 && cfg.Shortener.RedirectStatus != 302 {
		return nil, fmt.Errorf("REDIRECT_STATUS must be 301 or 302 (got %d)", cfg.Shortener.RedirectStatus)
	}
	if cfg.Shortener.SlugLength < 4 || cfg.Shortener.SlugLength > 32 {
		return nil, fmt.Errorf("SLUG_LENGTH must be between 4 and 32 (got %d)", cfg.Shortener.SlugLength)
	}

	return cfg, nil
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.DBName,
		c.SSLMode,
	)
}

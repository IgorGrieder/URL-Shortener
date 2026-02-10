package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Server    ServerConfig
	MongoDB   MongoDBConfig
	Redis     RedisConfig
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

type MongoDBConfig struct {
	URI      string
	Database string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
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
	Enabled  bool
	Endpoint string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	cfg := &Config{
		App: AppConfig{
			Name:     getEnv("APP_NAME", "encurtador-url"),
			Version:  getEnv("APP_VERSION", "0.1.0"),
			Env:      getEnv("APP_ENV", "development"),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
		Server: ServerConfig{
			Port: getEnv("APP_PORT", "8080"),
			Host: getEnv("APP_HOST", "localhost"),
		},
		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://localhost:27017"),
			Database: getEnv("MONGODB_DATABASE", "encurtador"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Shortener: ShortenerConfig{
			BaseURL:        getEnv("SHORTENER_BASE_URL", "http://localhost:8080"),
			SlugLength:     getEnvInt("SLUG_LENGTH", 6),
			RedirectStatus: getEnvInt("REDIRECT_STATUS", 302),
		},
		Security: SecurityConfig{
			APIKeys: getEnvSlice("API_KEYS", nil),
		},
		OTel: OTelConfig{
			Enabled:  getEnvBool("OTEL_ENABLED", false),
			Endpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return defaultValue
	}
	return out
}

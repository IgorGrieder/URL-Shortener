package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// GetEnv returns the value of the environment variable identified by key,
// or fallback when the variable is empty or unset.
func GetEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// GetEnvInt returns the environment variable as an int, or fallback on
// missing/invalid values.
func GetEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// GetEnvDuration returns the environment variable parsed as a time.Duration,
// or fallback on missing/invalid values.
func GetEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

// SplitCSV splits a comma-separated string into a slice, trimming whitespace
// and discarding empty entries.
func SplitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// DefaultPostgresDSN builds a PostgreSQL DSN from individual DB_* environment
// variables.
func DefaultPostgresDSN() string {
	host := GetEnv("DB_HOST", "localhost")
	port := GetEnv("DB_PORT", "5432")
	user := GetEnv("DB_USER", "postgres")
	password := GetEnv("DB_PASSWORD", "postgres")
	dbName := GetEnv("DB_NAME", "encurtador")
	sslMode := GetEnv("DB_SSL_MODE", "disable")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, dbName, sslMode)
}

// DefaultWorkerID returns a worker identifier built from the hostname and PID.
// fallbackName is used when the hostname cannot be determined.
func DefaultWorkerID(fallbackName string) string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = fallbackName
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

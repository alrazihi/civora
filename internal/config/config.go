package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Audit    AuditConfig
}

type ServerConfig struct {
	Port           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	CORSOrigins    []string
	TrustedProxies []string
	RateLimit      int
	RateLimitBurst int
}

type DatabaseConfig struct {
	Driver   string
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type AuthConfig struct {
	JWTSecret  string
	JWTExpiry  time.Duration
	BCryptCost int
}

type AuditConfig struct {
	Enabled          bool
	HashChainEnabled bool
	RetentionDays    int
}

func Load() (*Config, error) {
	if os.Getenv("CIVORA_ENV") != "production" {
		_ = godotenv.Load(".env")
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:           getEnv("CIVORA_SERVER_PORT", "8080"),
			ReadTimeout:    getEnvDuration("CIVORA_SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:   getEnvDuration("CIVORA_SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:    getEnvDuration("CIVORA_SERVER_IDLE_TIMEOUT", 120*time.Second),
			CORSOrigins:    getEnvCSV("CIVORA_SERVER_CORS_ORIGINS", "http://localhost:3000"),
			TrustedProxies: getEnvList("CIVORA_SERVER_TRUSTED_PROXIES"),
			RateLimit:      getEnvInt("CIVORA_SERVER_RATE_LIMIT", 1000),
			RateLimitBurst: getEnvInt("CIVORA_SERVER_RATE_LIMIT_BURST", 200),
		},
		Database: DatabaseConfig{
			Driver:   getEnv("CIVORA_DB_DRIVER", "pgx"),
			Host:     getEnv("CIVORA_DB_HOST", "localhost"),
			Port:     getEnv("CIVORA_DB_PORT", "5432"),
			User:     getEnv("CIVORA_DB_USER", "civora"),
			Password: getEnv("CIVORA_DB_PASSWORD", "civora"),
			DBName:   getEnv("CIVORA_DB_NAME", "civora"),
			SSLMode:  getEnv("CIVORA_DB_SSLMODE", "disable"),
		},
		Auth: AuthConfig{
			JWTSecret:  getEnv("CIVORA_AUTH_JWT_SECRET", ""),
			JWTExpiry:  getEnvDuration("CIVORA_AUTH_JWT_EXPIRY", 24*time.Hour),
			BCryptCost: getEnvInt("CIVORA_AUTH_BCRYPT_COST", 12),
		},
		Audit: AuditConfig{
			Enabled:          getEnvBool("CIVORA_AUDIT_ENABLED", true),
			HashChainEnabled: getEnvBool("CIVORA_AUDIT_HASH_CHAIN", true),
			RetentionDays:    getEnvInt("CIVORA_AUDIT_RETENTION_DAYS", 2555),
		},
	}

	if len(cfg.Auth.JWTSecret) < 32 {
		return nil, fmt.Errorf("CIVORA_AUTH_JWT_SECRET must be set to a secure value of at least 32 characters")
	}

	if cfg.Auth.BCryptCost < 10 {
		return nil, fmt.Errorf("CIVORA_AUTH_BCRYPT_COST must be at least 10, got %d", cfg.Auth.BCryptCost)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
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

func getEnvList(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := parts[:0]
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getEnvCSV(key, fallback string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	return strings.Split(fallback, ",")
}

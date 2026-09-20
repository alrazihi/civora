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
	Storage  StorageConfig
	AI       AIConfig
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
	TLSCertFile    string
	TLSKeyFile     string
	ForceHTTPS     bool
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
	JWTSecret          string
	JWTExpiry          time.Duration
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	BCryptCost         int
}

type AuditConfig struct {
	Enabled          bool
	HashChainEnabled bool
	RetentionDays    int
}

type StorageConfig struct {
	Provider    string
	LocalPath   string
	MaxFileSize int64
}

type AIConfig struct {
	Enabled         bool
	Provider        string
	MaxTokens       int
	LocalEnabled    bool
	LocalBaseURL    string
	LocalModel      string
	OpenAIAPIKey    string
	OpenAIModel     string
	OpenAIBaseURL   string
	PIISanitization bool
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
			TLSCertFile:    getEnv("CIVORA_SERVER_TLS_CERT", ""),
			TLSKeyFile:     getEnv("CIVORA_SERVER_TLS_KEY", ""),
			ForceHTTPS:     getEnvBool("CIVORA_SERVER_FORCE_HTTPS", false),
		},
		Database: DatabaseConfig{
			Driver:   getEnv("CIVORA_DB_DRIVER", "pgx"),
			Host:     getEnv("CIVORA_DB_HOST", "localhost"),
			Port:     getEnv("CIVORA_DB_PORT", "5432"),
			User:     getEnv("CIVORA_DB_USER", getDefaultDBUser()),
			Password: getEnv("CIVORA_DB_PASSWORD", getDefaultDBPassword()),
			DBName:   getEnv("CIVORA_DB_NAME", getDefaultDBName()),
			SSLMode:  getEnv("CIVORA_DB_SSLMODE", getDefaultDBSSLMode()),
		},
		Auth: AuthConfig{
			JWTSecret:          getEnv("CIVORA_AUTH_JWT_SECRET", ""),
			JWTExpiry:          getEnvDuration("CIVORA_AUTH_JWT_EXPIRY", 20*time.Minute),
			AccessTokenExpiry:  getEnvDuration("CIVORA_AUTH_ACCESS_TOKEN_EXPIRY", 20*time.Minute),
			RefreshTokenExpiry: getEnvDuration("CIVORA_AUTH_REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
			BCryptCost:         getEnvInt("CIVORA_AUTH_BCRYPT_COST", 12),
		},
		Audit: AuditConfig{
			Enabled:          getEnvBool("CIVORA_AUDIT_ENABLED", true),
			HashChainEnabled: getEnvBool("CIVORA_AUDIT_HASH_CHAIN", true),
			RetentionDays:    getEnvInt("CIVORA_AUDIT_RETENTION_DAYS", 2555),
		},
		Storage: StorageConfig{
			Provider:    getEnv("CIVORA_STORAGE_PROVIDER", "local"),
			LocalPath:   getEnv("CIVORA_STORAGE_LOCAL_PATH", "./storage"),
			MaxFileSize: getEnvInt64("CIVORA_STORAGE_MAX_FILE_SIZE", 10<<20), // 10 MB
		},
		AI: AIConfig{
			Enabled:         getEnvBool("CIVORA_AI_ENABLED", false),
			Provider:        getEnv("CIVORA_AI_PROVIDER", "noop"),
			MaxTokens:       getEnvInt("CIVORA_AI_MAX_TOKENS", 4096),
			LocalEnabled:    getEnvBool("CIVORA_AI_LOCAL_ENABLED", false),
			LocalBaseURL:    getEnv("CIVORA_AI_LOCAL_BASE_URL", "http://localhost:11434/v1"),
			LocalModel:      getEnv("CIVORA_AI_LOCAL_MODEL", "llama3"),
			OpenAIAPIKey:    getEnv("CIVORA_AI_OPENAI_API_KEY", ""),
			OpenAIModel:     getEnv("CIVORA_AI_OPENAI_MODEL", "gpt-4o-mini"),
			OpenAIBaseURL:   getEnv("CIVORA_AI_OPENAI_BASE_URL", "https://api.openai.com/v1"),
			PIISanitization: getEnvBool("CIVORA_AI_PII_SANITIZATION", true),
		},
	}

	if len(cfg.Auth.JWTSecret) < 32 {
		return nil, fmt.Errorf("CIVORA_AUTH_JWT_SECRET must be set to a secure value of at least 32 characters")
	}

	if cfg.Auth.BCryptCost < 10 {
		return nil, fmt.Errorf("CIVORA_AUTH_BCRYPT_COST must be at least 10, got %d", cfg.Auth.BCryptCost)
	}

	if (cfg.Server.TLSCertFile != "") != (cfg.Server.TLSKeyFile != "") {
		return nil, fmt.Errorf("CIVORA_SERVER_TLS_CERT and CIVORA_SERVER_TLS_KEY must both be set or both be empty")
	}

	if cfg.Server.ForceHTTPS && len(cfg.Server.TrustedProxies) == 0 {
		return nil, fmt.Errorf("CIVORA_SERVER_FORCE_HTTPS requires CIVORA_SERVER_TRUSTED_PROXIES to be configured")
	}

	if os.Getenv("CIVORA_ENV") == "production" {
		if cfg.Database.User == "" {
			return nil, fmt.Errorf("CIVORA_DB_USER must be set in production")
		}
		if cfg.Database.Password == "" {
			return nil, fmt.Errorf("CIVORA_DB_PASSWORD must be set in production")
		}
		if cfg.Database.DBName == "" {
			return nil, fmt.Errorf("CIVORA_DB_NAME must be set in production")
		}
		if cfg.Auth.JWTSecret == "dev-secret-change-me-32-chars-minimum" {
			return nil, fmt.Errorf("CIVORA_AUTH_JWT_SECRET must be changed in production")
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDefaultDBUser() string {
	if os.Getenv("CIVORA_ENV") == "production" {
		return ""
	}
	return "civora"
}

func getDefaultDBPassword() string {
	if os.Getenv("CIVORA_ENV") == "production" {
		return ""
	}
	return "civora"
}

func getDefaultDBName() string {
	if os.Getenv("CIVORA_ENV") == "production" {
		return ""
	}
	return "civora"
}

func getDefaultDBSSLMode() string {
	if os.Getenv("CIVORA_ENV") == "production" {
		return "require"
	}
	return "disable"
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
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

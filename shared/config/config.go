// Package config loads service configuration from environment variables and
// optional config files using Viper. Each service embeds or extends BaseConfig.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// BaseConfig holds fields common to every Relay microservice.
type BaseConfig struct {
	// Server
	HTTPPort int    `mapstructure:"HTTP_PORT"`
	GRPCPort int    `mapstructure:"GRPC_PORT"`
	Env      string `mapstructure:"ENV"` // "development" | "staging" | "production"

	// Database
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	// Redis
	RedisURL string `mapstructure:"REDIS_URL"`

	// NATS
	NATSURL string `mapstructure:"NATS_URL"`

	// Logging
	LogLevel  string `mapstructure:"LOG_LEVEL"`
	LogPretty bool   `mapstructure:"LOG_PRETTY"`

	// JWT
	JWTSecret          string `mapstructure:"JWT_SECRET"`
	JWTAccessTokenTTL  int    `mapstructure:"JWT_ACCESS_TTL_SECONDS"`
	JWTRefreshTokenTTL int    `mapstructure:"JWT_REFRESH_TTL_SECONDS"`

	// CORS — comma-separated list of allowed origins, e.g.
	// "http://localhost:3000,https://app.relay.im".
	// Set to "*" to allow all origins (development only).
	// Leave empty to disable CORS headers entirely (production default).
	CORSAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS"`
}

// Load reads configuration from environment variables (and optionally .env).
// prefix is the service-specific env prefix, e.g. "AUTH", "MSG".
func Load[T any](prefix string) (*T, error) {
	v := viper.New()

	// Env vars take precedence
	v.SetEnvPrefix(prefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicitly bind env vars so Unmarshal picks them up.
	for _, key := range []string{
		"HTTP_PORT", "GRPC_PORT", "ENV",
		"DATABASE_URL", "REDIS_URL", "NATS_URL",
		"LOG_LEVEL", "LOG_PRETTY",
		"JWT_SECRET", "JWT_ACCESS_TTL_SECONDS", "JWT_REFRESH_TTL_SECONDS",
		"CORS_ALLOWED_ORIGINS",
		// Auth-service specific
		"SMTP_HOST", "SMTP_PORT", "SMTP_FROM", "SMTP_USERNAME", "SMTP_PASSWORD",
		"BASE_URL",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET",
		"RATE_LIMIT_AUTH_LIMIT", "RATE_LIMIT_AUTH_WINDOW_SECONDS",
		// S3/MinIO
		"S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY",
		"S3_BUCKET_FILES", "S3_BUCKET_AVATARS",
		// MinIO (file-service)
		"MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_USE_SSL",
		// Elasticsearch
		"ELASTICSEARCH_URL",
		// Rate limiting (various services)
		"RATE_LIMIT_UPLOAD_LIMIT", "RATE_LIMIT_UPLOAD_WINDOW_SECONDS",
		"RATE_LIMIT_WS_LIMIT", "RATE_LIMIT_WS_WINDOW_SECONDS",
	} {
		_ = v.BindEnv(key)
	}

	// Sensible defaults
	v.SetDefault("HTTP_PORT", 8080)
	v.SetDefault("GRPC_PORT", 9090)
	v.SetDefault("ENV", "development")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_PRETTY", false)
	v.SetDefault("JWT_ACCESS_TTL_SECONDS", 900)
	v.SetDefault("JWT_REFRESH_TTL_SECONDS", 2592000)
	v.SetDefault("NATS_URL", "nats://localhost:4222")
	v.SetDefault("REDIS_URL", "redis://localhost:6379")

	var cfg T
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}
	return &cfg, nil
}

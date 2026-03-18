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
}

// Load reads configuration from environment variables (and optionally .env).
// prefix is the service-specific env prefix, e.g. "AUTH", "MSG".
func Load[T any](prefix string) (*T, error) {
	v := viper.New()

	// Env vars take precedence
	v.SetEnvPrefix(prefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

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

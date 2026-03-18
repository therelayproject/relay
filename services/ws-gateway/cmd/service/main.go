package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/relay-im/relay/services/ws-gateway/internal/handler"
	"github.com/relay-im/relay/services/ws-gateway/internal/hub"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/logger"
	"github.com/relay-im/relay/shared/middleware"
)

// GatewayConfig extends BaseConfig with ws-gateway-specific settings.
type GatewayConfig struct {
	config.BaseConfig `mapstructure:",squash"`

	// Rate limiting for WebSocket connection upgrades.
	// RATE_LIMIT_WS_LIMIT:          max connections per window (default 30).
	// RATE_LIMIT_WS_WINDOW_SECONDS: window size in seconds     (default 60).
	RateLimitWSLimit         int `mapstructure:"RATE_LIMIT_WS_LIMIT"`
	RateLimitWSWindowSeconds int `mapstructure:"RATE_LIMIT_WS_WINDOW_SECONDS"`
}

func main() {
	cfg, err := config.Load[GatewayConfig]("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8006
	}
	if cfg.RateLimitWSLimit == 0 {
		cfg.RateLimitWSLimit = 30
	}
	if cfg.RateLimitWSWindowSeconds == 0 {
		cfg.RateLimitWSWindowSeconds = 60
	}

	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "ws-gateway",
	})

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		log.Fatal().Err(err).Str("url", cfg.NATSURL).Msg("failed to connect to nats")
	}
	defer nc.Drain() //nolint:errcheck
	log.Info().Msg("nats connected")

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdbOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Str("url", cfg.RedisURL).Msg("invalid redis URL")
	}
	rdb := redis.NewClient(rdbOpts)
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatal().Err(err).Msg("redis ping failed")
	}
	log.Info().Msg("redis connected")

	wsRateLimit := middleware.RateLimitRedis(
		rdb,
		"ws:connect",
		cfg.RateLimitWSLimit,
		time.Duration(cfg.RateLimitWSWindowSeconds)*time.Second,
	)
	log.Info().
		Int("limit", cfg.RateLimitWSLimit).
		Int("window_seconds", cfg.RateLimitWSWindowSeconds).
		Msg("ws-gateway rate limiting configured")

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		log.Fatal().Msg("JWT_SECRET is required and must not be empty")
	}

	h := hub.New(nc, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	gwHandler := handler.NewGatewayHandler(h, jwtSecret, log)
	httpHandler := handler.NewRouter(gwHandler, log, wsRateLimit)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: httpHandler,
		// WebSocket connections are long-lived; only set read/write deadlines at
		// the per-message level inside WritePump/ReadPump.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       0, // disabled — long-lived WS
	}

	go func() {
		log.Info().Str("addr", addr).Msg("ws-gateway HTTP/WebSocket listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down ws-gateway")

	cancel() // stop hub

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown error")
		os.Exit(1)
	}
	log.Info().Msg("ws-gateway stopped")
}

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
	"github.com/relay-im/relay/services/presence-service/internal/handler"
	"github.com/relay-im/relay/services/presence-service/internal/service"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/logger"
)

func main() {
	cfg, err := config.Load[config.BaseConfig]("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8007
	}

	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "presence-service",
	})

	// Redis
	rdbOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid redis URL")
	}
	rdb := redis.NewClient(rdbOpts)
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatal().Err(err).Msg("redis ping failed")
	}

	// NATS
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		log.Fatal().Err(err).Str("url", cfg.NATSURL).Msg("failed to connect to nats")
	}
	defer nc.Drain() //nolint:errcheck

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		log.Fatal().Msg("JWT_SECRET is required and must not be empty")
	}

	presenceSvc := service.NewPresenceService(rdb, nc)
	presenceHandler := handler.NewPresenceHandler(presenceSvc)
	httpHandler := handler.NewRouter(presenceHandler, jwtSecret, log)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("presence-service HTTP listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down presence-service")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown error")
		os.Exit(1)
	}
	log.Info().Msg("presence-service stopped")
}

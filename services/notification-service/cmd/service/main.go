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
	"github.com/relay-im/relay/services/notification-service/internal/handler"
	"github.com/relay-im/relay/services/notification-service/internal/service"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/db"
	"github.com/relay-im/relay/shared/logger"
)

func main() {
	cfg, err := config.Load[config.BaseConfig]("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8008
	}

	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "notification-service",
	})

	connCtx, connCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer connCancel()

	pool, err := db.New(connCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer pool.Close()

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		log.Fatal().Err(err).Str("url", cfg.NATSURL).Msg("failed to connect to nats")
	}
	defer nc.Drain() //nolint:errcheck

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		log.Fatal().Msg("JWT_SECRET is required and must not be empty")
	}

	svc := service.New(pool, nc, log)

	// Start NATS consumer for mention notifications.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.StartConsumer(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start nats consumer")
	}

	h := handler.New(svc)
	httpHandler := handler.NewRouter(h, jwtSecret, log)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("notification-service HTTP listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down notification-service")
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown error")
		os.Exit(1)
	}
	log.Info().Msg("notification-service stopped")
}

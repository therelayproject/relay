package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/relay-im/relay/services/channel-service/internal/handler"
	"github.com/relay-im/relay/services/channel-service/internal/repository"
	"github.com/relay-im/relay/services/channel-service/internal/service"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/db"
	"github.com/relay-im/relay/shared/logger"
)

func main() {
	cfg, err := config.Load[config.BaseConfig]("CHANNEL")
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "channel-service",
	})

	// ── Database ──────────────────────────────────────────────────────────────
	initCtx, initCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer initCancel()

	pool, err := db.New(initCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer pool.Close()
	log.Info().Msg("postgres connected")

	// ── NATS (best-effort) ────────────────────────────────────────────────────
	var nc *nats.Conn
	if cfg.NATSURL != "" {
		nc, err = nats.Connect(cfg.NATSURL,
			nats.Name("channel-service"),
			nats.MaxReconnects(10),
			nats.ReconnectWait(2*time.Second),
			nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
				log.Warn().Err(err).Msg("NATS disconnected")
			}),
			nats.ReconnectHandler(func(_ *nats.Conn) {
				log.Info().Msg("NATS reconnected")
			}),
		)
		if err != nil {
			log.Warn().Err(err).Msg("NATS connect failed; events will be skipped")
			nc = nil
		} else {
			log.Info().Str("url", cfg.NATSURL).Msg("NATS connected")
			defer nc.Drain()
		}
	} else {
		log.Warn().Msg("NATS_URL not set; channel events disabled")
	}

	// ── Wire dependencies ─────────────────────────────────────────────────────
	channelRepo := repository.NewChannelRepo(pool)
	channelSvc := service.NewChannelService(channelRepo, nc, log)
	channelHandler := handler.NewChannelHandler(channelSvc)

	// ── HTTP server ───────────────────────────────────────────────────────────
	if cfg.JWTSecret == "" {
		log.Fatal().Msg("JWT_SECRET must be set")
	}

	httpPort := cfg.HTTPPort
	if httpPort == 0 {
		httpPort = 8004
	}

	router := handler.NewRouter(channelHandler, cfg.JWTSecret)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", httpPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("starting channel-service")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down channel-service...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
		os.Exit(1)
	}
	log.Info().Msg("channel-service stopped")
}

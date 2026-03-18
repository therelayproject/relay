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
	"github.com/relay-im/relay/services/search-service/internal/handler"
	"github.com/relay-im/relay/services/search-service/internal/service"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/logger"
)

// ServiceConfig extends BaseConfig with search-service–specific fields.
type ServiceConfig struct {
	config.BaseConfig `mapstructure:",squash"`
	ElasticsearchURL  string `mapstructure:"ELASTICSEARCH_URL"`
}

func main() {
	cfg, err := config.Load[ServiceConfig]("SEARCH")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8009
	}
	if cfg.ElasticsearchURL == "" {
		cfg.ElasticsearchURL = "http://localhost:9200"
	}

	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "search-service",
	})

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		log.Fatal().Err(err).Str("url", cfg.NATSURL).Msg("failed to connect to nats")
	}
	defer nc.Drain() //nolint:errcheck
	log.Info().Msg("nats connected")

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		log.Fatal().Msg("JWT_SECRET is required and must not be empty")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	searchSvc := service.New(cfg.ElasticsearchURL, nc, log)

	// Ensure Elasticsearch index exists.
	if err := searchSvc.EnsureIndex(ctx); err != nil {
		log.Warn().Err(err).Msg("failed to ensure ES index (will retry on next restart)")
	}

	// Start NATS consumer to index incoming messages.
	if err := searchSvc.StartConsumer(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start NATS consumer")
	}

	searchHandler := handler.NewSearchHandler(searchSvc)
	httpHandler := handler.NewRouter(searchHandler, jwtSecret, log)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("search-service HTTP listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down search-service")

	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown error")
		os.Exit(1)
	}
	log.Info().Msg("search-service stopped")
}

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/relay-im/relay/services/file-service/internal/handler"
	"github.com/relay-im/relay/services/file-service/internal/repository"
	"github.com/relay-im/relay/services/file-service/internal/service"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/db"
	"github.com/relay-im/relay/shared/logger"
	"github.com/relay-im/relay/shared/middleware"
)

// ServiceConfig extends BaseConfig with file-service–specific fields.
type ServiceConfig struct {
	config.BaseConfig `mapstructure:",squash"`
	MinioEndpoint     string `mapstructure:"MINIO_ENDPOINT"`
	MinioAccessKey    string `mapstructure:"MINIO_ACCESS_KEY"`
	MinioSecretKey    string `mapstructure:"MINIO_SECRET_KEY"`
	MinioUseSSL       bool   `mapstructure:"MINIO_USE_SSL"`

	// Rate limiting for file upload endpoint.
	// RATE_LIMIT_UPLOAD_LIMIT:          max uploads per window (default 20).
	// RATE_LIMIT_UPLOAD_WINDOW_SECONDS: window size in seconds  (default 60).
	RateLimitUploadLimit         int `mapstructure:"RATE_LIMIT_UPLOAD_LIMIT"`
	RateLimitUploadWindowSeconds int `mapstructure:"RATE_LIMIT_UPLOAD_WINDOW_SECONDS"`
}

func main() {
	cfg, err := config.Load[ServiceConfig]("FILE")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8010
	}
	if cfg.MinioEndpoint == "" {
		cfg.MinioEndpoint = "localhost:9000"
	}
	if cfg.MinioAccessKey == "" {
		fmt.Fprintln(os.Stderr, "fatal: MINIO_ACCESS_KEY is required")
		os.Exit(1)
	}
	if cfg.MinioSecretKey == "" {
		fmt.Fprintln(os.Stderr, "fatal: MINIO_SECRET_KEY is required")
		os.Exit(1)
	}
	if cfg.RateLimitUploadLimit == 0 {
		cfg.RateLimitUploadLimit = 20
	}
	if cfg.RateLimitUploadWindowSeconds == 0 {
		cfg.RateLimitUploadWindowSeconds = 60
	}

	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "file-service",
	})

	// Database
	ctx := context.Background()
	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()
	log.Info().Msg("database connected")

	// MinIO
	mc, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create minio client")
	}
	// Ensure bucket exists.
	exists, err := mc.BucketExists(ctx, "relay-files")
	if err != nil {
		log.Warn().Err(err).Msg("minio bucket check failed")
	} else if !exists {
		if err := mc.MakeBucket(ctx, "relay-files", minio.MakeBucketOptions{}); err != nil {
			log.Warn().Err(err).Msg("failed to create relay-files bucket")
		}
	}
	log.Info().Str("endpoint", cfg.MinioEndpoint).Msg("minio connected")

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

	uploadRateLimit := middleware.RateLimitRedis(
		rdb,
		"file:upload",
		cfg.RateLimitUploadLimit,
		time.Duration(cfg.RateLimitUploadWindowSeconds)*time.Second,
	)
	log.Info().
		Int("limit", cfg.RateLimitUploadLimit).
		Int("window_seconds", cfg.RateLimitUploadWindowSeconds).
		Msg("file upload rate limiting configured")

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		log.Fatal().Msg("JWT_SECRET is required and must not be empty")
	}

	fileRepo := repository.New(pool)
	fileSvc := service.New(fileRepo, mc, log)
	fileHandler := handler.NewFileHandler(fileSvc)
	httpHandler := handler.NewRouter(fileHandler, jwtSecret, log, uploadRateLimit)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  60 * time.Second, // longer for large uploads
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("file-service HTTP listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down file-service")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown error")
		os.Exit(1)
	}
	log.Info().Msg("file-service stopped")
}

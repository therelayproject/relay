package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/relay-im/relay/services/user-service/internal/grpcserver"
	"github.com/relay-im/relay/services/user-service/internal/handler"
	"github.com/relay-im/relay/services/user-service/internal/repository"
	"github.com/relay-im/relay/services/user-service/internal/service"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/db"
	"github.com/relay-im/relay/shared/logger"
	userv1 "github.com/relay-im/relay/shared/proto/gen/user/v1"
	"google.golang.org/grpc"
)

// userConfig extends BaseConfig with user-service specific settings.
type userConfig struct {
	config.BaseConfig `mapstructure:",squash"`

	// MinIO / S3
	MinIOEndpoint   string `mapstructure:"MINIO_ENDPOINT"`
	MinIOAccessKey  string `mapstructure:"MINIO_ACCESS_KEY"`
	MinIOSecretKey  string `mapstructure:"MINIO_SECRET_KEY"`
	MinIOBucket     string `mapstructure:"MINIO_BUCKET"`
	MinIOUseSSL     bool   `mapstructure:"MINIO_USE_SSL"`
	MinIOPublicURL  string `mapstructure:"MINIO_PUBLIC_URL"`
}

func main() {
	cfg, err := config.Load[userConfig]("USER")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8002
	}
	if cfg.GRPCPort == 0 {
		cfg.GRPCPort = 9002
	}

	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "user-service",
	})

	// ── PostgreSQL ────────────────────────────────────────────────────────────
	connCtx, connCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer connCancel()

	pool, err := db.New(connCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer pool.Close()
	log.Info().Msg("postgres connected")

	// ── Redis (used for caching profiles) ────────────────────────────────────
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
	_ = rdb // available for future profile caching

	// ── Object storage (MinIO) ────────────────────────────────────────────────
	var storage service.StorageService
	if cfg.MinIOEndpoint != "" {
		minioStorage, minioErr := service.NewMinIOStorage(context.Background(), service.MinIOConfig{
			Endpoint:        cfg.MinIOEndpoint,
			AccessKeyID:     cfg.MinIOAccessKey,
			SecretAccessKey: cfg.MinIOSecretKey,
			Bucket:          orDefault(cfg.MinIOBucket, "relay-avatars"),
			UseSSL:          cfg.MinIOUseSSL,
			PublicBaseURL:   orDefault(cfg.MinIOPublicURL, cfg.MinIOEndpoint),
		})
		if minioErr != nil {
			log.Fatal().Err(minioErr).Msg("failed to connect to MinIO")
		}
		log.Info().Str("endpoint", cfg.MinIOEndpoint).Msg("MinIO connected")
		storage = minioStorage
	} else {
		log.Warn().Msg("MinIO not configured; using noop storage (dev mode)")
		storage = &service.NoopStorage{}
	}

	// ── Repositories / Services ───────────────────────────────────────────────
	userRepo := repository.NewUserRepo(pool)
	userSvc := service.New(userRepo, storage)

	// ── HTTP server ───────────────────────────────────────────────────────────
	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		log.Warn().Msg("JWT_SECRET not set; using insecure placeholder")
		jwtSecret = "change-me-in-production"
	}

	userHandler := handler.NewUserHandler(userSvc)
	httpRouter := handler.NewRouter(userHandler, jwtSecret, log)

	httpAddr := fmt.Sprintf(":%d", cfg.HTTPPort)
	httpSrv := &http.Server{
		Addr:         httpAddr,
		Handler:      httpRouter,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", httpAddr).Msg("HTTP server listening")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", grpcAddr).Msg("failed to bind gRPC listener")
	}
	grpcSrv := grpc.NewServer()
	userv1.RegisterUserServiceServer(grpcSrv, grpcserver.New(userSvc))

	go func() {
		log.Info().Str("addr", grpcAddr).Msg("gRPC server listening")
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server error")
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("shutting down user-service")

	grpcSrv.GracefulStop()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("HTTP graceful shutdown error")
		os.Exit(1)
	}
	log.Info().Msg("user-service stopped")
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

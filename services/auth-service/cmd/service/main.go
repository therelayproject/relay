package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	natsclient "github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/relay-im/relay/services/auth-service/internal/grpcserver"
	"github.com/relay-im/relay/services/auth-service/internal/handler"
	"github.com/relay-im/relay/services/auth-service/internal/repository"
	"github.com/relay-im/relay/services/auth-service/internal/service"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/db"
	"github.com/relay-im/relay/shared/logger"
	"github.com/relay-im/relay/shared/middleware"
	authv1 "github.com/relay-im/relay/shared/proto/gen/auth/v1"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthConfig extends the shared BaseConfig with auth-service-specific settings.
// It is defined here (main package) so the service owns its own configuration
// surface without polluting shared packages.
type AuthConfig struct {
	config.BaseConfig `mapstructure:",squash"`

	// SMTP — when empty the service falls back to NoopMailer.
	SMTPHost     string `mapstructure:"SMTP_HOST"`
	SMTPPort     int    `mapstructure:"SMTP_PORT"`
	SMTPFrom     string `mapstructure:"SMTP_FROM"`
	SMTPUsername string `mapstructure:"SMTP_USERNAME"`
	SMTPPassword string `mapstructure:"SMTP_PASSWORD"`

	// AppBaseURL is used for building links in verification / reset emails.
	AppBaseURL string `mapstructure:"BASE_URL"`

	// OAuth2 provider credentials.
	GoogleClientID     string `mapstructure:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `mapstructure:"GOOGLE_CLIENT_SECRET"`
	GitHubClientID     string `mapstructure:"GITHUB_CLIENT_ID"`
	GitHubClientSecret string `mapstructure:"GITHUB_CLIENT_SECRET"`

	// Rate limiting for sensitive public endpoints (login / register).
	// RATE_LIMIT_AUTH_LIMIT:          max requests per window (default 10).
	// RATE_LIMIT_AUTH_WINDOW_SECONDS: window size in seconds    (default 60).
	RateLimitAuthLimit         int `mapstructure:"RATE_LIMIT_AUTH_LIMIT"`
	RateLimitAuthWindowSeconds int `mapstructure:"RATE_LIMIT_AUTH_WINDOW_SECONDS"`
}

func main() {
	// ── Configuration ─────────────────────────────────────────────────────────
	cfg, err := config.Load[AuthConfig]("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: load config: %v\n", err)
		os.Exit(1)
	}

	// Service-level port defaults (override shared defaults of 8080/9090).
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 8001
	}
	if cfg.GRPCPort == 0 {
		cfg.GRPCPort = 9001
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 587
	}

	// ── Logger ────────────────────────────────────────────────────────────────
	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "auth-service",
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

	// ── NATS ──────────────────────────────────────────────────────────────────
	nc, err := natsclient.Connect(cfg.NATSURL)
	if err != nil {
		log.Fatal().Err(err).Str("url", cfg.NATSURL).Msg("failed to connect to nats")
	}
	defer nc.Drain() //nolint:errcheck
	log.Info().Msg("nats connected")

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo := repository.NewUserRepo(pool)
	sessionRepo := repository.NewSessionRepo(pool)
	tokenRepo := repository.NewTokenRepo(pool)

	// ── Services ──────────────────────────────────────────────────────────────
	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		log.Fatal().Msg("JWT_SECRET is required and must not be empty")
	}

	accessTTL := time.Duration(cfg.JWTAccessTokenTTL) * time.Second
	if accessTTL == 0 {
		accessTTL = 15 * time.Minute
	}
	refreshTTL := time.Duration(cfg.JWTRefreshTokenTTL) * time.Second
	if refreshTTL == 0 {
		refreshTTL = 30 * 24 * time.Hour
	}

	jwtSvc := service.NewJWTService(service.JWTConfig{
		Secret:          []byte(jwtSecret),
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		Issuer:          "relay-auth",
	})

	oauthSvc := service.NewOAuthService(service.OAuthConfig{
		GoogleClientID:     cfg.GoogleClientID,
		GoogleClientSecret: cfg.GoogleClientSecret,
		GitHubClientID:     cfg.GitHubClientID,
		GitHubClientSecret: cfg.GitHubClientSecret,
		RedirectBaseURL:    cfg.AppBaseURL,
	})

	var mailer service.Mailer
	if cfg.SMTPHost != "" {
		mailer = service.NewSMTPMailer(service.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			From:     cfg.SMTPFrom,
			BaseURL:  cfg.AppBaseURL,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
		})
		log.Info().Str("host", cfg.SMTPHost).Msg("SMTP mailer configured")
	} else {
		mailer = &service.NoopMailer{}
		log.Warn().Msg("SMTP not configured; emails will be printed to stdout (dev mode)")
	}

	authSvc := service.NewAuthService(
		userRepo, sessionRepo, tokenRepo,
		jwtSvc, oauthSvc, mailer, rdb,
	)

	// ── Rate limit defaults ───────────────────────────────────────────────────
	if cfg.RateLimitAuthLimit == 0 {
		cfg.RateLimitAuthLimit = 10
	}
	if cfg.RateLimitAuthWindowSeconds == 0 {
		cfg.RateLimitAuthWindowSeconds = 60
	}
	authRateLimit := middleware.RateLimitRedis(
		rdb,
		"auth",
		cfg.RateLimitAuthLimit,
		time.Duration(cfg.RateLimitAuthWindowSeconds)*time.Second,
	)
	log.Info().
		Int("limit", cfg.RateLimitAuthLimit).
		Int("window_seconds", cfg.RateLimitAuthWindowSeconds).
		Msg("auth rate limiting configured")

	// ── HTTP routes ───────────────────────────────────────────────────────────
	authHandler := handler.NewAuthHandler(authSvc, oauthSvc)
	authMW := middleware.Auth(jwtSecret)

	mux := http.NewServeMux()

	// Health check — no auth, not rate-limited at the middleware layer.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"auth-service"}`))
	})

	// Sensitive public endpoints — Redis-backed rate limit applied per-route.
	mux.Handle("POST /api/v1/auth/register", authRateLimit(http.HandlerFunc(authHandler.Register)))
	mux.Handle("POST /api/v1/auth/login", authRateLimit(http.HandlerFunc(authHandler.Login)))

	// Other public endpoints.
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("GET /api/v1/auth/oauth/{provider}", authHandler.OAuthRedirect)
	mux.HandleFunc("POST /api/v1/auth/oauth/{provider}/callback", authHandler.OAuthCallback)
	mux.HandleFunc("POST /api/v1/auth/password/reset-request", authHandler.PasswordResetRequest)
	mux.HandleFunc("POST /api/v1/auth/password/reset", authHandler.PasswordReset)
	mux.HandleFunc("POST /api/v1/auth/verify-email", authHandler.VerifyEmail)

	// Authenticated endpoints — auth middleware applied per-route.
	mux.Handle("GET /api/v1/auth/sessions", authMW(http.HandlerFunc(authHandler.ListSessions)))
	mux.Handle("DELETE /api/v1/auth/sessions/{id}", authMW(http.HandlerFunc(authHandler.RevokeSession)))
	mux.Handle("POST /api/v1/auth/mfa/setup", authMW(http.HandlerFunc(authHandler.SetupMFA)))
	mux.Handle("POST /api/v1/auth/mfa/verify", authMW(http.HandlerFunc(authHandler.VerifyMFA)))

	// Outer middleware chain: RequestID → Logger → mux.
	httpHandler := middleware.RequestID(
		middleware.Logger(log)(mux),
	)

	httpAddr := fmt.Sprintf(":%d", cfg.HTTPPort)
	httpSrv := &http.Server{
		Addr:         httpAddr,
		Handler:      httpHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", grpcAddr).Msg("failed to bind gRPC listener")
	}

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			unaryRecoveryInterceptor(log),
		),
	)
	authv1.RegisterAuthServiceServer(grpcSrv, grpcserver.New(authSvc))

	// ── Start both servers ────────────────────────────────────────────────────
	go func() {
		log.Info().Str("addr", httpAddr).Msg("HTTP server listening")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

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
	log.Info().Str("signal", sig.String()).Msg("shutting down auth-service")

	// Stop gRPC first — GracefulStop waits for in-flight RPCs to complete.
	grpcSrv.GracefulStop()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("HTTP graceful shutdown error")
		os.Exit(1)
	}

	log.Info().Msg("auth-service stopped")
}

// unaryRecoveryInterceptor converts panics inside gRPC handlers into
// Internal status errors, preventing the server from crashing.
func unaryRecoveryInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Str("method", info.FullMethod).
					Interface("panic", r).
					Bytes("stack", debug.Stack()).
					Msg("gRPC handler panic recovered")
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

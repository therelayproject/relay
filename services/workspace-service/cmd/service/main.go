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
	"github.com/rs/zerolog"

	"github.com/relay-im/relay/services/workspace-service/internal/handler"
	"github.com/relay-im/relay/services/workspace-service/internal/repository"
	"github.com/relay-im/relay/services/workspace-service/internal/service"
	"github.com/relay-im/relay/shared/config"
	"github.com/relay-im/relay/shared/db"
	"github.com/relay-im/relay/shared/logger"
	"github.com/relay-im/relay/shared/middleware"
)

// ServiceConfig extends BaseConfig with workspace-service–specific fields.
// Currently no extra fields are needed, but the type alias keeps the pattern
// consistent with other services.
type ServiceConfig struct {
	config.BaseConfig `mapstructure:",squash"`
}

func main() {
	cfg, err := config.Load[ServiceConfig]("WORKSPACE")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.Init(logger.Config{
		Level:   cfg.LogLevel,
		Pretty:  cfg.LogPretty,
		Service: "workspace-service",
	})

	// ── Database ────────────────────────────────────────────────────────────
	ctx := context.Background()
	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()
	log.Info().Msg("database connected")

	// ── NATS ────────────────────────────────────────────────────────────────
	nc, natsErr := nats.Connect(cfg.NATSURL)
	if natsErr != nil {
		log.Warn().Err(natsErr).Str("url", cfg.NATSURL).Msg("NATS unavailable – events will not be published")
		nc = nil
	} else {
		defer nc.Drain() //nolint:errcheck
		log.Info().Str("url", cfg.NATSURL).Msg("NATS connected")
	}

	// ── Wiring ──────────────────────────────────────────────────────────────
	repo := repository.New(pool)
	svc := service.New(repo, nc, log)
	h := handler.New(svc, log)

	// ── Router ──────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health probe (unauthenticated)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeHealthz(w)
	})

	authMw := middleware.Auth(cfg.JWTSecret)

	// Workspace collection routes
	mux.Handle("POST /api/v1/workspaces",
		authMw(http.HandlerFunc(h.CreateWorkspace)))
	mux.Handle("GET /api/v1/workspaces",
		authMw(http.HandlerFunc(h.ListMine)))

	// Join by token – must appear before the {id} wildcard pattern
	mux.Handle("POST /api/v1/workspaces/join",
		authMw(http.HandlerFunc(h.JoinByToken)))

	// Single workspace routes
	mux.HandleFunc("GET /api/v1/workspaces/{id}", h.GetWorkspace)
	mux.Handle("PATCH /api/v1/workspaces/{id}",
		authMw(http.HandlerFunc(h.UpdateSettings)))

	// Member sub-resource routes
	mux.Handle("GET /api/v1/workspaces/{id}/members",
		authMw(http.HandlerFunc(h.ListMembers)))
	mux.Handle("PATCH /api/v1/workspaces/{id}/members/{userId}",
		authMw(http.HandlerFunc(h.UpdateMemberRole)))
	mux.Handle("DELETE /api/v1/workspaces/{id}/members/{userId}",
		authMw(http.HandlerFunc(h.RemoveMember)))

	// Invitation routes
	mux.Handle("POST /api/v1/workspaces/{id}/invitations",
		authMw(http.HandlerFunc(h.CreateInvitation)))

	// ── HTTP server ─────────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%d", httpPort(cfg.HTTPPort))
	srv := &http.Server{
		Addr:         addr,
		Handler:      requestLogger(log, mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("starting workspace-service")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutdown signal received")

	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
		os.Exit(1)
	}
	log.Info().Msg("workspace-service stopped")
}

// httpPort returns 8003 if the configured port is zero (not set).
func httpPort(p int) int {
	if p == 0 {
		return 8003
	}
	return p
}

func writeHealthz(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"workspace-service"}`)) //nolint:errcheck
}

// requestLogger is a minimal structured HTTP request logging middleware.
func requestLogger(log zerolog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.status).
			Dur("latency_ms", time.Since(start)).
			Msg("http request")
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

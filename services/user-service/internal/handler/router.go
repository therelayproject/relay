package handler

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires all user-service routes with middleware.
func NewRouter(h *UserHandler, jwtSecret string, log zerolog.Logger) http.Handler {
	mux := http.NewServeMux()

	// ── Health ────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "user-service"})
	})

	authMW := middleware.Auth(jwtSecret)

	// ── Public read endpoints ─────────────────────────────────────────────────
	// Single user lookup (used by message rendering, workspace member lists, etc.)
	mux.Handle("GET /api/v1/users/{id}", authMW(http.HandlerFunc(h.GetUser)))

	// Batch user lookup (comma-separated ?ids=...)
	mux.Handle("GET /api/v1/users", authMW(http.HandlerFunc(h.GetUsers)))

	// ── Self-service profile endpoints ────────────────────────────────────────
	mux.Handle("PATCH /api/v1/users/{id}", authMW(http.HandlerFunc(h.UpdateProfile)))
	mux.Handle("PUT /api/v1/users/{id}/status", authMW(http.HandlerFunc(h.SetStatus)))
	mux.Handle("POST /api/v1/users/{id}/avatar", authMW(http.HandlerFunc(h.UploadAvatar)))

	return middleware.RequestID(
		middleware.Logger(log)(
			middleware.RateLimit(200, 400)(mux),
		),
	)
}

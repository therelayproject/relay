package handler

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires all auth routes and attaches middleware.
func NewRouter(h *AuthHandler, jwtSecret string, log zerolog.Logger) http.Handler {
	mux := http.NewServeMux()

	// ── Health ────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "auth-service"})
	})

	// ── Public auth endpoints ─────────────────────────────────────────────────
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/refresh", h.Refresh)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	mux.HandleFunc("POST /auth/oauth/google", h.OAuthCallback)
	mux.HandleFunc("POST /auth/oauth/github", h.OAuthCallback)
	mux.HandleFunc("POST /auth/password/reset-request", h.PasswordResetRequest)
	mux.HandleFunc("POST /auth/password/reset", h.PasswordReset)
	mux.HandleFunc("GET /auth/verify-email", h.VerifyEmail)

	// ── Authenticated endpoints ───────────────────────────────────────────────
	authMW := middleware.Auth(jwtSecret)

	mux.Handle("GET /auth/sessions", authMW(http.HandlerFunc(h.ListSessions)))
	mux.Handle("DELETE /auth/sessions/", authMW(http.HandlerFunc(h.RevokeSession)))
	mux.Handle("POST /auth/mfa/setup", authMW(http.HandlerFunc(h.SetupMFA)))
	mux.Handle("POST /auth/mfa/verify", authMW(http.HandlerFunc(h.VerifyMFA)))

	// Wrap entire mux with request-id and logging middleware.
	return middleware.RequestID(
		middleware.Logger(log)(mux),
	)
}

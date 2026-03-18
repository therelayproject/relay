package handler

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires presence-service routes.
func NewRouter(h *PresenceHandler, jwtSecret string, log zerolog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "presence-service"})
	})

	authMW := middleware.Auth(jwtSecret)

	mux.Handle("POST /api/v1/presence/heartbeat",
		authMW(http.HandlerFunc(h.Heartbeat)))
	mux.Handle("GET /api/v1/workspaces/{workspace_id}/presence",
		authMW(http.HandlerFunc(h.WorkspacePresence)))

	// Custom status (PRES-02)
	mux.Handle("PUT /api/v1/users/me/status",
		authMW(http.HandlerFunc(h.SetCustomStatus)))
	mux.Handle("DELETE /api/v1/users/me/status",
		authMW(http.HandlerFunc(h.ClearCustomStatus)))
	mux.HandleFunc("GET /api/v1/users/{user_id}/status", h.GetCustomStatus)

	return middleware.RequestID(middleware.Logger(log)(mux))
}

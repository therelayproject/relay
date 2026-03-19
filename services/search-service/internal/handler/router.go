package handler

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires search-service routes.
func NewRouter(h *SearchHandler, jwtSecret string, log zerolog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "search-service"})
	})

	authMW := middleware.Auth(jwtSecret)

	// SRCH-01,02: Full-text search with optional filters
	// SRCH-03: Pass channel_id query param to scope search to a channel
	mux.Handle("GET /api/v1/workspaces/{workspace_id}/search",
		authMW(http.HandlerFunc(h.Search)))

	return middleware.RequestID(middleware.Logger(log)(mux))
}

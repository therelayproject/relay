package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires ws-gateway routes.
func NewRouter(h *GatewayHandler, log zerolog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "ws-gateway"})
	})

	// WebSocket endpoint — token auth happens inside the handler.
	mux.HandleFunc("GET /gateway/connect", h.Connect)

	return middleware.RequestID(middleware.Logger(log)(mux))
}

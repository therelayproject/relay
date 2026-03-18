package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires ws-gateway routes.
// connectMiddlewares are applied (innermost-first) to the WebSocket connect
// endpoint only (e.g. rate limiting).
func NewRouter(h *GatewayHandler, log zerolog.Logger, connectMiddlewares ...func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "ws-gateway"})
	})

	// WebSocket endpoint — token auth happens inside the handler.
	// Wrap with any provided per-endpoint middlewares (e.g. rate limiter).
	var connectHandler http.Handler = http.HandlerFunc(h.Connect)
	for i := len(connectMiddlewares) - 1; i >= 0; i-- {
		connectHandler = connectMiddlewares[i](connectHandler)
	}
	mux.Handle("GET /gateway/connect", connectHandler)

	return middleware.RequestID(middleware.Logger(log)(mux))
}

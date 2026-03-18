package handler

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires all message-service routes.
func NewRouter(h *MessageHandler, jwtSecret string, log zerolog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "message-service"})
	})

	authMW := middleware.Auth(jwtSecret)

	// Messages
	mux.Handle("POST /api/v1/channels/{channel_id}/messages",
		authMW(http.HandlerFunc(h.SendMessage)))
	mux.HandleFunc("GET /api/v1/channels/{channel_id}/messages",
		h.ListMessages)
	mux.HandleFunc("GET /api/v1/channels/{channel_id}/messages/{message_id}/thread",
		h.GetThread)
	mux.Handle("PATCH /api/v1/channels/{channel_id}/messages/{message_id}",
		authMW(http.HandlerFunc(h.EditMessage)))
	mux.Handle("DELETE /api/v1/channels/{channel_id}/messages/{message_id}",
		authMW(http.HandlerFunc(h.DeleteMessage)))

	// Reactions
	mux.Handle("POST /api/v1/channels/{channel_id}/messages/{message_id}/reactions",
		authMW(http.HandlerFunc(h.AddReaction)))
	mux.Handle("DELETE /api/v1/channels/{channel_id}/messages/{message_id}/reactions/{emoji}",
		authMW(http.HandlerFunc(h.RemoveReaction)))
	mux.HandleFunc("GET /api/v1/channels/{channel_id}/messages/{message_id}/reactions",
		h.ListReactions)

	// Pins
	mux.Handle("POST /api/v1/channels/{channel_id}/pins/{message_id}",
		authMW(http.HandlerFunc(h.PinMessage)))
	mux.Handle("DELETE /api/v1/channels/{channel_id}/pins/{message_id}",
		authMW(http.HandlerFunc(h.UnpinMessage)))
	mux.HandleFunc("GET /api/v1/channels/{channel_id}/pins",
		h.ListPins)

	return middleware.RequestID(
		middleware.Logger(log)(
			middleware.RateLimit(200, 400)(mux),
		),
	)
}

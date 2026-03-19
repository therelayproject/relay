package handler

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires notification-service routes.
func NewRouter(h *NotificationHandler, jwtSecret string, log zerolog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "notification-service"})
	})

	authMW := middleware.Auth(jwtSecret)

	mux.Handle("GET /api/v1/users/me/notification-preferences",
		authMW(http.HandlerFunc(h.GetPreferences)))
	mux.Handle("PUT /api/v1/users/me/notification-preferences/{scope}",
		authMW(http.HandlerFunc(h.UpsertPreference)))
	mux.Handle("POST /api/v1/users/me/push-tokens",
		authMW(http.HandlerFunc(h.RegisterPushToken)))
	mux.Handle("DELETE /api/v1/users/me/push-tokens/{platform}",
		authMW(http.HandlerFunc(h.DeletePushToken)))
	mux.Handle("POST /api/v1/users/me/dnd",
		authMW(http.HandlerFunc(h.SetDND)))

	return middleware.RequestID(middleware.Logger(log)(mux))
}

// Package handler contains HTTP handlers for the notification service.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/relay-im/relay/services/notification-service/internal/domain"
	"github.com/relay-im/relay/services/notification-service/internal/service"
	"github.com/relay-im/relay/shared/middleware"
)

// NotificationHandler handles notification REST endpoints.
type NotificationHandler struct {
	svc *service.NotificationService
}

// New creates a NotificationHandler.
func New(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// ── Preferences ───────────────────────────────────────────────────────────────

// GetPreferences handles GET /api/v1/users/me/notification-preferences
func (h *NotificationHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "missing auth"))
		return
	}
	prefs, err := h.svc.GetPreferences(r.Context(), claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preferences": prefs})
}

// UpsertPreference handles PUT /api/v1/users/me/notification-preferences/{scope}
func (h *NotificationHandler) UpsertPreference(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "missing auth"))
		return
	}
	scope := r.PathValue("scope")
	if scope == "" {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "scope required"))
		return
	}

	var req struct {
		Level domain.Level `json:"level"`
		Muted bool         `json:"muted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "invalid JSON"))
		return
	}
	if req.Level == "" {
		req.Level = domain.LevelMentions
	}

	pref, err := h.svc.UpsertPreference(r.Context(), claims.UserID, scope, req.Level, req.Muted)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

// ── Push tokens ───────────────────────────────────────────────────────────────

// RegisterPushToken handles POST /api/v1/users/me/push-tokens
func (h *NotificationHandler) RegisterPushToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "missing auth"))
		return
	}

	var req struct {
		Platform string `json:"platform"`
		Token    string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "invalid JSON"))
		return
	}
	if req.Platform == "" || req.Token == "" {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "platform and token required"))
		return
	}

	if err := h.svc.UpsertPushToken(r.Context(), claims.UserID, req.Platform, req.Token); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// DeletePushToken handles DELETE /api/v1/users/me/push-tokens/{platform}
func (h *NotificationHandler) DeletePushToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "missing auth"))
		return
	}
	platform := r.PathValue("platform")
	if err := h.svc.DeletePushToken(r.Context(), claims.UserID, platform); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── DND ───────────────────────────────────────────────────────────────────────

// SetDND handles POST /api/v1/users/me/dnd
func (h *NotificationHandler) SetDND(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errResp("UNAUTHORIZED", "missing auth"))
		return
	}

	var req struct {
		Until string `json:"until"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "invalid JSON"))
		return
	}

	until, err := time.Parse(time.RFC3339, req.Until)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("VALIDATION_ERROR", "until must be RFC3339"))
		return
	}

	if err := h.svc.SetDND(r.Context(), claims.UserID, until); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("INTERNAL_ERROR", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

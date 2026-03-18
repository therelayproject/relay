// Package handler contains HTTP handlers for the presence service.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/relay-im/relay/services/presence-service/internal/service"
	"github.com/relay-im/relay/shared/middleware"
)

// PresenceHandler handles presence REST endpoints.
type PresenceHandler struct {
	svc *service.PresenceService
}

// NewPresenceHandler creates a PresenceHandler.
func NewPresenceHandler(svc *service.PresenceService) *PresenceHandler {
	return &PresenceHandler{svc: svc}
}

type heartbeatRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

// Heartbeat handles POST /api/v1/presence/heartbeat
func (h *PresenceHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}

	var req heartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	if req.WorkspaceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "workspace_id required"})
		return
	}

	if err := h.svc.Heartbeat(r.Context(), claims.UserID, req.WorkspaceID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// WorkspacePresence handles GET /api/v1/workspaces/{workspace_id}/presence
func (h *PresenceHandler) WorkspacePresence(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspace_id")
	if workspaceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "workspace_id required"})
		return
	}

	presence, err := h.svc.WorkspacePresence(r.Context(), workspaceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"presence": presence})
}

type setStatusRequest struct {
	Emoji     string  `json:"emoji"`
	Text      string  `json:"text"`
	ExpiresAt *string `json:"expires_at"` // RFC3339 or null
}

// SetCustomStatus handles PUT /api/v1/users/me/status
func (h *PresenceHandler) SetCustomStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}

	var req setStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "expires_at must be RFC3339"})
			return
		}
		expiresAt = &t
	}

	if err := h.svc.SetCustomStatus(r.Context(), claims.UserID, req.Emoji, req.Text, expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GetCustomStatus handles GET /api/v1/users/{user_id}/status
func (h *PresenceHandler) GetCustomStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "user_id required"})
		return
	}

	cs, err := h.svc.GetCustomStatus(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	if cs == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": map[string]any{
			"emoji":      cs.Emoji,
			"text":       cs.Text,
			"expires_at": cs.ExpiresAt,
			"updated_at": cs.UpdatedAt.Format(time.RFC3339),
		},
	})
}

// ClearCustomStatus handles DELETE /api/v1/users/me/status
func (h *PresenceHandler) ClearCustomStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}
	if err := h.svc.ClearCustomStatus(r.Context(), claims.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

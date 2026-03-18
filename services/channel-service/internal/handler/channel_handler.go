package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/relay-im/relay/services/channel-service/internal/domain"
	"github.com/relay-im/relay/services/channel-service/internal/service"
	apperrors "github.com/relay-im/relay/shared/errors"
	"github.com/relay-im/relay/shared/middleware"
)

// ChannelHandler exposes the REST API for channel operations.
type ChannelHandler struct {
	svc *service.ChannelService
}

// NewChannelHandler constructs a ChannelHandler.
func NewChannelHandler(svc *service.ChannelService) *ChannelHandler {
	return &ChannelHandler{svc: svc}
}

// ── Request / Response shapes ─────────────────────────────────────────────────

type createChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type updateChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Topic       string `json:"topic"`
}

type addMemberRequest struct {
	UserID string `json:"user_id"`
}

type channelResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Topic       string `json:"topic"`
	Type        string `json:"type"`
	IsArchived  bool   `json:"is_archived"`
	CreatedBy   string `json:"created_by"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type memberResponse struct {
	ChannelID  string  `json:"channel_id"`
	UserID     string  `json:"user_id"`
	Role       string  `json:"role"`
	LastReadAt *string `json:"last_read_at,omitempty"`
	JoinedAt   string  `json:"joined_at"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func toChannelResponse(c *domain.Channel) channelResponse {
	return channelResponse{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Slug:        c.Slug,
		Description: c.Description,
		Topic:       c.Topic,
		Type:        string(c.Type),
		IsArchived:  c.IsArchived,
		CreatedBy:   c.CreatedBy,
		MemberCount: c.MemberCount,
		CreatedAt:   c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   c.UpdatedAt.Format(time.RFC3339),
	}
}

func toMemberResponse(m domain.ChannelMember) memberResponse {
	r := memberResponse{
		ChannelID: m.ChannelID,
		UserID:    m.UserID,
		Role:      string(m.Role),
		JoinedAt:  m.JoinedAt.Format(time.RFC3339),
	}
	if m.LastReadAt != nil {
		s := m.LastReadAt.Format(time.RFC3339)
		r.LastReadAt = &s
	}
	return r
}

func requireClaims(w http.ResponseWriter, r *http.Request) (*middleware.Claims, bool) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error":   "UNAUTHENTICATED",
			"message": "missing auth claims",
		})
	}
	return claims, ok
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// CreateChannel handles POST /api/v1/workspaces/{workspaceId}/channels
func (h *ChannelHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	workspaceID := r.PathValue("workspaceId")

	var req createChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "INVALID_ARGUMENT",
			"message": "invalid JSON body",
		})
		return
	}

	ch, err := h.svc.CreateChannel(r.Context(), workspaceID, req.Name, req.Description, req.Type, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toChannelResponse(ch))
}

// BrowseChannels handles GET /api/v1/workspaces/{workspaceId}/channels
func (h *ChannelHandler) BrowseChannels(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	workspaceID := r.PathValue("workspaceId")

	channels, err := h.svc.BrowseChannels(r.Context(), workspaceID, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]channelResponse, len(channels))
	for i, c := range channels {
		resp[i] = toChannelResponse(&c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": resp})
}

// GetChannel handles GET /api/v1/workspaces/{workspaceId}/channels/{id}
func (h *ChannelHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	ch, err := h.svc.GetChannel(r.Context(), id, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toChannelResponse(ch))
}

// UpdateChannel handles PATCH /api/v1/workspaces/{workspaceId}/channels/{id}
func (h *ChannelHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	var req updateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "INVALID_ARGUMENT",
			"message": "invalid JSON body",
		})
		return
	}

	ch, err := h.svc.UpdateChannel(r.Context(), id, req.Name, req.Description, req.Topic, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toChannelResponse(ch))
}

// ArchiveChannel handles DELETE /api/v1/workspaces/{workspaceId}/channels/{id}
func (h *ChannelHandler) ArchiveChannel(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	if err := h.svc.ArchiveChannel(r.Context(), id, claims.UserID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// JoinChannel handles POST /api/v1/workspaces/{workspaceId}/channels/{id}/join
func (h *ChannelHandler) JoinChannel(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	if err := h.svc.JoinChannel(r.Context(), id, claims.UserID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// LeaveChannel handles POST /api/v1/workspaces/{workspaceId}/channels/{id}/leave
func (h *ChannelHandler) LeaveChannel(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	if err := h.svc.LeaveChannel(r.Context(), id, claims.UserID); err != nil {
		// Not a member — treat as success (idempotent leave).
		if ae, ok2 := err.(*apperrors.AppError); ok2 && ae.Code == apperrors.CodeNotFound {
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ListMembers handles GET /api/v1/workspaces/{workspaceId}/channels/{id}/members
func (h *ChannelHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	members, err := h.svc.ListMembers(r.Context(), id, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]memberResponse, len(members))
	for i, m := range members {
		resp[i] = toMemberResponse(m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": resp})
}

// AddMember handles POST /api/v1/workspaces/{workspaceId}/channels/{id}/members
func (h *ChannelHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")

	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "INVALID_ARGUMENT",
			"message": "user_id is required",
		})
		return
	}

	if err := h.svc.AddMember(r.Context(), id, req.UserID, claims.UserID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

// RemoveMember handles DELETE /api/v1/workspaces/{workspaceId}/channels/{id}/members/{userId}
func (h *ChannelHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	claims, ok := requireClaims(w, r)
	if !ok {
		return
	}
	channelID := r.PathValue("id")
	userID := r.PathValue("userId")

	if err := h.svc.RemoveMember(r.Context(), channelID, userID, claims.UserID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// NewRouter wires all channel routes onto mux, wrapped with auth middleware.
// The healthz route is unauthenticated.
func NewRouter(h *ChannelHandler, jwtSecret string) http.Handler {
	mux := http.NewServeMux()

	authMW := middleware.Auth(jwtSecret)

	// Healthcheck — no auth required.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "channel-service",
		})
	})

	// Authenticated channel routes.
	mux.Handle("POST /api/v1/workspaces/{workspaceId}/channels",
		authMW(http.HandlerFunc(h.CreateChannel)))
	mux.Handle("GET /api/v1/workspaces/{workspaceId}/channels",
		authMW(http.HandlerFunc(h.BrowseChannels)))
	mux.Handle("GET /api/v1/workspaces/{workspaceId}/channels/{id}",
		authMW(http.HandlerFunc(h.GetChannel)))
	mux.Handle("PATCH /api/v1/workspaces/{workspaceId}/channels/{id}",
		authMW(http.HandlerFunc(h.UpdateChannel)))
	mux.Handle("DELETE /api/v1/workspaces/{workspaceId}/channels/{id}",
		authMW(http.HandlerFunc(h.ArchiveChannel)))
	mux.Handle("POST /api/v1/workspaces/{workspaceId}/channels/{id}/join",
		authMW(http.HandlerFunc(h.JoinChannel)))
	mux.Handle("POST /api/v1/workspaces/{workspaceId}/channels/{id}/leave",
		authMW(http.HandlerFunc(h.LeaveChannel)))
	mux.Handle("GET /api/v1/workspaces/{workspaceId}/channels/{id}/members",
		authMW(http.HandlerFunc(h.ListMembers)))
	mux.Handle("POST /api/v1/workspaces/{workspaceId}/channels/{id}/members",
		authMW(http.HandlerFunc(h.AddMember)))
	mux.Handle("DELETE /api/v1/workspaces/{workspaceId}/channels/{id}/members/{userId}",
		authMW(http.HandlerFunc(h.RemoveMember)))

	return mux
}

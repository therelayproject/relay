// Package handler wires HTTP routes to workspace service operations.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/relay-im/relay/services/workspace-service/internal/domain"
	"github.com/relay-im/relay/services/workspace-service/internal/service"
	apperrors "github.com/relay-im/relay/shared/errors"
	"github.com/relay-im/relay/shared/middleware"
)

// WorkspaceHandler holds the HTTP handlers for workspace routes.
type WorkspaceHandler struct {
	svc *service.WorkspaceService
	log zerolog.Logger
}

// New returns a WorkspaceHandler.
func New(svc *service.WorkspaceService, log zerolog.Logger) *WorkspaceHandler {
	return &WorkspaceHandler{svc: svc, log: log}
}

// ── Request / Response types ─────────────────────────────────────────────────

type createWorkspaceRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type updateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IconURL     string `json:"icon_url"`
}

type createInvitationRequest struct {
	Email string `json:"email"` // optional; omit for link-only invite
	Role  string `json:"role"`
}

type joinRequest struct {
	Token string `json:"token"`
}

type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

type workspaceResponse struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Slug              string `json:"slug"`
	Description       string `json:"description,omitempty"`
	IconURL           string `json:"icon_url,omitempty"`
	OwnerID           string `json:"owner_id"`
	AllowGuestInvites bool   `json:"allow_guest_invites"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type memberResponse struct {
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Role        string `json:"role"`
	InvitedBy   string `json:"invited_by,omitempty"`
	JoinedAt    string `json:"joined_at"`
}

type invitationResponse struct {
	Token string `json:"token"`
}

func toWorkspaceResponse(ws *domain.Workspace) workspaceResponse {
	return workspaceResponse{
		ID:                ws.ID,
		Name:              ws.Name,
		Slug:              ws.Slug,
		Description:       ws.Description,
		IconURL:           ws.IconURL,
		OwnerID:           ws.OwnerID,
		AllowGuestInvites: ws.AllowGuestInvites,
		CreatedAt:         ws.CreatedAt.UTC().String(),
		UpdatedAt:         ws.UpdatedAt.UTC().String(),
	}
}

func toMemberResponse(m *domain.WorkspaceMember) memberResponse {
	return memberResponse{
		WorkspaceID: m.WorkspaceID,
		UserID:      m.UserID,
		Role:        m.Role,
		InvitedBy:   m.InvitedBy,
		JoinedAt:    m.JoinedAt.UTC().String(),
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// claimsFromCtx extracts JWT claims or writes a 401 and returns false.
func claimsFromCtx(w http.ResponseWriter, r *http.Request) (*middleware.Claims, bool) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeUnauthenticated, "authentication required"))
		return nil, false
	}
	return claims, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "malformed request body"))
		return false
	}
	return true
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// CreateWorkspace handles POST /api/v1/workspaces.
func (h *WorkspaceHandler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromCtx(w, r)
	if !ok {
		return
	}

	var req createWorkspaceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ws, err := h.svc.CreateWorkspace(r.Context(), req.Name, req.Slug, req.Description, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toWorkspaceResponse(ws))
}

// ListMine handles GET /api/v1/workspaces.
func (h *WorkspaceHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromCtx(w, r)
	if !ok {
		return
	}

	workspaces, err := h.svc.ListWorkspaces(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]workspaceResponse, 0, len(workspaces))
	for i := range workspaces {
		resp = append(resp, toWorkspaceResponse(&workspaces[i]))
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetWorkspace handles GET /api/v1/workspaces/{id}.
func (h *WorkspaceHandler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ws, err := h.svc.GetWorkspace(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWorkspaceResponse(ws))
}

// UpdateSettings handles PATCH /api/v1/workspaces/{id}.
func (h *WorkspaceHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromCtx(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	var req updateWorkspaceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ws, err := h.svc.UpdateSettings(r.Context(), id, req.Name, req.Description, req.IconURL, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWorkspaceResponse(ws))
}

// ListMembers handles GET /api/v1/workspaces/{id}/members.
func (h *WorkspaceHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromCtx(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	members, err := h.svc.ListMembers(r.Context(), id, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]memberResponse, 0, len(members))
	for i := range members {
		resp = append(resp, toMemberResponse(&members[i]))
	}
	writeJSON(w, http.StatusOK, resp)
}

// CreateInvitation handles POST /api/v1/workspaces/{id}/invitations.
// If the request body includes an "email" field an email-based invitation is
// created; otherwise a shareable link token is generated.
func (h *WorkspaceHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromCtx(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	var req createInvitationRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Role == "" {
		req.Role = domain.RoleMember
	}

	var token string
	var err error
	if req.Email != "" {
		token, err = h.svc.InviteByEmail(r.Context(), id, req.Email, req.Role, claims.UserID)
	} else {
		token, err = h.svc.InviteByLink(r.Context(), id, req.Role, claims.UserID)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invitationResponse{Token: token})
}

// JoinByToken handles POST /api/v1/workspaces/join.
func (h *WorkspaceHandler) JoinByToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromCtx(w, r)
	if !ok {
		return
	}

	var req joinRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	member, err := h.svc.JoinByToken(r.Context(), req.Token, claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toMemberResponse(member))
}

// UpdateMemberRole handles PATCH /api/v1/workspaces/{id}/members/{userId}.
func (h *WorkspaceHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromCtx(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	userID := r.PathValue("userId")

	var req updateMemberRoleRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.svc.UpdateMemberRole(r.Context(), id, userID, req.Role, claims.UserID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveMember handles DELETE /api/v1/workspaces/{id}/members/{userId}.
func (h *WorkspaceHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromCtx(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	userID := r.PathValue("userId")

	if err := h.svc.RemoveMember(r.Context(), id, userID, claims.UserID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

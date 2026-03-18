package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/relay-im/relay/services/user-service/internal/domain"
	"github.com/relay-im/relay/services/user-service/internal/service"
	apperrors "github.com/relay-im/relay/shared/errors"
	"github.com/relay-im/relay/shared/middleware"
)

// UserHandler exposes the REST API for user profile operations.
type UserHandler struct {
	svc *service.UserService
}

// NewUserHandler constructs a UserHandler.
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// ─── Response shapes ──────────────────────────────────────────────────────────

type profileResponse struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	StatusText  string `json:"status_text"`
	StatusEmoji string `json:"status_emoji"`
	Timezone    string `json:"timezone"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toProfileResponse(p *domain.Profile) profileResponse {
	return profileResponse{
		UserID:      p.UserID,
		DisplayName: p.DisplayName,
		AvatarURL:   p.AvatarURL,
		StatusText:  p.StatusText,
		StatusEmoji: p.StatusEmoji,
		Timezone:    p.Timezone,
		CreatedAt:   p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// GetUser handles GET /api/v1/users/{id}
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "user id is required"))
		return
	}

	profile, err := h.svc.GetProfile(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": toProfileResponse(profile)})
}

// GetUsers handles GET /api/v1/users?ids=u1,u2,u3
func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "ids query parameter is required"))
		return
	}

	ids := splitIDs(raw)
	if len(ids) == 0 {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "ids must contain at least one non-empty value"))
		return
	}

	profiles, err := h.svc.GetProfiles(r.Context(), ids)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]profileResponse, len(profiles))
	for i := range profiles {
		resp[i] = toProfileResponse(&profiles[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": resp})
}

// UpdateProfile handles PATCH /api/v1/users/{id}
// The authenticated user may only update their own profile.
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeUnauthenticated, "authentication required"))
		return
	}
	if claims.UserID != userID {
		writeError(w, apperrors.New(apperrors.CodePermissionDenied, "cannot modify another user's profile"))
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
		Timezone    string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid JSON body"))
		return
	}

	profile, err := h.svc.UpdateProfile(r.Context(), userID, req.DisplayName, req.AvatarURL, req.Timezone)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"user": toProfileResponse(profile)})
}

// SetStatus handles PUT /api/v1/users/{id}/status
// The authenticated user may only set their own status.
func (h *UserHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeUnauthenticated, "authentication required"))
		return
	}
	if claims.UserID != userID {
		writeError(w, apperrors.New(apperrors.CodePermissionDenied, "cannot modify another user's status"))
		return
	}

	var req struct {
		StatusText  string `json:"status_text"`
		StatusEmoji string `json:"status_emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid JSON body"))
		return
	}

	if err := h.svc.SetStatus(r.Context(), domain.StatusUpdate{
		UserID:      userID,
		StatusText:  req.StatusText,
		StatusEmoji: req.StatusEmoji,
	}); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// UploadAvatar handles POST /api/v1/users/{id}/avatar
// Accepts a multipart/form-data file upload (field name "avatar").
// The actual object-store upload is delegated to the StorageService wired at
// startup; by default a noop stub returns a placeholder MinIO URL.
func (h *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeUnauthenticated, "authentication required"))
		return
	}
	if claims.UserID != userID {
		writeError(w, apperrors.New(apperrors.CodePermissionDenied, "cannot upload avatar for another user"))
		return
	}

	const maxSize = 8 << 20 // 8 MiB
	if err := r.ParseMultipartForm(maxSize); err != nil {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "failed to parse multipart form"))
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "avatar file field is required"))
		return
	}
	defer file.Close()

	// Detect content type from the MIME header or sniff from the first bytes.
	contentType := header.Header.Get("Content-Type")

	// Read up to maxSize bytes.
	data := make([]byte, 0, min(int(header.Size), maxSize))
	buf := make([]byte, 32*1024)
	total := 0
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			total += n
			if total > maxSize {
				writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "avatar file exceeds 8 MiB limit"))
				return
			}
			data = append(data, buf[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	if contentType == "" && len(data) > 0 {
		contentType = http.DetectContentType(data)
	}

	avatarURL, uploadErr := h.svc.UploadAvatar(r.Context(), userID, data, contentType)
	if uploadErr != nil {
		writeError(w, uploadErr)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"avatar_url": avatarURL,
	})
}

// splitIDs splits a comma-separated ID string, trimming whitespace and
// dropping empty segments.
func splitIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

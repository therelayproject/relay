// Package handler contains HTTP handlers for the file service.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/relay-im/relay/services/file-service/internal/domain"
	"github.com/relay-im/relay/services/file-service/internal/service"
	"github.com/relay-im/relay/shared/middleware"
	apperrors "github.com/relay-im/relay/shared/errors"
)

const maxMultipartMemory = 32 << 20 // 32 MiB

// FileHandler handles file REST endpoints.
type FileHandler struct {
	svc *service.FileService
}

// NewFileHandler creates a FileHandler.
func NewFileHandler(svc *service.FileService) *FileHandler {
	return &FileHandler{svc: svc}
}

// Upload handles POST /api/v1/workspaces/{workspace_id}/files
// Expects multipart/form-data with field "file" and optional "channel_id".
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeError(w, apperrors.New(apperrors.CodeUnauthenticated, "missing auth"))
		return
	}

	workspaceID := r.PathValue("workspace_id")
	if workspaceID == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "workspace_id required"))
		return
	}

	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "invalid multipart form"))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "file field required"))
		return
	}
	defer file.Close()

	channelID := r.FormValue("channel_id")
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	created, err := h.svc.Upload(
		r.Context(),
		workspaceID, channelID, claims.UserID,
		header.Filename, contentType, header.Size,
		file,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, fileResponse(created))
}

// GetDownloadURL handles GET /api/v1/files/{file_id}/download
func (h *FileHandler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("file_id")
	if fileID == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "file_id required"))
		return
	}

	signedURL, err := h.svc.PresignDownload(r.Context(), fileID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": signedURL})
}

// GetThumbnailURL handles GET /api/v1/files/{file_id}/thumbnail
func (h *FileHandler) GetThumbnailURL(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("file_id")
	if fileID == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "file_id required"))
		return
	}

	signedURL, err := h.svc.PresignThumbnail(r.Context(), fileID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": signedURL})
}

// GetFile handles GET /api/v1/files/{file_id}
func (h *FileHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("file_id")
	if fileID == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "file_id required"))
		return
	}

	f, err := h.svc.GetFile(r.Context(), fileID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fileResponse(f))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func fileResponse(f *domain.File) map[string]any {
	return map[string]any{
		"id":            f.ID,
		"workspace_id":  f.WorkspaceID,
		"channel_id":    f.ChannelID,
		"uploader_id":   f.UploaderID,
		"filename":      f.Filename,
		"content_type":  f.ContentType,
		"size_bytes":    f.SizeBytes,
		"is_image":      f.IsImage,
		"has_thumbnail": f.ThumbnailKey != "",
		"created_at":    f.CreatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, err error) {
	status := apperrors.HTTPStatus(err)
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

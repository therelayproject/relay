package handler

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/shared/middleware"
)

// NewRouter wires file-service routes.
// uploadMiddlewares are applied (innermost-first) to the upload endpoint only
// (e.g. rate limiting). They wrap the handler after auth middleware.
func NewRouter(h *FileHandler, jwtSecret string, log zerolog.Logger, uploadMiddlewares ...func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "file-service"})
	})

	authMW := middleware.Auth(jwtSecret)

	// FILE-01: Upload — auth + rate limiting applied.
	var uploadHandler http.Handler = http.HandlerFunc(h.Upload)
	for i := len(uploadMiddlewares) - 1; i >= 0; i-- {
		uploadHandler = uploadMiddlewares[i](uploadHandler)
	}
	mux.Handle("POST /api/v1/workspaces/{workspace_id}/files",
		authMW(uploadHandler))

	// Metadata
	mux.Handle("GET /api/v1/files/{file_id}",
		authMW(http.HandlerFunc(h.GetFile)))

	// FILE-03: Download (presigned URL)
	mux.Handle("GET /api/v1/files/{file_id}/download",
		authMW(http.HandlerFunc(h.GetDownloadURL)))

	// FILE-02: Image thumbnail URL
	mux.Handle("GET /api/v1/files/{file_id}/thumbnail",
		authMW(http.HandlerFunc(h.GetThumbnailURL)))

	return middleware.RequestID(middleware.Logger(log)(mux))
}

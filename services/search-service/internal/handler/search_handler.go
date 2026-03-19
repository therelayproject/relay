// Package handler contains HTTP handlers for the search service.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/relay-im/relay/services/search-service/internal/domain"
	"github.com/relay-im/relay/services/search-service/internal/service"
	"github.com/relay-im/relay/shared/middleware"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// SearchHandler handles search REST endpoints.
type SearchHandler struct {
	svc *service.SearchService
}

// NewSearchHandler creates a SearchHandler.
func NewSearchHandler(svc *service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// Search handles GET /api/v1/workspaces/{workspace_id}/search
// Query params: q, channel_id, author_id, after, before, from, size
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.UserID == "" {
		writeError(w, apperrors.New(apperrors.CodeUnauthenticated, "missing auth"))
		return
	}

	workspaceID := r.PathValue("workspace_id")
	if workspaceID == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "workspace_id required"))
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, apperrors.New(apperrors.CodeInvalidArgument, "q (query) is required"))
		return
	}

	filter := domain.SearchFilter{
		WorkspaceID: workspaceID,
		ChannelID:   r.URL.Query().Get("channel_id"),
		AuthorID:    r.URL.Query().Get("author_id"),
	}

	if afterStr := r.URL.Query().Get("after"); afterStr != "" {
		if t, err := time.Parse(time.RFC3339, afterStr); err == nil {
			filter.After = t
		}
	}
	if beforeStr := r.URL.Query().Get("before"); beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			filter.Before = t
		}
	}

	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	page, err := h.svc.Search(r.Context(), q, filter, from, size)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, page)
}

// ── helpers ───────────────────────────────────────────────────────────────────

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

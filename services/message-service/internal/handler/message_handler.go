// Package handler contains HTTP handlers for the message service.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/relay-im/relay/services/message-service/internal/domain"
	"github.com/relay-im/relay/services/message-service/internal/service"
	"github.com/relay-im/relay/shared/middleware"
)

// MessageHandler handles all message REST API endpoints.
type MessageHandler struct {
	svc *service.MessageService
}

// NewMessageHandler creates a MessageHandler.
func NewMessageHandler(svc *service.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

// ── Request shapes ────────────────────────────────────────────────────────────

type sendMessageRequest struct {
	Body           string  `json:"body"`
	ThreadID       *string `json:"thread_id"`
	ParentID       *string `json:"parent_id"`
	IdempotencyKey *string `json:"idempotency_key"`
}

type editMessageRequest struct {
	Body string `json:"body"`
}

type addReactionRequest struct {
	Emoji string `json:"emoji"`
}

// ── Message handlers ──────────────────────────────────────────────────────────

// SendMessage handles POST /channels/{channel_id}/messages
func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}

	channelID := r.PathValue("channel_id")
	if channelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "channel_id required"})
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}

	msg, err := h.svc.Send(r.Context(), channelID, claims.UserID, req.Body, req.ThreadID, req.ParentID, req.IdempotencyKey)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, messageResponse(msg))
}

// ListMessages handles GET /channels/{channel_id}/messages
func (h *MessageHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channel_id")
	if channelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "channel_id required"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")

	page, err := h.svc.List(r.Context(), channelID, limit, cursor)
	if err != nil {
		writeError(w, err)
		return
	}

	data := make([]map[string]any, 0, len(page.Messages))
	for _, m := range page.Messages {
		data = append(data, messageResponse(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":     data,
		"cursor":   page.Cursor,
		"has_more": page.HasMore,
	})
}

// GetThread handles GET /channels/{channel_id}/messages/{message_id}/thread
func (h *MessageHandler) GetThread(w http.ResponseWriter, r *http.Request) {
	messageID := r.PathValue("message_id")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")

	page, err := h.svc.GetThread(r.Context(), messageID, limit, cursor)
	if err != nil {
		writeError(w, err)
		return
	}

	data := make([]map[string]any, 0, len(page.Messages))
	for _, m := range page.Messages {
		data = append(data, messageResponse(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":     data,
		"cursor":   page.Cursor,
		"has_more": page.HasMore,
	})
}

// EditMessage handles PATCH /channels/{channel_id}/messages/{message_id}
func (h *MessageHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}

	messageID := r.PathValue("message_id")

	var req editMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}

	msg, err := h.svc.Edit(r.Context(), messageID, claims.UserID, req.Body)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, messageResponse(msg))
}

// DeleteMessage handles DELETE /channels/{channel_id}/messages/{message_id}
func (h *MessageHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}

	messageID := r.PathValue("message_id")

	if err := h.svc.Delete(r.Context(), messageID, claims.UserID); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── Reaction handlers ─────────────────────────────────────────────────────────

// AddReaction handles POST /channels/{channel_id}/messages/{message_id}/reactions
func (h *MessageHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}

	channelID := r.PathValue("channel_id")
	messageID := r.PathValue("message_id")

	var req addReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}

	if err := h.svc.AddReaction(r.Context(), messageID, channelID, claims.UserID, req.Emoji); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// RemoveReaction handles DELETE /channels/{channel_id}/messages/{message_id}/reactions/{emoji}
func (h *MessageHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}

	channelID := r.PathValue("channel_id")
	messageID := r.PathValue("message_id")
	emoji := r.PathValue("emoji")
	if emoji == "" {
		emoji = strings.TrimPrefix(r.URL.Path, "/api/v1/channels/"+channelID+"/messages/"+messageID+"/reactions/")
	}

	if err := h.svc.RemoveReaction(r.Context(), messageID, channelID, claims.UserID, emoji); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ListReactions handles GET /channels/{channel_id}/messages/{message_id}/reactions
func (h *MessageHandler) ListReactions(w http.ResponseWriter, r *http.Request) {
	messageID := r.PathValue("message_id")

	summaries, err := h.svc.ListReactions(r.Context(), messageID)
	if err != nil {
		writeError(w, err)
		return
	}

	if summaries == nil {
		summaries = []domain.ReactionSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"reactions": summaries})
}

// ── Pin handlers ──────────────────────────────────────────────────────────────

// PinMessage handles POST /channels/{channel_id}/pins/{message_id}
func (h *MessageHandler) PinMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}

	channelID := r.PathValue("channel_id")
	messageID := r.PathValue("message_id")

	if err := h.svc.PinMessage(r.Context(), channelID, messageID, claims.UserID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// UnpinMessage handles DELETE /channels/{channel_id}/pins/{message_id}
func (h *MessageHandler) UnpinMessage(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channel_id")
	messageID := r.PathValue("message_id")

	if err := h.svc.UnpinMessage(r.Context(), channelID, messageID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ListPins handles GET /channels/{channel_id}/pins
func (h *MessageHandler) ListPins(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channel_id")

	pins, err := h.svc.ListPins(r.Context(), channelID)
	if err != nil {
		writeError(w, err)
		return
	}

	data := make([]map[string]any, 0, len(pins))
	for _, p := range pins {
		data = append(data, map[string]any{
			"id":         p.ID,
			"channel_id": p.ChannelID,
			"message_id": p.MessageID,
			"pinned_by":  p.PinnedBy,
			"created_at": p.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// ── shared response helpers ───────────────────────────────────────────────────

func messageResponse(m *domain.Message) map[string]any {
	resp := map[string]any{
		"id":          m.ID,
		"channel_id":  m.ChannelID,
		"author_id":   m.AuthorID,
		"body":        m.Body,
		"body_parsed": m.BodyParsed,
		"is_edited":   m.IsEdited,
		"reply_count": m.ReplyCount,
		"created_at":  m.CreatedAt.Format(time.RFC3339),
		"updated_at":  m.UpdatedAt.Format(time.RFC3339),
	}
	if m.ThreadID != nil {
		resp["thread_id"] = *m.ThreadID
	}
	if m.ParentID != nil {
		resp["parent_id"] = *m.ParentID
	}
	return resp
}

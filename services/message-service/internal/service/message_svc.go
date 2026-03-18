// Package service contains the message service business logic.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/relay-im/relay/services/message-service/internal/domain"
	"github.com/relay-im/relay/services/message-service/internal/repository"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// MessageService orchestrates message operations and NATS event publishing.
type MessageService struct {
	repo *repository.MessageRepo
	nc   *nats.Conn
}

// NewMessageService creates a MessageService.
func NewMessageService(repo *repository.MessageRepo, nc *nats.Conn) *MessageService {
	return &MessageService{repo: repo, nc: nc}
}

// Send creates a new message in a channel.
func (s *MessageService) Send(ctx context.Context, channelID, authorID, body string, threadID, parentID, idempotencyKey *string) (*domain.Message, error) {
	if strings.TrimSpace(body) == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "message body cannot be empty")
	}
	if len(body) > 40000 {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "message body too long")
	}

	parsed := parseBody(body)

	msg := &domain.Message{
		ChannelID:      channelID,
		AuthorID:       authorID,
		Body:           body,
		BodyParsed:     parsed,
		ThreadID:       threadID,
		ParentID:       parentID,
		IdempotencyKey: idempotencyKey,
	}

	created, err := s.repo.Create(ctx, msg)
	if err != nil {
		return nil, err
	}

	// Increment reply count on the thread root when this is a reply.
	if threadID != nil && *threadID != "" {
		_ = s.repo.IncrementReplyCount(ctx, *threadID)
	}

	s.publish("message.created", map[string]any{
		"id":          created.ID,
		"channel_id":  created.ChannelID,
		"author_id":   created.AuthorID,
		"body":        created.Body,
		"body_parsed": created.BodyParsed,
		"thread_id":   created.ThreadID,
		"parent_id":   created.ParentID,
		"created_at":  created.CreatedAt.Format(time.RFC3339),
	})

	return created, nil
}

// List returns a paginated list of top-level messages for a channel.
func (s *MessageService) List(ctx context.Context, channelID string, limit int, cursor string) (*domain.Page, error) {
	return s.repo.ListByChannel(ctx, channelID, limit, cursor)
}

// GetThread returns replies to a thread.
func (s *MessageService) GetThread(ctx context.Context, threadID string, limit int, cursor string) (*domain.Page, error) {
	return s.repo.ListThread(ctx, threadID, limit, cursor)
}

// Edit updates a message body.
func (s *MessageService) Edit(ctx context.Context, messageID, authorID, body string) (*domain.Message, error) {
	if strings.TrimSpace(body) == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "message body cannot be empty")
	}
	parsed := parseBody(body)
	updated, err := s.repo.Update(ctx, messageID, authorID, body, parsed)
	if err != nil {
		return nil, err
	}

	s.publish("message.updated", map[string]any{
		"id":          updated.ID,
		"channel_id":  updated.ChannelID,
		"body":        updated.Body,
		"body_parsed": updated.BodyParsed,
		"is_edited":   true,
		"updated_at":  updated.UpdatedAt.Format(time.RFC3339),
	})

	return updated, nil
}

// Delete soft-deletes a message.
func (s *MessageService) Delete(ctx context.Context, messageID, authorID string) error {
	// Need channel_id for the event; fetch first.
	msg, err := s.repo.GetByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.AuthorID != authorID {
		return apperrors.New(apperrors.CodePermissionDenied, "cannot delete another user's message")
	}

	if err := s.repo.SoftDelete(ctx, messageID, authorID); err != nil {
		return err
	}

	s.publish("message.deleted", map[string]any{
		"id":         messageID,
		"channel_id": msg.ChannelID,
	})

	return nil
}

// AddReaction adds an emoji reaction to a message.
func (s *MessageService) AddReaction(ctx context.Context, messageID, channelID, userID, emoji string) error {
	if emoji == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "emoji is required")
	}

	rx := &domain.Reaction{
		MessageID: messageID,
		ChannelID: channelID,
		UserID:    userID,
		Emoji:     emoji,
	}
	if err := s.repo.AddReaction(ctx, rx); err != nil {
		return err
	}

	summaries, _ := s.repo.ListReactions(ctx, messageID)
	count := 0
	for _, s2 := range summaries {
		if s2.Emoji == emoji {
			count = s2.Count
			break
		}
	}

	s.publish("reaction.added", map[string]any{
		"message_id": messageID,
		"channel_id": channelID,
		"user_id":    userID,
		"emoji":      emoji,
		"count":      count,
	})

	return nil
}

// RemoveReaction removes a reaction.
func (s *MessageService) RemoveReaction(ctx context.Context, messageID, channelID, userID, emoji string) error {
	if err := s.repo.RemoveReaction(ctx, messageID, userID, emoji); err != nil {
		return err
	}

	summaries, _ := s.repo.ListReactions(ctx, messageID)
	count := 0
	for _, s2 := range summaries {
		if s2.Emoji == emoji {
			count = s2.Count
			break
		}
	}

	s.publish("reaction.removed", map[string]any{
		"message_id": messageID,
		"channel_id": channelID,
		"user_id":    userID,
		"emoji":      emoji,
		"count":      count,
	})

	return nil
}

// ListReactions returns aggregated reactions for a message.
func (s *MessageService) ListReactions(ctx context.Context, messageID string) ([]domain.ReactionSummary, error) {
	return s.repo.ListReactions(ctx, messageID)
}

// PinMessage pins a message.
func (s *MessageService) PinMessage(ctx context.Context, channelID, messageID, userID string) error {
	pin := &domain.Pin{
		ChannelID: channelID,
		MessageID: messageID,
		PinnedBy:  userID,
	}
	if err := s.repo.PinMessage(ctx, pin); err != nil {
		return err
	}
	s.publish("message.pinned", map[string]any{
		"channel_id": channelID,
		"message_id": messageID,
		"pinned_by":  userID,
	})
	return nil
}

// UnpinMessage unpins a message.
func (s *MessageService) UnpinMessage(ctx context.Context, channelID, messageID string) error {
	return s.repo.UnpinMessage(ctx, channelID, messageID)
}

// ListPins returns pinned messages for a channel.
func (s *MessageService) ListPins(ctx context.Context, channelID string) ([]*domain.Pin, error) {
	return s.repo.ListPins(ctx, channelID)
}

// GetByID retrieves a single message.
func (s *MessageService) GetByID(ctx context.Context, id string) (*domain.Message, error) {
	return s.repo.GetByID(ctx, id)
}

// ── helpers ───────────────────────────────────────────────────────────────────

var mentionRe = regexp.MustCompile(`@(\w+)`)

// parseBody converts raw markdown body into a minimal parsed representation.
// In production this would invoke a full markdown/mention parser.
func parseBody(body string) json.RawMessage {
	type parsed struct {
		Raw      string   `json:"raw"`
		Mentions []string `json:"mentions"`
	}
	matches := mentionRe.FindAllStringSubmatch(body, -1)
	mentions := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		handle := m[1]
		if !seen[handle] {
			seen[handle] = true
			mentions = append(mentions, handle)
		}
	}
	p := parsed{Raw: body, Mentions: mentions}
	b, _ := json.Marshal(p)
	return b
}

// publish serialises an event and publishes it to NATS, swallowing errors.
func (s *MessageService) publish(subject string, payload map[string]any) {
	if s.nc == nil {
		return
	}
	b, err := json.Marshal(map[string]any{
		"type":    subject,
		"payload": payload,
		"ts":      fmt.Sprintf("%d", time.Now().UnixMilli()),
	})
	if err != nil {
		return
	}
	_ = s.nc.Publish(subject, b)
}

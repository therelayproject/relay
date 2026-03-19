// Package repository provides PostgreSQL-backed data access for the message service.
package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/relay-im/relay/services/message-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// MessageRepo handles message persistence in PostgreSQL.
type MessageRepo struct {
	db *pgxpool.Pool
}

// NewMessageRepo creates a MessageRepo.
func NewMessageRepo(db *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{db: db}
}

// Create inserts a new message and returns it.
func (r *MessageRepo) Create(ctx context.Context, m *domain.Message) (*domain.Message, error) {
	var out domain.Message
	var bodyParsedRaw []byte
	err := r.db.QueryRow(ctx, `
		INSERT INTO relay_messages
			(channel_id, author_id, body, body_parsed, thread_id, parent_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, channel_id, author_id, body, body_parsed,
		          thread_id::text, parent_id::text, idempotency_key::text,
		          is_edited, is_deleted, reply_count, created_at, updated_at
	`, m.ChannelID, m.AuthorID, m.Body, nullableJSON(m.BodyParsed),
		m.ThreadID, m.ParentID, m.IdempotencyKey,
	).Scan(
		&out.ID, &out.ChannelID, &out.AuthorID, &out.Body, &bodyParsedRaw,
		&out.ThreadID, &out.ParentID, &out.IdempotencyKey,
		&out.IsEdited, &out.IsDeleted, &out.ReplyCount, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("message create: %w", err)
	}
	out.BodyParsed = bodyParsedRaw
	return &out, nil
}

// GetByID retrieves a single message.
func (r *MessageRepo) GetByID(ctx context.Context, id string) (*domain.Message, error) {
	var out domain.Message
	var bodyParsedRaw []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, channel_id, author_id, body, body_parsed,
		       thread_id::text, parent_id::text, idempotency_key::text,
		       is_edited, is_deleted, reply_count, created_at, updated_at
		FROM relay_messages
		WHERE id = $1 AND is_deleted = FALSE
	`, id).Scan(
		&out.ID, &out.ChannelID, &out.AuthorID, &out.Body, &bodyParsedRaw,
		&out.ThreadID, &out.ParentID, &out.IdempotencyKey,
		&out.IsEdited, &out.IsDeleted, &out.ReplyCount, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "message not found")
		}
		return nil, fmt.Errorf("message get: %w", err)
	}
	out.BodyParsed = bodyParsedRaw
	return &out, nil
}

// ListByChannel returns paginated messages for a channel (newest-first by default).
// cursor is a base64-encoded "<created_at_unix_nano>:<id>" string.
func (r *MessageRepo) ListByChannel(ctx context.Context, channelID string, limit int, cursor string) (*domain.Page, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var (
		cursorTime time.Time
		cursorID   string
	)
	if cursor != "" {
		t, id, err := decodeCursor(cursor)
		if err == nil {
			cursorTime = t
			cursorID = id
		}
	}

	var rows pgx.Rows
	var err error
	if cursorTime.IsZero() {
		rows, err = r.db.Query(ctx, `
			SELECT id, channel_id, author_id, body, body_parsed,
			       thread_id::text, parent_id::text, idempotency_key::text,
			       is_edited, is_deleted, reply_count, created_at, updated_at
			FROM relay_messages
			WHERE channel_id = $1 AND is_deleted = FALSE AND thread_id IS NULL
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		`, channelID, limit+1)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT id, channel_id, author_id, body, body_parsed,
			       thread_id::text, parent_id::text, idempotency_key::text,
			       is_edited, is_deleted, reply_count, created_at, updated_at
			FROM relay_messages
			WHERE channel_id = $1 AND is_deleted = FALSE AND thread_id IS NULL
			  AND (created_at, id) < ($2, $3)
			ORDER BY created_at DESC, id DESC
			LIMIT $4
		`, channelID, cursorTime, cursorID, limit+1)
	}
	if err != nil {
		return nil, fmt.Errorf("list channel messages: %w", err)
	}
	defer rows.Close()

	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}

	page := &domain.Page{}
	if len(msgs) > limit {
		page.HasMore = true
		msgs = msgs[:limit]
	}
	page.Messages = msgs
	if len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		page.Cursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

// ListThread returns replies to a thread root (oldest-first).
func (r *MessageRepo) ListThread(ctx context.Context, threadID string, limit int, cursor string) (*domain.Page, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var (
		cursorTime time.Time
		cursorID   string
	)
	if cursor != "" {
		t, id, err := decodeCursor(cursor)
		if err == nil {
			cursorTime = t
			cursorID = id
		}
	}

	var rows pgx.Rows
	var err error
	if cursorTime.IsZero() {
		rows, err = r.db.Query(ctx, `
			SELECT id, channel_id, author_id, body, body_parsed,
			       thread_id::text, parent_id::text, idempotency_key::text,
			       is_edited, is_deleted, reply_count, created_at, updated_at
			FROM relay_messages
			WHERE thread_id = $1 AND is_deleted = FALSE
			ORDER BY created_at ASC, id ASC
			LIMIT $2
		`, threadID, limit+1)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT id, channel_id, author_id, body, body_parsed,
			       thread_id::text, parent_id::text, idempotency_key::text,
			       is_edited, is_deleted, reply_count, created_at, updated_at
			FROM relay_messages
			WHERE thread_id = $1 AND is_deleted = FALSE
			  AND (created_at, id) > ($2, $3)
			ORDER BY created_at ASC, id ASC
			LIMIT $4
		`, threadID, cursorTime, cursorID, limit+1)
	}
	if err != nil {
		return nil, fmt.Errorf("list thread: %w", err)
	}
	defer rows.Close()

	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}

	page := &domain.Page{}
	if len(msgs) > limit {
		page.HasMore = true
		msgs = msgs[:limit]
	}
	page.Messages = msgs
	if len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		page.Cursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

// Update edits a message body.
func (r *MessageRepo) Update(ctx context.Context, id, authorID, body string, bodyParsed json.RawMessage) (*domain.Message, error) {
	var out domain.Message
	var bodyParsedRaw []byte
	err := r.db.QueryRow(ctx, `
		UPDATE relay_messages
		SET body = $1, body_parsed = $2, is_edited = TRUE, updated_at = NOW()
		WHERE id = $3 AND author_id = $4 AND is_deleted = FALSE
		RETURNING id, channel_id, author_id, body, body_parsed,
		          thread_id::text, parent_id::text, idempotency_key::text,
		          is_edited, is_deleted, reply_count, created_at, updated_at
	`, body, nullableJSON(bodyParsed), id, authorID,
	).Scan(
		&out.ID, &out.ChannelID, &out.AuthorID, &out.Body, &bodyParsedRaw,
		&out.ThreadID, &out.ParentID, &out.IdempotencyKey,
		&out.IsEdited, &out.IsDeleted, &out.ReplyCount, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodePermissionDenied, "message not found or not authored by you")
		}
		return nil, fmt.Errorf("message update: %w", err)
	}
	out.BodyParsed = bodyParsedRaw
	return &out, nil
}

// SoftDelete marks a message as deleted.
func (r *MessageRepo) SoftDelete(ctx context.Context, id, authorID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE relay_messages
		SET is_deleted = TRUE, updated_at = NOW()
		WHERE id = $1 AND author_id = $2 AND is_deleted = FALSE
	`, id, authorID)
	if err != nil {
		return fmt.Errorf("message delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodePermissionDenied, "message not found or not authored by you")
	}
	return nil
}

// IncrementReplyCount bumps reply_count on a root/thread message.
func (r *MessageRepo) IncrementReplyCount(ctx context.Context, messageID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE relay_messages SET reply_count = reply_count + 1 WHERE id = $1`,
		messageID)
	return err
}

// ── Reactions ────────────────────────────────────────────────────────────────

// AddReaction inserts a reaction; returns ALREADY_EXISTS if duplicate.
func (r *MessageRepo) AddReaction(ctx context.Context, rx *domain.Reaction) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO relay_reactions (message_id, channel_id, user_id, emoji)
		VALUES ($1, $2, $3, $4)
	`, rx.MessageID, rx.ChannelID, rx.UserID, rx.Emoji)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.New(apperrors.CodeAlreadyExists, "already reacted")
		}
		return fmt.Errorf("add reaction: %w", err)
	}
	return nil
}

// RemoveReaction deletes a reaction.
func (r *MessageRepo) RemoveReaction(ctx context.Context, messageID, userID, emoji string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM relay_reactions WHERE message_id=$1 AND user_id=$2 AND emoji=$3`,
		messageID, userID, emoji)
	if err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "reaction not found")
	}
	return nil
}

// ListReactions returns aggregated reactions for a message.
func (r *MessageRepo) ListReactions(ctx context.Context, messageID string) ([]domain.ReactionSummary, error) {
	rows, err := r.db.Query(ctx, `
		SELECT emoji, COUNT(*) AS cnt, ARRAY_AGG(user_id::text) AS user_ids
		FROM relay_reactions
		WHERE message_id = $1
		GROUP BY emoji
		ORDER BY emoji
	`, messageID)
	if err != nil {
		return nil, fmt.Errorf("list reactions: %w", err)
	}
	defer rows.Close()

	var out []domain.ReactionSummary
	for rows.Next() {
		var s domain.ReactionSummary
		if err := rows.Scan(&s.Emoji, &s.Count, &s.UserIDs); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ── Pins ─────────────────────────────────────────────────────────────────────

// PinMessage pins a message in a channel.
func (r *MessageRepo) PinMessage(ctx context.Context, pin *domain.Pin) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO relay_pins (channel_id, message_id, pinned_by)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, pin.ChannelID, pin.MessageID, pin.PinnedBy)
	return err
}

// UnpinMessage removes a pin.
func (r *MessageRepo) UnpinMessage(ctx context.Context, channelID, messageID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM relay_pins WHERE channel_id=$1 AND message_id=$2`,
		channelID, messageID)
	return err
}

// ListPins returns all pins for a channel.
func (r *MessageRepo) ListPins(ctx context.Context, channelID string) ([]*domain.Pin, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, channel_id, message_id, pinned_by, created_at
		FROM relay_pins
		WHERE channel_id = $1
		ORDER BY created_at DESC
	`, channelID)
	if err != nil {
		return nil, fmt.Errorf("list pins: %w", err)
	}
	defer rows.Close()
	var pins []*domain.Pin
	for rows.Next() {
		var p domain.Pin
		if err := rows.Scan(&p.ID, &p.ChannelID, &p.MessageID, &p.PinnedBy, &p.CreatedAt); err != nil {
			return nil, err
		}
		pins = append(pins, &p)
	}
	return pins, rows.Err()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanMessages(rows pgx.Rows) ([]*domain.Message, error) {
	var msgs []*domain.Message
	for rows.Next() {
		var m domain.Message
		var bodyParsedRaw []byte
		if err := rows.Scan(
			&m.ID, &m.ChannelID, &m.AuthorID, &m.Body, &bodyParsedRaw,
			&m.ThreadID, &m.ParentID, &m.IdempotencyKey,
			&m.IsEdited, &m.IsDeleted, &m.ReplyCount, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		m.BodyParsed = bodyParsedRaw
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

func encodeCursor(t time.Time, id string) string {
	raw := fmt.Sprintf("%d:%s", t.UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	var nano int64
	if _, err := fmt.Sscanf(parts[0], "%d", &nano); err != nil {
		return time.Time{}, "", err
	}
	return time.Unix(0, nano).UTC(), parts[1], nil
}

func nullableJSON(v json.RawMessage) interface{} {
	if len(v) == 0 {
		return nil
	}
	return v
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "23505") || strings.Contains(s, "unique constraint")
}

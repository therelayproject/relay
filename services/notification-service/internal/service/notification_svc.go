// Package service contains the notification service business logic.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	nats "github.com/nats-io/nats.go"
	"github.com/relay-im/relay/services/notification-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
	"github.com/rs/zerolog"
)

// NotificationService manages preferences, DND, push tokens, and in-app notifications.
type NotificationService struct {
	db  *pgxpool.Pool
	nc  *nats.Conn
	log zerolog.Logger
}

// New creates a NotificationService.
func New(db *pgxpool.Pool, nc *nats.Conn, log zerolog.Logger) *NotificationService {
	return &NotificationService{db: db, nc: nc, log: log}
}

// ── Preferences ───────────────────────────────────────────────────────────────

// GetPreferences returns all preferences for a user.
func (s *NotificationService) GetPreferences(ctx context.Context, userID string) ([]domain.Preference, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, scope, level, muted, created_at, updated_at
		FROM relay_notification_preferences
		WHERE user_id = $1
		ORDER BY scope
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	defer rows.Close()

	var prefs []domain.Preference
	for rows.Next() {
		var p domain.Preference
		if err := rows.Scan(&p.ID, &p.UserID, &p.Scope, &p.Level, &p.Muted, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	if prefs == nil {
		prefs = []domain.Preference{}
	}
	return prefs, rows.Err()
}

// UpsertPreference creates or updates a preference for a scope.
func (s *NotificationService) UpsertPreference(ctx context.Context, userID, scope string, level domain.Level, muted bool) (*domain.Preference, error) {
	var p domain.Preference
	err := s.db.QueryRow(ctx, `
		INSERT INTO relay_notification_preferences (user_id, scope, level, muted)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, scope)
		DO UPDATE SET level = EXCLUDED.level, muted = EXCLUDED.muted, updated_at = NOW()
		RETURNING id, user_id, scope, level, muted, created_at, updated_at
	`, userID, scope, level, muted).Scan(&p.ID, &p.UserID, &p.Scope, &p.Level, &p.Muted, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert preference: %w", err)
	}
	return &p, nil
}

// ── Push tokens ───────────────────────────────────────────────────────────────

// UpsertPushToken registers or replaces a push token.
func (s *NotificationService) UpsertPushToken(ctx context.Context, userID, platform, token string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO relay_push_tokens (user_id, platform, token)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, platform)
		DO UPDATE SET token = EXCLUDED.token
	`, userID, platform, token)
	return err
}

// DeletePushToken removes a push token for a platform.
func (s *NotificationService) DeletePushToken(ctx context.Context, userID, platform string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM relay_push_tokens WHERE user_id = $1 AND platform = $2`,
		userID, platform)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "push token not found")
	}
	return nil
}

// ── DND ───────────────────────────────────────────────────────────────────────

// SetDND enables Do Not Disturb until the given time.
func (s *NotificationService) SetDND(ctx context.Context, userID string, until time.Time) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO relay_dnd (user_id, until)
		VALUES ($1, $2)
		ON CONFLICT (user_id)
		DO UPDATE SET until = EXCLUDED.until
	`, userID, until)
	return err
}

// GetDND returns the user's active DND period, or nil if not set / expired.
func (s *NotificationService) GetDND(ctx context.Context, userID string) (*domain.DND, error) {
	var d domain.DND
	err := s.db.QueryRow(ctx,
		`SELECT user_id, until, created_at FROM relay_dnd WHERE user_id = $1 AND until > NOW()`,
		userID).Scan(&d.UserID, &d.Until, &d.CreatedAt)
	if err != nil {
		return nil, nil // not in DND
	}
	return &d, nil
}

// ── In-app notifications ──────────────────────────────────────────────────────

// CreateNotification inserts an in-app notification and publishes it via NATS.
func (s *NotificationService) CreateNotification(ctx context.Context, n *domain.Notification) (*domain.Notification, error) {
	var out domain.Notification
	err := s.db.QueryRow(ctx, `
		INSERT INTO relay_notifications (user_id, type, title, body, action_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, type, title, body, action_url, read_at, created_at
	`, n.UserID, n.Type, n.Title, n.Body, n.ActionURL,
	).Scan(&out.ID, &out.UserID, &out.Type, &out.Title, &out.Body, &out.ActionURL, &out.ReadAt, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}

	// Fan out via NATS so ws-gateway can deliver it in real time.
	s.publish("notification", map[string]any{
		"id":         out.ID,
		"user_id":    out.UserID,
		"type":       out.Type,
		"title":      out.Title,
		"body":       out.Body,
		"action_url": out.ActionURL,
	})

	return &out, nil
}

// ── NATS consumer ─────────────────────────────────────────────────────────────

// StartConsumer listens on message.created and creates mention notifications.
func (s *NotificationService) StartConsumer(ctx context.Context) error {
	sub, err := s.nc.Subscribe("message.created", func(msg *nats.Msg) {
		s.handleMessageCreated(ctx, msg.Data)
	})
	if err != nil {
		return fmt.Errorf("subscribe message.created: %w", err)
	}
	go func() {
		<-ctx.Done()
		_ = sub.Unsubscribe()
	}()
	return nil
}

func (s *NotificationService) handleMessageCreated(ctx context.Context, data []byte) {
	var env struct {
		Payload struct {
			ChannelID  string `json:"channel_id"`
			AuthorID   string `json:"author_id"`
			BodyParsed struct {
				Mentions []string `json:"mentions"`
			} `json:"body_parsed"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}

	// For each mentioned user, create a notification.
	for _, handle := range env.Payload.BodyParsed.Mentions {
		// In production we'd resolve handle -> userID via user-service gRPC.
		// For now, emit a notification event with the handle as a placeholder.
		n := &domain.Notification{
			UserID:    handle, // placeholder until user resolution is wired
			Type:      "mention",
			Title:     "You were mentioned",
			Body:      fmt.Sprintf("Someone mentioned @%s in a channel", handle),
			ActionURL: fmt.Sprintf("/channels/%s", env.Payload.ChannelID),
		}
		if _, err := s.CreateNotification(ctx, n); err != nil {
			s.log.Error().Err(err).Str("handle", handle).Msg("failed to create mention notification")
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (s *NotificationService) publish(subject string, payload map[string]any) {
	if s.nc == nil {
		return
	}
	b, _ := json.Marshal(map[string]any{
		"type":    subject,
		"payload": payload,
		"ts":      fmt.Sprintf("%d", time.Now().UnixMilli()),
	})
	_ = s.nc.Publish(subject, b)
}

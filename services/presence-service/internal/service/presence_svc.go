// Package service contains the presence service business logic.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/relay-im/relay/services/presence-service/internal/domain"
)

const (
	// onlineTTL is how long the online key lives without a heartbeat.
	onlineTTL = 90 * time.Second
	// awayTTL controls how long "away" is set after the online key expires.
	awayTTL = 5 * time.Minute
)

// PresenceService manages online/away/offline state in Redis.
type PresenceService struct {
	rdb *redis.Client
	nc  *nats.Conn
}

// NewPresenceService creates a PresenceService.
func NewPresenceService(rdb *redis.Client, nc *nats.Conn) *PresenceService {
	return &PresenceService{rdb: rdb, nc: nc}
}

// Heartbeat records that a user is online in a workspace.
// Call every 30 s from the client; TTL is 90 s, so one missed beat won't flip to away.
func (s *PresenceService) Heartbeat(ctx context.Context, userID, workspaceID string) error {
	onlineKey := onlineKey(userID, workspaceID)
	awayKey := awayKey(userID, workspaceID)

	now := time.Now().UTC()
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, onlineKey, now.Unix(), onlineTTL)
	pipe.Set(ctx, awayKey, now.Unix(), onlineTTL+awayTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("presence heartbeat: %w", err)
	}

	s.publishChange(userID, domain.StatusOnline, now)
	return nil
}

// GetStatus returns the current presence for a single user in a workspace.
func (s *PresenceService) GetStatus(ctx context.Context, userID, workspaceID string) domain.Presence {
	onlineKey := onlineKey(userID, workspaceID)
	awayKey := awayKey(userID, workspaceID)

	onlineTs, err := s.rdb.Get(ctx, onlineKey).Int64()
	if err == nil {
		return domain.Presence{
			UserID:     userID,
			Status:     domain.StatusOnline,
			LastSeenAt: time.Unix(onlineTs, 0).UTC(),
		}
	}

	awayTs, err := s.rdb.Get(ctx, awayKey).Int64()
	if err == nil {
		return domain.Presence{
			UserID:     userID,
			Status:     domain.StatusAway,
			LastSeenAt: time.Unix(awayTs, 0).UTC(),
		}
	}

	return domain.Presence{
		UserID:     userID,
		Status:     domain.StatusOffline,
		LastSeenAt: time.Time{},
	}
}

// BulkPresence returns presence for a list of user IDs in a workspace.
func (s *PresenceService) BulkPresence(ctx context.Context, userIDs []string, workspaceID string) (map[string]domain.Presence, error) {
	result := make(map[string]domain.Presence, len(userIDs))
	for _, uid := range userIDs {
		result[uid] = s.GetStatus(ctx, uid, workspaceID)
	}
	return result, nil
}

// WorkspacePresence returns all known presence for users in a workspace.
// It uses a Redis SCAN over the workspace presence key namespace.
func (s *PresenceService) WorkspacePresence(ctx context.Context, workspaceID string) (map[string]string, error) {
	pattern := fmt.Sprintf("presence:online:%s:*", workspaceID)
	awayPattern := fmt.Sprintf("presence:away:%s:*", workspaceID)

	result := make(map[string]string)

	// Collect online users.
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("presence scan: %w", err)
		}
		for _, k := range keys {
			uid := userIDFromKey(k, "presence:online:"+workspaceID+":")
			if uid != "" {
				result[uid] = string(domain.StatusOnline)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	// Collect away users (not already online).
	cursor = 0
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, awayPattern, 100).Result()
		if err != nil {
			break
		}
		for _, k := range keys {
			uid := userIDFromKey(k, "presence:away:"+workspaceID+":")
			if uid != "" {
				if _, alreadyOnline := result[uid]; !alreadyOnline {
					result[uid] = string(domain.StatusAway)
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	return result, nil
}

// SetCustomStatus stores the user's custom emoji+text status in Redis (PRES-02).
// If expiresAt is non-nil the key is given a TTL; otherwise it persists until cleared.
func (s *PresenceService) SetCustomStatus(ctx context.Context, userID, emoji, text string, expiresAt *time.Time) error {
	key := fmt.Sprintf("custom_status:%s", userID)
	now := time.Now().UTC()
	val, err := json.Marshal(map[string]any{
		"emoji":      emoji,
		"text":       text,
		"expires_at": expiresAt,
		"updated_at": now.Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("marshal custom status: %w", err)
	}
	var ttl time.Duration
	if expiresAt != nil {
		ttl = time.Until(*expiresAt)
		if ttl <= 0 {
			return fmt.Errorf("expires_at is in the past")
		}
	}
	if err := s.rdb.Set(ctx, key, val, ttl).Err(); err != nil {
		return fmt.Errorf("set custom status: %w", err)
	}
	return nil
}

// GetCustomStatus retrieves the user's custom status, returning nil if unset.
func (s *PresenceService) GetCustomStatus(ctx context.Context, userID string) (*domain.CustomStatus, error) {
	key := fmt.Sprintf("custom_status:%s", userID)
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get custom status: %w", err)
	}

	var m struct {
		Emoji     string     `json:"emoji"`
		Text      string     `json:"text"`
		ExpiresAt *time.Time `json:"expires_at"`
		UpdatedAt string     `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal custom status: %w", err)
	}
	updatedAt, _ := time.Parse(time.RFC3339, m.UpdatedAt)
	return &domain.CustomStatus{
		UserID:    userID,
		Emoji:     m.Emoji,
		Text:      m.Text,
		ExpiresAt: m.ExpiresAt,
		UpdatedAt: updatedAt,
	}, nil
}

// ClearCustomStatus removes the user's custom status.
func (s *PresenceService) ClearCustomStatus(ctx context.Context, userID string) error {
	return s.rdb.Del(ctx, fmt.Sprintf("custom_status:%s", userID)).Err()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func onlineKey(userID, workspaceID string) string {
	return fmt.Sprintf("presence:online:%s:%s", workspaceID, userID)
}

func awayKey(userID, workspaceID string) string {
	return fmt.Sprintf("presence:away:%s:%s", workspaceID, userID)
}

func userIDFromKey(key, prefix string) string {
	if len(key) > len(prefix) {
		return key[len(prefix):]
	}
	return ""
}

func (s *PresenceService) publishChange(userID string, status domain.Status, at time.Time) {
	if s.nc == nil {
		return
	}
	b, err := json.Marshal(map[string]any{
		"type": "presence.changed",
		"payload": map[string]any{
			"user_id":      userID,
			"status":       status,
			"last_seen_at": at.Format(time.RFC3339),
		},
		"ts": fmt.Sprintf("%d", at.UnixMilli()),
	})
	if err != nil {
		return
	}
	_ = s.nc.Publish("presence.changed", b)
}

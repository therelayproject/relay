// Package repository provides PostgreSQL-backed data access for the channel service.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/relay-im/relay/services/channel-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// ChannelRepo handles channel and channel-member CRUD in relay_channels.
type ChannelRepo struct {
	db *pgxpool.Pool
}

// NewChannelRepo constructs a ChannelRepo backed by the given pool.
func NewChannelRepo(db *pgxpool.Pool) *ChannelRepo {
	return &ChannelRepo{db: db}
}

// ── Channel operations ────────────────────────────────────────────────────────

// Create inserts a new channel and returns it with the generated ID.
func (r *ChannelRepo) Create(
	ctx context.Context,
	workspaceID, name, slug, description, channelType, createdBy string,
) (*domain.Channel, error) {
	var c domain.Channel
	err := r.db.QueryRow(ctx, `
		INSERT INTO channels (workspace_id, name, slug, description, type, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workspace_id, name, slug, COALESCE(description,''), COALESCE(topic,''),
		          type, is_archived, created_by, member_count, created_at, updated_at
	`, workspaceID, name, slug, nullableString(description), channelType, createdBy).Scan(
		&c.ID, &c.WorkspaceID, &c.Name, &c.Slug, &c.Description, &c.Topic,
		&c.Type, &c.IsArchived, &c.CreatedBy, &c.MemberCount, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "channel slug already exists in workspace")
		}
		return nil, fmt.Errorf("channel create: %w", err)
	}
	return &c, nil
}

// GetByID retrieves a channel by its primary key.
func (r *ChannelRepo) GetByID(ctx context.Context, id string) (*domain.Channel, error) {
	var c domain.Channel
	err := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, name, slug, COALESCE(description,''), COALESCE(topic,''),
		       type, is_archived, created_by, member_count, created_at, updated_at
		FROM channels WHERE id = $1
	`, id).Scan(
		&c.ID, &c.WorkspaceID, &c.Name, &c.Slug, &c.Description, &c.Topic,
		&c.Type, &c.IsArchived, &c.CreatedBy, &c.MemberCount, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "channel not found")
		}
		return nil, fmt.Errorf("channel get by id: %w", err)
	}
	return &c, nil
}

// GetBySlug retrieves a channel by workspace ID and slug.
func (r *ChannelRepo) GetBySlug(ctx context.Context, workspaceID, slug string) (*domain.Channel, error) {
	var c domain.Channel
	err := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, name, slug, COALESCE(description,''), COALESCE(topic,''),
		       type, is_archived, created_by, member_count, created_at, updated_at
		FROM channels WHERE workspace_id = $1 AND slug = $2
	`, workspaceID, slug).Scan(
		&c.ID, &c.WorkspaceID, &c.Name, &c.Slug, &c.Description, &c.Topic,
		&c.Type, &c.IsArchived, &c.CreatedBy, &c.MemberCount, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "channel not found")
		}
		return nil, fmt.Errorf("channel get by slug: %w", err)
	}
	return &c, nil
}

// ListByWorkspace returns channels in a workspace.
// If includePrivate is false only public channels are returned.
// If includePrivate is true, public channels plus private channels where userID is a member are returned.
func (r *ChannelRepo) ListByWorkspace(
	ctx context.Context,
	workspaceID string,
	includePrivate bool,
	userID string,
) ([]domain.Channel, error) {
	var rows pgx.Rows
	var err error

	if includePrivate {
		rows, err = r.db.Query(ctx, `
			SELECT DISTINCT c.id, c.workspace_id, c.name, c.slug, COALESCE(c.description,''), COALESCE(c.topic,''),
			       c.type, c.is_archived, c.created_by, c.member_count, c.created_at, c.updated_at
			FROM channels c
			LEFT JOIN channel_members cm ON cm.channel_id = c.id AND cm.user_id = $2
			WHERE c.workspace_id = $1
			  AND c.is_archived = false
			  AND (c.type = 'public' OR (c.type = 'private' AND cm.user_id IS NOT NULL) OR c.type = 'dm')
			ORDER BY c.name
		`, workspaceID, userID)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT id, workspace_id, name, slug, COALESCE(description,''), COALESCE(topic,''),
			       type, is_archived, created_by, member_count, created_at, updated_at
			FROM channels
			WHERE workspace_id = $1 AND type = 'public' AND is_archived = false
			ORDER BY name
		`, workspaceID)
	}
	if err != nil {
		return nil, fmt.Errorf("channel list by workspace: %w", err)
	}
	defer rows.Close()
	return scanChannels(rows)
}

// ListByMember returns all non-archived channels a user belongs to.
func (r *ChannelRepo) ListByMember(ctx context.Context, userID string) ([]domain.Channel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.workspace_id, c.name, c.slug, COALESCE(c.description,''), COALESCE(c.topic,''),
		       c.type, c.is_archived, c.created_by, c.member_count, c.created_at, c.updated_at
		FROM channels c
		JOIN channel_members cm ON cm.channel_id = c.id
		WHERE cm.user_id = $1 AND c.is_archived = false
		ORDER BY c.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("channel list by member: %w", err)
	}
	defer rows.Close()
	return scanChannels(rows)
}

// Update sets name, description, and topic on a channel.
func (r *ChannelRepo) Update(ctx context.Context, id, name, description, topic string) (*domain.Channel, error) {
	var c domain.Channel
	err := r.db.QueryRow(ctx, `
		UPDATE channels
		SET name = $2, description = $3, topic = $4
		WHERE id = $1
		RETURNING id, workspace_id, name, slug, COALESCE(description,''), COALESCE(topic,''),
		          type, is_archived, created_by, member_count, created_at, updated_at
	`, id, name, nullableString(description), nullableString(topic)).Scan(
		&c.ID, &c.WorkspaceID, &c.Name, &c.Slug, &c.Description, &c.Topic,
		&c.Type, &c.IsArchived, &c.CreatedBy, &c.MemberCount, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "channel not found")
		}
		return nil, fmt.Errorf("channel update: %w", err)
	}
	return &c, nil
}

// Archive marks a channel as archived.
func (r *ChannelRepo) Archive(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `UPDATE channels SET is_archived = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("channel archive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "channel not found")
	}
	return nil
}

// ── Member operations ─────────────────────────────────────────────────────────

// AddMember inserts a membership row. Returns the new ChannelMember.
func (r *ChannelRepo) AddMember(
	ctx context.Context,
	channelID, userID, role string,
) (*domain.ChannelMember, error) {
	var m domain.ChannelMember
	err := r.db.QueryRow(ctx, `
		INSERT INTO channel_members (channel_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING channel_id, user_id, role, last_read_at, joined_at
	`, channelID, userID, role).Scan(
		&m.ChannelID, &m.UserID, &m.Role, &m.LastReadAt, &m.JoinedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "user is already a member of this channel")
		}
		return nil, fmt.Errorf("channel add member: %w", err)
	}
	return &m, nil
}

// RemoveMember deletes a membership row.
func (r *ChannelRepo) RemoveMember(ctx context.Context, channelID, userID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM channel_members WHERE channel_id = $1 AND user_id = $2
	`, channelID, userID)
	if err != nil {
		return fmt.Errorf("channel remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "membership not found")
	}
	return nil
}

// GetMember retrieves a single membership record.
func (r *ChannelRepo) GetMember(ctx context.Context, channelID, userID string) (*domain.ChannelMember, error) {
	var m domain.ChannelMember
	err := r.db.QueryRow(ctx, `
		SELECT channel_id, user_id, role, last_read_at, joined_at
		FROM channel_members WHERE channel_id = $1 AND user_id = $2
	`, channelID, userID).Scan(
		&m.ChannelID, &m.UserID, &m.Role, &m.LastReadAt, &m.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "membership not found")
		}
		return nil, fmt.Errorf("channel get member: %w", err)
	}
	return &m, nil
}

// ListMembers returns all members of a channel.
func (r *ChannelRepo) ListMembers(ctx context.Context, channelID string) ([]domain.ChannelMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT channel_id, user_id, role, last_read_at, joined_at
		FROM channel_members WHERE channel_id = $1
		ORDER BY joined_at
	`, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel list members: %w", err)
	}
	defer rows.Close()

	var members []domain.ChannelMember
	for rows.Next() {
		var m domain.ChannelMember
		if err := rows.Scan(&m.ChannelID, &m.UserID, &m.Role, &m.LastReadAt, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("channel list members scan: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// UpdateLastRead sets last_read_at to now() for a membership.
func (r *ChannelRepo) UpdateLastRead(ctx context.Context, channelID, userID string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		UPDATE channel_members SET last_read_at = $3 WHERE channel_id = $1 AND user_id = $2
	`, channelID, userID, now)
	if err != nil {
		return fmt.Errorf("channel update last read: %w", err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanChannels(rows pgx.Rows) ([]domain.Channel, error) {
	var channels []domain.Channel
	for rows.Next() {
		var c domain.Channel
		if err := rows.Scan(
			&c.ID, &c.WorkspaceID, &c.Name, &c.Slug, &c.Description, &c.Topic,
			&c.Type, &c.IsArchived, &c.CreatedBy, &c.MemberCount, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("channel scan: %w", err)
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

// nullableString converts an empty string to nil so the DB stores NULL.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// isUniqueViolation detects PostgreSQL unique-constraint errors (code 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique constraint")
}

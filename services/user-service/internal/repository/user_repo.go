// Package repository provides PostgreSQL-backed data access for the user service.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/relay-im/relay/services/user-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// scanCols is the ordered column list used by all SELECT / RETURNING queries.
const scanCols = `user_id, display_name,
	COALESCE(avatar_url, ''), COALESCE(timezone, 'UTC'), COALESCE(locale, 'en'),
	COALESCE(status_emoji, ''), COALESCE(status_text, ''),
	status_expires_at, is_active, deactivated_at,
	created_at, updated_at`

// UserRepo handles CRUD for user_profiles.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo constructs a UserRepo backed by the given connection pool.
func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// scanProfile reads a single Profile from a pgx row.
func scanProfile(row pgx.Row) (*domain.Profile, error) {
	var p domain.Profile
	if err := row.Scan(
		&p.UserID, &p.DisplayName, &p.AvatarURL,
		&p.Timezone, &p.Locale,
		&p.StatusEmoji, &p.StatusText,
		&p.StatusExpiresAt, &p.IsActive, &p.DeactivatedAt,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "user not found")
		}
		return nil, fmt.Errorf("scan profile: %w", err)
	}
	return &p, nil
}

// Create inserts a new profile row for the given auth-service user UUID.
func (r *UserRepo) Create(ctx context.Context, userID, displayName, _ string) (*domain.Profile, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO user_profiles (user_id, display_name)
		VALUES ($1, $2)
		RETURNING `+scanCols,
		userID, displayName,
	)
	p, err := scanProfile(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "profile already exists for user")
		}
		return nil, fmt.Errorf("user create: %w", err)
	}
	return p, nil
}

// GetByID returns the profile for the given auth-service user UUID.
func (r *UserRepo) GetByID(ctx context.Context, userID string) (*domain.Profile, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+scanCols+`
		FROM user_profiles
		WHERE user_id = $1
	`, userID)
	return scanProfile(row)
}

// GetByIDs returns profiles for multiple user IDs in a single query.
func (r *UserRepo) GetByIDs(ctx context.Context, userIDs []string) ([]domain.Profile, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(userIDs))
	args := make([]any, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := `SELECT ` + scanCols + ` FROM user_profiles WHERE user_id IN (` +
		strings.Join(placeholders, ",") + `)`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("user get by ids: %w", err)
	}
	defer rows.Close()
	var profiles []domain.Profile
	for rows.Next() {
		var p domain.Profile
		if err := rows.Scan(
			&p.UserID, &p.DisplayName, &p.AvatarURL,
			&p.Timezone, &p.Locale,
			&p.StatusEmoji, &p.StatusText,
			&p.StatusExpiresAt, &p.IsActive, &p.DeactivatedAt,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan profile row: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// UpdateProfile applies display_name, avatar_url, and timezone changes.
func (r *UserRepo) UpdateProfile(ctx context.Context, userID, displayName, avatarURL, timezone string) (*domain.Profile, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE user_profiles
		SET display_name = $2,
		    avatar_url   = NULLIF($3, ''),
		    timezone     = NULLIF($4, '')
		WHERE user_id = $1
		RETURNING `+scanCols,
		userID, displayName, avatarURL, timezone,
	)
	p, err := scanProfile(row)
	if err != nil {
		return nil, fmt.Errorf("user update profile: %w", err)
	}
	return p, nil
}

// SetStatus updates status fields for a user.
func (r *UserRepo) SetStatus(ctx context.Context, userID, statusText, statusEmoji string, expiresAt *time.Time) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE user_profiles
		SET status_text       = NULLIF($2, ''),
		    status_emoji      = NULLIF($3, ''),
		    status_expires_at = $4
		WHERE user_id = $1
	`, userID, statusText, statusEmoji, expiresAt)
	if err != nil {
		return fmt.Errorf("user set status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "user not found")
	}
	return nil
}

// SetAvatarURL updates the avatar_url field directly (called after MinIO upload).
func (r *UserRepo) SetAvatarURL(ctx context.Context, userID, avatarURL string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE user_profiles SET avatar_url = $2 WHERE user_id = $1
	`, userID, avatarURL)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "user not found")
	}
	return nil
}

// isUniqueViolation returns true for PostgreSQL unique-constraint errors (code 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique constraint")
}

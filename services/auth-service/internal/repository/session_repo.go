package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/relay-im/relay/services/auth-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// SessionRepo manages device sessions in PostgreSQL.
type SessionRepo struct {
	db *pgxpool.Pool
}

func NewSessionRepo(db *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create inserts a new session and returns it.
func (r *SessionRepo) Create(ctx context.Context, userID, deviceName, userAgent, ip string) (*domain.Session, error) {
	// ip_address is inet type — pass nil when empty to avoid casting "" to inet.
	var ipParam any
	if ip != "" {
		ipParam = ip
	}
	var s domain.Session
	var ipOut *string
	err := r.db.QueryRow(ctx, `
		INSERT INTO sessions (user_id, device_name, user_agent, ip_address)
		VALUES ($1, $2, $3, $4::inet)
		RETURNING id, user_id, device_name, user_agent, ip_address::text, last_seen_at, created_at
	`, userID, deviceName, userAgent, ipParam).Scan(
		&s.ID, &s.UserID, &s.DeviceName, &s.UserAgent, &ipOut, &s.LastSeenAt, &s.CreatedAt,
	)
	if ipOut != nil {
		s.IPAddress = *ipOut
	}
	if err != nil {
		return nil, fmt.Errorf("session create: %w", err)
	}
	return &s, nil
}

// GetByID retrieves a session by its primary key.
func (r *SessionRepo) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	var s domain.Session
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, device_name, user_agent, ip_address::text, last_seen_at, created_at
		FROM sessions WHERE id = $1
	`, id).Scan(
		&s.ID, &s.UserID, &s.DeviceName, &s.UserAgent, &s.IPAddress, &s.LastSeenAt, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "session not found")
		}
		return nil, fmt.Errorf("session get: %w", err)
	}
	return &s, nil
}

// ListByUser returns all active sessions for a user.
func (r *SessionRepo) ListByUser(ctx context.Context, userID string) ([]domain.Session, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, device_name, user_agent, ip_address::text, last_seen_at, created_at
		FROM sessions WHERE user_id = $1
		ORDER BY last_seen_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		var s domain.Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.DeviceName, &s.UserAgent, &s.IPAddress, &s.LastSeenAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// Touch updates last_seen_at to now.
func (r *SessionRepo) Touch(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET last_seen_at = $1 WHERE id = $2`, time.Now(), id)
	return err
}

// Delete removes a session (logout).
func (r *SessionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

// DeleteByUser removes all sessions for a user (e.g., deactivation).
func (r *SessionRepo) DeleteByUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

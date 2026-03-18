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

// TokenRepo manages password reset and email verification tokens.
type TokenRepo struct {
	db *pgxpool.Pool
}

func NewTokenRepo(db *pgxpool.Pool) *TokenRepo {
	return &TokenRepo{db: db}
}

// CreatePasswordResetToken inserts a new password reset token.
func (r *TokenRepo) CreatePasswordResetToken(ctx context.Context, token, userID string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO password_reset_tokens (token, user_id, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (token) DO NOTHING
	`, token, userID, expiresAt)
	return err
}

// GetPasswordResetToken retrieves a valid (unused, unexpired) password reset token.
func (r *TokenRepo) GetPasswordResetToken(ctx context.Context, token string) (*domain.PasswordResetToken, error) {
	var t domain.PasswordResetToken
	err := r.db.QueryRow(ctx, `
		SELECT token, user_id, expires_at, used_at
		FROM password_reset_tokens
		WHERE token = $1 AND used_at IS NULL AND expires_at > now()
	`, token).Scan(&t.Token, &t.UserID, &t.ExpiresAt, &t.UsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeInvalidArgument, "invalid or expired reset token")
		}
		return nil, fmt.Errorf("get reset token: %w", err)
	}
	return &t, nil
}

// ConsumePasswordResetToken marks the token as used.
func (r *TokenRepo) ConsumePasswordResetToken(ctx context.Context, token string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE password_reset_tokens SET used_at = now()
		WHERE token = $1 AND used_at IS NULL AND expires_at > now()
	`, token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeInvalidArgument, "token already used or expired")
	}
	return nil
}

// CreateEmailVerificationToken inserts an email verification token.
func (r *TokenRepo) CreateEmailVerificationToken(ctx context.Context, token, userID string, expiresAt time.Time) error {
	// Delete any existing token for the user before inserting a new one.
	_, _ = r.db.Exec(ctx, `DELETE FROM email_verification_tokens WHERE user_id = $1`, userID)
	_, err := r.db.Exec(ctx, `
		INSERT INTO email_verification_tokens (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, userID, expiresAt)
	return err
}

// GetEmailVerificationToken retrieves a valid email verification token.
func (r *TokenRepo) GetEmailVerificationToken(ctx context.Context, token string) (*domain.EmailVerificationToken, error) {
	var t domain.EmailVerificationToken
	err := r.db.QueryRow(ctx, `
		SELECT token, user_id, expires_at
		FROM email_verification_tokens
		WHERE token = $1 AND expires_at > now()
	`, token).Scan(&t.Token, &t.UserID, &t.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeInvalidArgument, "invalid or expired verification token")
		}
		return nil, fmt.Errorf("get verification token: %w", err)
	}
	return &t, nil
}

// DeleteEmailVerificationToken removes a verification token after use.
func (r *TokenRepo) DeleteEmailVerificationToken(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM email_verification_tokens WHERE token = $1`, token)
	return err
}

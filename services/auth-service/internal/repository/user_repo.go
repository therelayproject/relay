// Package repository provides PostgreSQL-backed data access for the auth service.
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

// UserRepo handles user CRUD in relay_auth.
type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// Create inserts a new user and returns it with the generated ID.
func (r *UserRepo) Create(ctx context.Context, email, passwordHash string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, totp_secret, totp_enabled, email_verified, is_active, created_at, updated_at
	`, email, passwordHash).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled,
		&u.EmailVerified, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "email already registered")
		}
		return nil, fmt.Errorf("user create: %w", err)
	}
	return &u, nil
}

// GetByID retrieves a user by primary key.
func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled,
		&u.EmailVerified, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "user not found")
		}
		return nil, fmt.Errorf("user get by id: %w", err)
	}
	return &u, nil
}

// GetByEmail retrieves a user by email address.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, is_active, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled,
		&u.EmailVerified, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "user not found")
		}
		return nil, fmt.Errorf("user get by email: %w", err)
	}
	return &u, nil
}

// MarkEmailVerified sets email_verified = true.
func (r *UserRepo) MarkEmailVerified(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET email_verified = true WHERE id = $1`, id)
	return err
}

// UpdatePassword replaces the password hash.
func (r *UserRepo) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, id)
	return err
}

// UpsertOAuthProvider creates or updates an OAuth provider link.
func (r *UserRepo) UpsertOAuthProvider(ctx context.Context, op *domain.OAuthProvider) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO oauth_providers (user_id, provider, provider_user_id, access_token, refresh_token)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider, provider_user_id)
		DO UPDATE SET access_token = EXCLUDED.access_token, refresh_token = EXCLUDED.refresh_token
	`, op.UserID, op.Provider, op.ProviderUserID, op.AccessToken, op.RefreshToken)
	return err
}

// GetByOAuthProvider finds the user linked to a provider identity.
func (r *UserRepo) GetByOAuthProvider(ctx context.Context, provider, providerUserID string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.password_hash, u.totp_secret, u.totp_enabled,
		       u.email_verified, u.is_active, u.created_at, u.updated_at
		FROM users u
		JOIN oauth_providers op ON op.user_id = u.id
		WHERE op.provider = $1 AND op.provider_user_id = $2
	`, provider, providerUserID).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled,
		&u.EmailVerified, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "oauth user not found")
		}
		return nil, fmt.Errorf("get by oauth provider: %w", err)
	}
	return &u, nil
}

// CreateOAuthUser creates a new user for an OAuth login (no password).
func (r *UserRepo) CreateOAuthUser(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (email, email_verified)
		VALUES ($1, true)
		RETURNING id, email, password_hash, totp_secret, totp_enabled, email_verified, is_active, created_at, updated_at
	`, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled,
		&u.EmailVerified, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "email already registered")
		}
		return nil, fmt.Errorf("create oauth user: %w", err)
	}
	return &u, nil
}

// EnableTOTP stores the encrypted TOTP secret and enables TOTP.
func (r *UserRepo) EnableTOTP(ctx context.Context, userID, encryptedSecret string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET totp_secret = $1, totp_enabled = true WHERE id = $2
	`, encryptedSecret, userID)
	return err
}

// StoreTOTPBackupCodes inserts hashed backup codes for a user.
func (r *UserRepo) StoreTOTPBackupCodes(ctx context.Context, userID string, codeHashes []string) error {
	batch := &pgx.Batch{}
	for _, h := range codeHashes {
		batch.Queue(`INSERT INTO totp_backup_codes (user_id, code_hash) VALUES ($1, $2)`, userID, h)
	}
	br := r.db.SendBatch(ctx, batch)
	defer br.Close()
	for range codeHashes {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("store backup codes: %w", err)
		}
	}
	return nil
}

// GetUnusedTOTPBackupCodes returns unused backup codes for a user.
func (r *UserRepo) GetUnusedTOTPBackupCodes(ctx context.Context, userID string) ([]domain.TOTPBackupCode, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, code_hash, used_at FROM totp_backup_codes
		WHERE user_id = $1 AND used_at IS NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var codes []domain.TOTPBackupCode
	for rows.Next() {
		var c domain.TOTPBackupCode
		if err := rows.Scan(&c.ID, &c.UserID, &c.CodeHash, &c.UsedAt); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}
	return codes, rows.Err()
}

// MarkTOTPBackupCodeUsed marks a backup code as used.
func (r *UserRepo) MarkTOTPBackupCodeUsed(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.Exec(ctx, `UPDATE totp_backup_codes SET used_at = $1 WHERE id = $2`, now, id)
	return err
}

// isUniqueViolation detects PostgreSQL unique constraint errors (code 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "23505") || contains(err.Error(), "unique constraint")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

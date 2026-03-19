package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"github.com/relay-im/relay/services/auth-service/internal/domain"
	"github.com/relay-im/relay/services/auth-service/internal/repository"
	apperrors "github.com/relay-im/relay/shared/errors"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost         = 12
	resetTokenTTL      = time.Hour
	verifyTokenTTL     = 24 * time.Hour
	refreshTokenPrefix = "refresh:"
)

// AuthService orchestrates registration, login, OAuth, and password management.
type AuthService struct {
	users    *repository.UserRepo
	sessions *repository.SessionRepo
	tokens   *repository.TokenRepo
	jwt      *JWTService
	oauth    *OAuthService
	mailer   Mailer
	redis    *redis.Client
}

// NewAuthService constructs an AuthService with all dependencies.
func NewAuthService(
	users *repository.UserRepo,
	sessions *repository.SessionRepo,
	tokens *repository.TokenRepo,
	jwt *JWTService,
	oauth *OAuthService,
	mailer Mailer,
	rdb *redis.Client,
) *AuthService {
	return &AuthService{
		users:    users,
		sessions: sessions,
		tokens:   tokens,
		jwt:      jwt,
		oauth:    oauth,
		mailer:   mailer,
		redis:    rdb,
	}
}

// Register creates a new user account, issues auth tokens, and sends a
// verification email asynchronously. Email verification is not required to use
// the account (dev/beta behaviour), so we auto-login immediately.
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*domain.TokenPair, *domain.User, error) {
	if email == "" || password == "" {
		return nil, nil, apperrors.New(apperrors.CodeInvalidArgument, "email and password are required")
	}
	if len(password) < 8 {
		return nil, nil, apperrors.New(apperrors.CodeInvalidArgument, "password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, nil, fmt.Errorf("bcrypt: %w", err)
	}

	user, err := s.users.Create(ctx, email, string(hash))
	if err != nil {
		return nil, nil, err
	}

	// Issue email verification token and send the email asynchronously.
	token, err := randomHex(32)
	if err == nil {
		_ = s.tokens.CreateEmailVerificationToken(ctx, token, user.ID, time.Now().Add(verifyTokenTTL))
		go s.mailer.SendVerification(email, token) //nolint:errcheck
	}

	// Auto-login: issue tokens so the client can proceed without a separate login step.
	pair, err := s.issueTokens(ctx, user, "", "", "")
	if err != nil {
		return nil, user, err
	}

	return pair, user, nil
}

// Login validates credentials, creates a session, and returns a token pair.
func (s *AuthService) Login(ctx context.Context, email, password, deviceName, userAgent, ip string) (*domain.TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// Return generic error to prevent user enumeration.
		return nil, apperrors.New(apperrors.CodeUnauthenticated, "invalid credentials")
	}

	if !user.IsActive {
		return nil, apperrors.New(apperrors.CodePermissionDenied, "account deactivated")
	}

	if user.PasswordHash == "" {
		return nil, apperrors.New(apperrors.CodeUnauthenticated, "this account uses OAuth login")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, apperrors.New(apperrors.CodeUnauthenticated, "invalid credentials")
	}

	return s.issueTokens(ctx, user, deviceName, userAgent, ip)
}

// OAuthLogin handles the OAuth2 callback flow for a given provider.
func (s *AuthService) OAuthLogin(ctx context.Context, provider, code, deviceName, userAgent, ip string) (*domain.TokenPair, error) {
	info, err := s.oauth.Exchange(ctx, provider, code)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeUnauthenticated, "oauth exchange failed", err)
	}
	if info.Email == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "oauth provider did not return an email")
	}

	// Find existing user linked to this provider identity.
	user, err := s.users.GetByOAuthProvider(ctx, provider, info.ProviderUserID)
	if err != nil {
		// No existing link — find or create the user by email.
		user, err = s.users.GetByEmail(ctx, info.Email)
		if err != nil {
			// New user; create account.
			user, err = s.users.CreateOAuthUser(ctx, info.Email)
			if err != nil {
				return nil, err
			}
		}
		// Link the provider.
		_ = s.users.UpsertOAuthProvider(ctx, &domain.OAuthProvider{
			UserID:         user.ID,
			Provider:       provider,
			ProviderUserID: info.ProviderUserID,
		})
	}

	if !user.IsActive {
		return nil, apperrors.New(apperrors.CodePermissionDenied, "account deactivated")
	}

	return s.issueTokens(ctx, user, deviceName, userAgent, ip)
}

// RefreshTokens exchanges a valid session ID (opaque refresh token) for new access + refresh tokens.
func (s *AuthService) RefreshTokens(ctx context.Context, sessionID string) (*domain.TokenPair, error) {
	// Validate the refresh token in Redis.
	key := refreshTokenPrefix + sessionID
	userID, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, apperrors.New(apperrors.CodeUnauthenticated, "invalid or expired refresh token")
	}
	if err != nil {
		return nil, fmt.Errorf("redis refresh lookup: %w", err)
	}

	// Confirm session still exists in PostgreSQL.
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return nil, apperrors.New(apperrors.CodeUnauthenticated, "session not found")
	}
	_ = userID // already validated via redis

	accessToken, expiresAt, err := s.jwt.IssueAccessToken(session.UserID, session.ID)
	if err != nil {
		return nil, err
	}
	_ = s.sessions.Touch(ctx, sessionID)

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: sessionID,
		ExpiresAt:    expiresAt,
		UserID:       session.UserID,
		SessionID:    sessionID,
	}, nil
}

// Logout revokes a session.
func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	_ = s.redis.Del(ctx, refreshTokenPrefix+sessionID)
	return s.sessions.Delete(ctx, sessionID)
}

// ValidateToken parses an access JWT and returns its claims.
func (s *AuthService) ValidateToken(tokenStr string) (*domain.TokenPair, error) {
	return s.jwt.ValidateAccessToken(tokenStr)
}

// ListSessions returns active sessions for a user.
func (s *AuthService) ListSessions(ctx context.Context, userID string) ([]domain.Session, error) {
	return s.sessions.ListByUser(ctx, userID)
}

// RevokeSession deletes a specific session.
func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID string) error {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID {
		return apperrors.New(apperrors.CodePermissionDenied, "cannot revoke another user's session")
	}
	_ = s.redis.Del(ctx, refreshTokenPrefix+sessionID)
	return s.sessions.Delete(ctx, sessionID)
}

// RequestPasswordReset sends a reset email if the email is registered.
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// Return success even when user not found to prevent enumeration.
		return nil
	}
	token, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}
	if err := s.tokens.CreatePasswordResetToken(ctx, token, user.ID, time.Now().Add(resetTokenTTL)); err != nil {
		return err
	}
	go s.mailer.SendPasswordReset(email, token) //nolint:errcheck
	return nil
}

// ResetPassword applies a new password using a valid reset token.
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return apperrors.New(apperrors.CodeInvalidArgument, "password must be at least 8 characters")
	}
	rt, err := s.tokens.GetPasswordResetToken(ctx, token)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}
	if err := s.users.UpdatePassword(ctx, rt.UserID, string(hash)); err != nil {
		return err
	}
	if err := s.tokens.ConsumePasswordResetToken(ctx, token); err != nil {
		return err
	}
	// Invalidate all existing sessions after password reset.
	go s.sessions.DeleteByUser(context.Background(), rt.UserID) //nolint:errcheck
	return nil
}

// VerifyEmail confirms a user's email address.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	vt, err := s.tokens.GetEmailVerificationToken(ctx, token)
	if err != nil {
		return err
	}
	if err := s.users.MarkEmailVerified(ctx, vt.UserID); err != nil {
		return err
	}
	return s.tokens.DeleteEmailVerificationToken(ctx, token)
}

// SetupTOTP generates a TOTP secret and returns the QR code URL.
func (s *AuthService) SetupTOTP(ctx context.Context, userID, email string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Relay",
		AccountName: email,
	})
	if err != nil {
		return "", "", fmt.Errorf("totp generate: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// VerifyTOTP validates the TOTP code and enables TOTP for the user.
func (s *AuthService) VerifyTOTP(ctx context.Context, userID, secret, code string) ([]string, error) {
	if !totp.Validate(code, secret) {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "invalid TOTP code")
	}
	if err := s.users.EnableTOTP(ctx, userID, secret); err != nil {
		return nil, err
	}
	// Generate 10 backup codes.
	backupCodes := make([]string, 10)
	hashes := make([]string, 10)
	for i := range backupCodes {
		code, err := randomHex(8)
		if err != nil {
			return nil, err
		}
		backupCodes[i] = code
		h, _ := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
		hashes[i] = string(h)
	}
	if err := s.users.StoreTOTPBackupCodes(ctx, userID, hashes); err != nil {
		return nil, err
	}
	return backupCodes, nil
}

// issueTokens creates a session and mints a token pair.
func (s *AuthService) issueTokens(ctx context.Context, user *domain.User, deviceName, userAgent, ip string) (*domain.TokenPair, error) {
	session, err := s.sessions.Create(ctx, user.ID, deviceName, userAgent, ip)
	if err != nil {
		return nil, err
	}

	accessToken, expiresAt, err := s.jwt.IssueAccessToken(user.ID, session.ID)
	if err != nil {
		return nil, err
	}

	// Store refresh token in Redis (session ID is the opaque refresh token).
	refreshTTL := 30 * 24 * time.Hour
	_ = s.redis.Set(ctx, refreshTokenPrefix+session.ID, user.ID, refreshTTL)

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: session.ID,
		ExpiresAt:    expiresAt,
		UserID:       user.ID,
		SessionID:    session.ID,
	}, nil
}

// randomHex generates a cryptographically-random hex string of n bytes.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

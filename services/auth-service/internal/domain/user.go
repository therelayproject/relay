// Package domain defines the core business types for the auth service.
package domain

import (
	"time"
)

// User represents an authenticated identity in the relay_auth database.
type User struct {
	ID            string
	Email         string
	PasswordHash  string // bcrypt hash; empty for OAuth-only accounts
	TOTPSecret    string // AES-256 encrypted; empty until TOTP is set up
	TOTPEnabled   bool
	EmailVerified bool
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// OAuthProvider links an external OAuth identity to a User.
type OAuthProvider struct {
	ID             string
	UserID         string
	Provider       string // "google" | "github"
	ProviderUserID string
	AccessToken    string // encrypted at rest
	RefreshToken   string // encrypted at rest
	CreatedAt      time.Time
}

// Session represents an authenticated device session.
type Session struct {
	ID         string
	UserID     string
	DeviceName string
	UserAgent  string
	IPAddress  string
	LastSeenAt time.Time
	CreatedAt  time.Time
}

// PasswordResetToken is a short-lived single-use token for password reset.
type PasswordResetToken struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// EmailVerificationToken is a short-lived token for verifying an email address.
type EmailVerificationToken struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

// TOTPBackupCode is a hashed single-use backup code for TOTP recovery.
type TOTPBackupCode struct {
	ID       string
	UserID   string
	CodeHash string
	UsedAt   *time.Time
}

// TokenPair holds an access token and a refresh token issued together.
type TokenPair struct {
	AccessToken  string
	RefreshToken string // session ID used as opaque refresh token
	ExpiresAt    time.Time
	UserID       string
	SessionID    string
}

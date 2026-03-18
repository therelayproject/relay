// Package service contains the auth business logic.
package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/relay-im/relay/services/auth-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// JWTConfig holds JWT issuance configuration.
type JWTConfig struct {
	Secret          []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
}

// jwtClaims is the payload embedded in Relay JWTs.
type jwtClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

// JWTService issues and validates JWT access tokens.
type JWTService struct {
	cfg JWTConfig
}

// NewJWTService constructs a JWTService.
func NewJWTService(cfg JWTConfig) *JWTService {
	return &JWTService{cfg: cfg}
}

// IssueAccessToken creates a signed JWT for the given user and session.
func (j *JWTService) IssueAccessToken(userID, sessionID string) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(j.cfg.AccessTokenTTL)
	claims := jwtClaims{
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    j.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(j.cfg.Secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("jwt sign: %w", err)
	}
	return signed, exp, nil
}

// ValidateAccessToken parses and validates a JWT, returning the embedded claims.
func (j *JWTService) ValidateAccessToken(tokenStr string) (*domain.TokenPair, error) {
	claims := &jwtClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.cfg.Secret, nil
	}, jwt.WithIssuer(j.cfg.Issuer))
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeUnauthenticated, "invalid token", err)
	}
	return &domain.TokenPair{
		UserID:    claims.Subject,
		SessionID: claims.SessionID,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

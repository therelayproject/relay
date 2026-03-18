package service

import (
	"testing"
	"time"
)

func newTestJWTService() *JWTService {
	return NewJWTService(JWTConfig{
		Secret:         []byte("super-secret-key-for-tests"),
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "relay-test",
	})
}

func TestJWTService_IssueAndValidate(t *testing.T) {
	svc := newTestJWTService()
	userID := "user-123"
	sessionID := "session-456"

	token, expiresAt, err := svc.IssueAccessToken(userID, sessionID)
	if err != nil {
		t.Fatalf("IssueAccessToken: unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if expiresAt.Before(time.Now()) {
		t.Error("expiresAt should be in the future")
	}

	pair, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("ValidateAccessToken: unexpected error: %v", err)
	}
	if pair.UserID != userID {
		t.Errorf("UserID: got %q, want %q", pair.UserID, userID)
	}
	if pair.SessionID != sessionID {
		t.Errorf("SessionID: got %q, want %q", pair.SessionID, sessionID)
	}
}

func TestJWTService_ValidateRejectsInvalidToken(t *testing.T) {
	svc := newTestJWTService()
	_, err := svc.ValidateAccessToken("not-a-jwt")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}

func TestJWTService_ValidateRejectsWrongSecret(t *testing.T) {
	svc1 := newTestJWTService()
	svc2 := NewJWTService(JWTConfig{
		Secret:         []byte("different-secret"),
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "relay-test",
	})

	token, _, err := svc1.IssueAccessToken("user-1", "sess-1")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	_, err = svc2.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error validating token with wrong secret, got nil")
	}
}

func TestJWTService_ValidateRejectsWrongIssuer(t *testing.T) {
	svc1 := NewJWTService(JWTConfig{
		Secret:         []byte("same-secret"),
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "issuer-a",
	})
	svc2 := NewJWTService(JWTConfig{
		Secret:         []byte("same-secret"),
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "issuer-b",
	})

	token, _, err := svc1.IssueAccessToken("u", "s")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	_, err = svc2.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error validating token with wrong issuer, got nil")
	}
}

func TestJWTService_ValidateRejectsExpiredToken(t *testing.T) {
	svc := NewJWTService(JWTConfig{
		Secret:         []byte("secret"),
		AccessTokenTTL: -time.Second, // already expired
		Issuer:         "relay-test",
	})

	token, _, err := svc.IssueAccessToken("u", "s")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	_, err = svc.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestJWTService_ValidateRejectsEmptyToken(t *testing.T) {
	svc := newTestJWTService()
	_, err := svc.ValidateAccessToken("")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

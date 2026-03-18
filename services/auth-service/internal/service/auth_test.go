package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relay-im/relay/services/auth-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// ── mock mailer ───────────────────────────────────────────────────────────────

type mockMailer struct {
	verificationCalled bool
	resetCalled        bool
}

func (m *mockMailer) SendVerification(to, token string) error {
	m.verificationCalled = true
	return nil
}

func (m *mockMailer) SendPasswordReset(to, token string) error {
	m.resetCalled = true
	return nil
}

// ── Register validation tests (no DB, fails before first repo call) ───────────

func TestRegister_EmptyEmailOrPassword(t *testing.T) {
	svc := &AuthService{mailer: &mockMailer{}, jwt: newTestJWTService()}

	cases := []struct{ email, pass string }{
		{"", "password123"},
		{"user@example.com", ""},
	}
	for _, c := range cases {
		_, err := svc.Register(context.Background(), c.email, c.pass, "Alice")
		if err == nil {
			t.Fatalf("Register(%q, %q): expected error, got nil", c.email, c.pass)
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != apperrors.CodeInvalidArgument {
			t.Errorf("expected CodeInvalidArgument, got %v", err)
		}
	}
}

func TestRegister_PasswordTooShort(t *testing.T) {
	svc := &AuthService{mailer: &mockMailer{}, jwt: newTestJWTService()}

	_, err := svc.Register(context.Background(), "user@example.com", "short", "Alice")
	if err == nil {
		t.Fatal("expected error for short password, got nil")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", err)
	}
}

// ── ResetPassword validation (no DB, fails before first repo call) ────────────

func TestResetPassword_ShortPassword(t *testing.T) {
	svc := &AuthService{}
	err := svc.ResetPassword(context.Background(), "tok", "short")
	if err == nil {
		t.Fatal("expected error for short new password, got nil")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", err)
	}
}

// ── SetupTOTP ─────────────────────────────────────────────────────────────────

func TestSetupTOTP_ReturnsSecretAndURL(t *testing.T) {
	svc := &AuthService{}
	secret, qrURL, err := svc.SetupTOTP(context.Background(), "user-1", "user@example.com")
	if err != nil {
		t.Fatalf("SetupTOTP: unexpected error: %v", err)
	}
	if secret == "" {
		t.Error("expected non-empty TOTP secret")
	}
	if qrURL == "" {
		t.Error("expected non-empty QR URL")
	}
}

// ── VerifyTOTP ────────────────────────────────────────────────────────────────

func TestVerifyTOTP_InvalidCode(t *testing.T) {
	svc := &AuthService{}
	// "000000" is essentially never a valid TOTP code for a real secret.
	_, err := svc.VerifyTOTP(context.Background(), "user-1", "JBSWY3DPEHPK3PXP", "000000")
	if err == nil {
		t.Fatal("expected error for invalid TOTP code, got nil")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", err)
	}
}

// ── randomHex ─────────────────────────────────────────────────────────────────

func TestRandomHex_Length(t *testing.T) {
	for _, n := range []int{8, 16, 32} {
		h, err := randomHex(n)
		if err != nil {
			t.Fatalf("randomHex(%d): unexpected error: %v", n, err)
		}
		if len(h) != n*2 {
			t.Errorf("randomHex(%d): got len %d, want %d", n, len(h), n*2)
		}
	}
}

func TestRandomHex_Uniqueness(t *testing.T) {
	h1, _ := randomHex(16)
	h2, _ := randomHex(16)
	if h1 == h2 {
		t.Error("two randomHex calls should produce different values")
	}
}

// ── domain type sanity ────────────────────────────────────────────────────────

func TestTokenPair_Fields(t *testing.T) {
	exp := time.Now().Add(15 * time.Minute)
	tp := domain.TokenPair{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    exp,
		UserID:       "u",
		SessionID:    "s",
	}
	if tp.UserID != "u" || tp.SessionID != "s" {
		t.Errorf("unexpected field values: %+v", tp)
	}
}

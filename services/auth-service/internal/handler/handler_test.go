package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/relay-im/relay/shared/errors"
)

// ── writeJSON / writeError tests ──────────────────────────────────────────────

func TestWriteJSON_Status(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "yes"})

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["ok"] != "yes" {
		t.Errorf("body: %v", body)
	}
}

func TestWriteError_AppError(t *testing.T) {
	cases := []struct {
		code       apperrors.Code
		wantStatus int
		wantError  string
	}{
		{apperrors.CodeAlreadyExists, http.StatusConflict, "EMAIL_TAKEN"},
		{apperrors.CodeUnauthenticated, http.StatusUnauthorized, "INVALID_CREDENTIALS"},
		{apperrors.CodePermissionDenied, http.StatusForbidden, "FORBIDDEN"},
		{apperrors.CodeNotFound, http.StatusNotFound, "NOT_FOUND"},
		{apperrors.CodeInvalidArgument, http.StatusBadRequest, "VALIDATION_ERROR"},
		{apperrors.CodeResourceExhausted, http.StatusTooManyRequests, "RATE_LIMITED"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		writeError(w, apperrors.New(c.code, "test message"))

		if w.Code != c.wantStatus {
			t.Errorf("code=%s: status got %d, want %d", c.code, w.Code, c.wantStatus)
		}
		var body map[string]string
		_ = json.NewDecoder(w.Body).Decode(&body)
		if body["error"] != c.wantError {
			t.Errorf("code=%s: error field got %q, want %q", c.code, body["error"], c.wantError)
		}
	}
}

func TestWriteError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, apperrors.New(apperrors.CodeInternal, "boom"))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── realIP tests ──────────────────────────────────────────────────────────────

func TestRealIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	ip := realIP(r)
	if ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %q", ip)
	}
}

func TestRealIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.0.100:12345"
	ip := realIP(r)
	if ip != "192.168.0.100" {
		t.Errorf("expected 192.168.0.100, got %q", ip)
	}
}

func TestRealIP_RemoteAddrNoPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.10.10.10"
	ip := realIP(r)
	if ip != "10.10.10.10" {
		t.Errorf("expected 10.10.10.10, got %q", ip)
	}
}

// ── Register handler: invalid JSON body ───────────────────────────────────────

func TestRegisterHandler_InvalidJSON(t *testing.T) {
	h := &AuthHandler{auth: nil, oauth: nil}
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString("NOT JSON"))
	w := httptest.NewRecorder()
	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %q", body["error"])
	}
}

// ── Login handler: invalid JSON body ─────────────────────────────────────────

func TestLoginHandler_InvalidJSON(t *testing.T) {
	h := &AuthHandler{auth: nil, oauth: nil}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── VerifyEmail handler: missing token ───────────────────────────────────────

func TestVerifyEmailHandler_MissingToken(t *testing.T) {
	h := &AuthHandler{auth: nil}
	req := httptest.NewRequest(http.MethodGet, "/auth/verify-email", nil)
	w := httptest.NewRecorder()
	h.VerifyEmail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── OAuthRedirect handler: missing provider ───────────────────────────────────

func TestOAuthRedirectHandler_MissingProvider(t *testing.T) {
	h := &AuthHandler{auth: nil, oauth: nil}
	req := httptest.NewRequest(http.MethodGet, "/auth/oauth/", nil)
	w := httptest.NewRecorder()
	h.OAuthRedirect(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── PasswordResetRequest handler ─────────────────────────────────────────────

func TestPasswordResetRequestHandler_InvalidJSON(t *testing.T) {
	h := &AuthHandler{auth: nil}
	req := httptest.NewRequest(http.MethodPost, "/auth/password/reset-request", bytes.NewBufferString("bad"))
	w := httptest.NewRecorder()
	h.PasswordResetRequest(w, req)

	// Validation error in JSON decode → 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

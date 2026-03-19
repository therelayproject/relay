package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/relay-im/relay/services/auth-service/internal/service"
	"github.com/relay-im/relay/shared/middleware"
)

// AuthHandler exposes the REST API for auth operations.
type AuthHandler struct {
	auth  *service.AuthService
	oauth *service.OAuthService
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(auth *service.AuthService, oauth *service.OAuthService) *AuthHandler {
	return &AuthHandler{auth: auth, oauth: oauth}
}

// ─── Request / Response shapes ───────────────────────────────────────────────

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type passwordResetRequestBody struct {
	Email string `json:"email"`
}

type passwordResetBody struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type totpVerifyRequest struct {
	Secret string `json:"totp_secret"`
	Code   string `json:"totp_code"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// Register handles POST /auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	pair, user, err := h.auth.Register(r.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"user": map[string]any{
			"id":           user.ID,
			"email":        user.Email,
			"display_name": req.DisplayName,
		},
		"accessToken":  pair.AccessToken,
		"refreshToken": pair.RefreshToken,
	})
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	ua := r.Header.Get("User-Agent")
	ip := realIP(r)
	pair, err := h.auth.Login(r.Context(), req.Email, req.Password, "", ua, ip)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    900,
	})
}

// Refresh handles POST /auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	pair, err := h.auth.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    900,
	})
}

// Logout handles POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	if err := h.auth.Logout(r.Context(), req.RefreshToken); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// OAuthRedirect handles GET /auth/oauth/{provider} — redirects the browser to
// the provider's authorization URL.
func (h *AuthHandler) OAuthRedirect(w http.ResponseWriter, r *http.Request) {
	// Accept both the Go 1.22 pattern variable and a plain path suffix so the
	// handler works whether it is registered with or without {provider}.
	provider := r.PathValue("provider")
	if provider == "" {
		// Fall back to stripping the known prefix from the raw path.
		provider = strings.TrimPrefix(r.URL.Path, "/api/v1/auth/oauth/")
		provider = strings.TrimPrefix(provider, "/auth/oauth/")
	}
	if provider == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "provider required"})
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		state = "relay"
	}

	redirectURL, err := h.oauth.AuthURL(provider, state)
	if err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// OAuthCallback handles POST /auth/oauth/{provider}/callback
func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if provider == "" {
		// Legacy path extraction for routes registered without a pattern variable.
		provider = strings.TrimPrefix(r.URL.Path, "/api/v1/auth/oauth/")
		provider = strings.TrimPrefix(provider, "/auth/oauth/")
		provider = strings.TrimSuffix(provider, "/callback")
	}
	var req struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	ua := r.Header.Get("User-Agent")
	ip := realIP(r)
	pair, err := h.auth.OAuthLogin(r.Context(), provider, req.Code, "", ua, ip)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    900,
	})
}

// PasswordResetRequest handles POST /auth/password/reset-request
func (h *AuthHandler) PasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	_ = h.auth.RequestPasswordReset(r.Context(), req.Email)
	// Always return 200 to prevent enumeration.
	writeJSON(w, http.StatusOK, map[string]string{"message": "Reset email sent if account exists"})
}

// PasswordReset handles POST /auth/password/reset
func (h *AuthHandler) PasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	if err := h.auth.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// VerifyEmail handles GET /auth/verify-email?token=...
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "token required"})
		return
	}
	if err := h.auth.VerifyEmail(r.Context(), token); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ListSessions handles GET /auth/sessions
func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}
	sessions, err := h.auth.ListSessions(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	type sessionResp struct {
		ID         string `json:"id"`
		DeviceName string `json:"device_name"`
		IP         string `json:"ip"`
		LastSeenAt string `json:"last_seen_at"`
		CreatedAt  string `json:"created_at"`
	}
	resp := make([]sessionResp, len(sessions))
	for i, s := range sessions {
		resp[i] = sessionResp{
			ID:         s.ID,
			DeviceName: s.DeviceName,
			IP:         s.IPAddress,
			LastSeenAt: s.LastSeenAt.Format("2006-01-02T15:04:05Z"),
			CreatedAt:  s.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": resp})
}

// RevokeSession handles DELETE /auth/sessions/{session_id}
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/auth/sessions/")
	if err := h.auth.RevokeSession(r.Context(), claims.UserID, sessionID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// SetupMFA handles POST /auth/mfa/setup
func (h *AuthHandler) SetupMFA(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}
	// We need the user's email to generate the TOTP URL; fetch from JWT claims (email not in claims here, use sub).
	secret, qrURL, err := h.auth.SetupTOTP(r.Context(), claims.UserID, claims.UserID+"@relay")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"totp_secret": secret,
		"qr_code_url": qrURL,
	})
}

// VerifyMFA handles POST /auth/mfa/verify
func (h *AuthHandler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "UNAUTHORIZED", "message": "missing auth"})
		return
	}
	var req totpVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VALIDATION_ERROR", "message": "invalid JSON"})
		return
	}
	codes, err := h.auth.VerifyTOTP(r.Context(), claims.UserID, req.Secret, req.Code)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"backup_codes": codes,
		"ok":           true,
	})
}

// realIP extracts the client IP from X-Forwarded-For or RemoteAddr.
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// Package middleware provides shared HTTP middleware for Relay services.
// Every service that needs to authenticate requests imports this package
// instead of re-implementing JWT validation.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsKey contextKey = "relay_claims"

// Claims is the standard JWT payload used across all Relay services.
type Claims struct {
	UserID      int64  `json:"user_id"`
	WorkspaceID int64  `json:"workspace_id"`
	Role        string `json:"role"` // owner|admin|member|guest
	jwt.RegisteredClaims
}

// RequireAuth returns an HTTP middleware that validates the Bearer JWT and
// stores the parsed Claims in the request context.
//
// Usage in any service:
//
//	mux.Handle("/messages", middleware.RequireAuth(secret)(handler))
func RequireAuth(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if raw == "" {
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
				return
			}

			claims := &Claims{}
			_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return jwtSecret, nil
			})
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
		})
	}
}

// ClaimsFrom retrieves the validated Claims from the request context.
// Returns nil if RequireAuth middleware was not applied.
func ClaimsFrom(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

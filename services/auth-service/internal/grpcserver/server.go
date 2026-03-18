// Package grpcserver implements the gRPC AuthService for inter-service calls.
package grpcserver

import (
	"context"

	apperrors "github.com/relay-im/relay/shared/errors"
	authv1 "github.com/relay-im/relay/shared/proto/gen/auth/v1"
	"github.com/relay-im/relay/services/auth-service/internal/service"
)

// AuthGRPCServer implements authv1.AuthServiceServer.
type AuthGRPCServer struct {
	authv1.UnimplementedAuthServiceServer
	auth *service.AuthService
}

// New constructs the gRPC server wrapping the provided AuthService.
func New(auth *service.AuthService) *AuthGRPCServer {
	return &AuthGRPCServer{auth: auth}
}

// Login handles gRPC-based credential login (used by internal services).
// The Device field from the request is passed as the deviceName; user-agent
// and IP are empty because they are not available over gRPC.
func (s *AuthGRPCServer) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	pair, err := s.auth.Login(ctx, req.GetEmail(), req.GetPassword(), req.GetDevice(), "", "")
	if err != nil {
		return nil, apperrors.GRPCStatus(err)
	}
	return &authv1.LoginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt.Unix(),
		UserId:       pair.UserID,
	}, nil
}

// Logout revokes the session identified by SessionId.
func (s *AuthGRPCServer) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if err := s.auth.Logout(ctx, req.GetSessionId()); err != nil {
		return nil, apperrors.GRPCStatus(err)
	}
	return &authv1.LogoutResponse{}, nil
}

// RefreshToken issues a new access token from an opaque refresh token
// (the session ID stored in Redis).
func (s *AuthGRPCServer) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	pair, err := s.auth.RefreshTokens(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, apperrors.GRPCStatus(err)
	}
	return &authv1.RefreshTokenResponse{
		AccessToken: pair.AccessToken,
		ExpiresAt:   pair.ExpiresAt.Unix(),
	}, nil
}

// ValidateToken parses a JWT access token and returns its embedded claims.
// An invalid or expired token returns Valid=false with no error so that
// callers can distinguish "definitely invalid" from transient failures.
func (s *AuthGRPCServer) ValidateToken(_ context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	pair, err := s.auth.ValidateToken(req.GetAccessToken())
	if err != nil {
		// Token is invalid; surface as a clean false rather than an error so
		// gateway-level middleware can make a straightforward boolean decision.
		return &authv1.ValidateTokenResponse{Valid: false}, nil
	}
	return &authv1.ValidateTokenResponse{
		Valid:     true,
		UserId:    pair.UserID,
		SessionId: pair.SessionID,
	}, nil
}

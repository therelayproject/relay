// Package grpcserver implements the gRPC UserService for inter-service calls.
package grpcserver

import (
	"context"

	"github.com/relay-im/relay/services/user-service/internal/domain"
	"github.com/relay-im/relay/services/user-service/internal/service"
	apperrors "github.com/relay-im/relay/shared/errors"
	userv1 "github.com/relay-im/relay/shared/proto/gen/user/v1"
)

// UserGRPCServer implements userv1.UserServiceServer.
type UserGRPCServer struct {
	userv1.UnimplementedUserServiceServer
	svc *service.UserService
}

// New constructs the gRPC server.
func New(svc *service.UserService) *UserGRPCServer {
	return &UserGRPCServer{svc: svc}
}

// GetUser fetches a single user profile.
func (s *UserGRPCServer) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	profile, err := s.svc.GetProfile(ctx, req.GetUserId())
	if err != nil {
		return nil, apperrors.GRPCStatus(err)
	}
	return &userv1.GetUserResponse{User: toProto(profile)}, nil
}

// GetUsers fetches multiple user profiles in a single call.
func (s *UserGRPCServer) GetUsers(ctx context.Context, req *userv1.GetUsersRequest) (*userv1.GetUsersResponse, error) {
	profiles, err := s.svc.GetProfiles(ctx, req.GetUserIds())
	if err != nil {
		return nil, apperrors.GRPCStatus(err)
	}
	users := make([]*userv1.User, len(profiles))
	for i := range profiles {
		users[i] = toProto(&profiles[i])
	}
	return &userv1.GetUsersResponse{Users: users}, nil
}

// UpdateProfile applies display_name, avatar_url, and timezone changes.
func (s *UserGRPCServer) UpdateProfile(ctx context.Context, req *userv1.UpdateProfileRequest) (*userv1.UpdateProfileResponse, error) {
	profile, err := s.svc.UpdateProfile(ctx, req.GetUserId(), req.GetDisplayName(), req.GetAvatarUrl(), "UTC")
	if err != nil {
		return nil, apperrors.GRPCStatus(err)
	}
	return &userv1.UpdateProfileResponse{User: toProto(profile)}, nil
}

// SetStatus sets the user's status emoji and text via gRPC.
func (s *UserGRPCServer) SetStatus(ctx context.Context, req *userv1.SetStatusRequest) (*userv1.SetStatusResponse, error) {
	if err := s.svc.SetStatus(ctx, domain.StatusUpdate{
		UserID:      req.GetUserId(),
		StatusText:  req.GetStatusText(),
		StatusEmoji: req.GetStatusEmoji(),
	}); err != nil {
		return nil, apperrors.GRPCStatus(err)
	}
	return &userv1.SetStatusResponse{}, nil
}

// ResolveMentions resolves @mention handles within a workspace.
// @here/@channel/@everyone resolve to mentionsAll=true. Handle-to-user-ID
// resolution requires workspace membership data and is stubbed for now.
func (s *UserGRPCServer) ResolveMentions(_ context.Context, req *userv1.ResolveMentionsRequest) (*userv1.ResolveMentionsResponse, error) {
	mentionsAll := false
	for _, h := range req.GetHandles() {
		switch h {
		case "here", "channel", "everyone":
			mentionsAll = true
		}
	}
	return &userv1.ResolveMentionsResponse{
		UserIds:     nil, // TODO: resolve via workspace membership
		MentionsAll: mentionsAll,
	}, nil
}

// toProto converts a *domain.Profile to a *userv1.User protobuf message.
func toProto(p *domain.Profile) *userv1.User {
	if p == nil {
		return nil
	}
	return &userv1.User{
		Id:          p.UserID,
		DisplayName: p.DisplayName,
		AvatarUrl:   p.AvatarURL,
		StatusText:  p.StatusText,
		StatusEmoji: p.StatusEmoji,
		CreatedAt:   p.CreatedAt.Unix(),
	}
}

// Package service contains the workspace business logic layer.
package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	nats "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	"github.com/relay-im/relay/services/workspace-service/internal/domain"
	"github.com/relay-im/relay/services/workspace-service/internal/repository"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// invitationTTL is the default validity window for a newly created invitation.
const invitationTTL = 7 * 24 * time.Hour

// memberJoinedEvent is the payload published to NATS when a user joins a workspace.
type memberJoinedEvent struct {
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

// WorkspaceRepository describes the persistence operations the workspace service needs.
type WorkspaceRepository interface {
	Create(ctx context.Context, name, slug, description, ownerID string) (*domain.Workspace, error)
	GetByID(ctx context.Context, id string) (*domain.Workspace, error)
	Update(ctx context.Context, id, name, description, iconURL string) (*domain.Workspace, error)
	ListByMember(ctx context.Context, userID string) ([]domain.Workspace, error)
	AddMember(ctx context.Context, workspaceID, userID, role, invitedBy string) (*domain.WorkspaceMember, error)
	GetMember(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceMember, error)
	UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) error
	RemoveMember(ctx context.Context, workspaceID, userID string) error
	ListMembers(ctx context.Context, workspaceID string) ([]domain.WorkspaceMember, error)
	CreateInvitation(ctx context.Context, inv *domain.WorkspaceInvitation) (*domain.WorkspaceInvitation, error)
	GetInvitationByToken(ctx context.Context, token string) (*domain.WorkspaceInvitation, error)
	AcceptInvitation(ctx context.Context, token string) error
}

// WorkspaceService orchestrates workspace operations and NATS event publishing.
type WorkspaceService struct {
	repo WorkspaceRepository
	nc   *nats.Conn // may be nil when NATS is unavailable
	log  zerolog.Logger
}

// New creates a WorkspaceService. nc may be nil; missing NATS connectivity is
// logged as a warning and operations proceed normally.
func New(repo *repository.WorkspaceRepo, nc *nats.Conn, log zerolog.Logger) *WorkspaceService {
	return &WorkspaceService{repo: repo, nc: nc, log: log}
}

// CreateWorkspace creates a new workspace and records the caller as owner.
func (s *WorkspaceService) CreateWorkspace(ctx context.Context, name, slug, description, ownerID string) (*domain.Workspace, error) {
	if name == "" || slug == "" || ownerID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "name, slug, and owner_id are required")
	}
	return s.repo.Create(ctx, name, slug, description, ownerID)
}

// GetWorkspace returns a workspace by ID.
func (s *WorkspaceService) GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error) {
	if id == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "id is required")
	}
	return s.repo.GetByID(ctx, id)
}

// UpdateSettings updates mutable workspace fields. The requester must be an
// admin or owner.
func (s *WorkspaceService) UpdateSettings(ctx context.Context, id, name, description, iconURL, requesterID string) (*domain.Workspace, error) {
	if id == "" || requesterID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "id and requester_id are required")
	}
	if err := s.requireAdminOrOwner(ctx, id, requesterID); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "name must not be empty")
	}
	return s.repo.Update(ctx, id, name, description, iconURL)
}

// InviteByEmail creates an email-targeted invitation and returns the invite token.
func (s *WorkspaceService) InviteByEmail(ctx context.Context, workspaceID, email, role, inviterID string) (string, error) {
	if workspaceID == "" || email == "" || inviterID == "" {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "workspace_id, email, and inviter_id are required")
	}
	if !domain.IsValidRole(role) {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "invalid role")
	}
	if err := s.requireAdminOrOwner(ctx, workspaceID, inviterID); err != nil {
		return "", err
	}
	inv, err := s.repo.CreateInvitation(ctx, &domain.WorkspaceInvitation{
		WorkspaceID: workspaceID,
		Email:       email,
		Token:       uuid.NewString(),
		Role:        role,
		InvitedBy:   inviterID,
		ExpiresAt:   time.Now().Add(invitationTTL),
	})
	if err != nil {
		return "", err
	}
	return inv.Token, nil
}

// InviteByLink creates a link-based invitation (no specific email) and returns
// the invite token.
func (s *WorkspaceService) InviteByLink(ctx context.Context, workspaceID, role, inviterID string) (string, error) {
	if workspaceID == "" || inviterID == "" {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "workspace_id and inviter_id are required")
	}
	if !domain.IsValidRole(role) {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "invalid role")
	}
	if err := s.requireAdminOrOwner(ctx, workspaceID, inviterID); err != nil {
		return "", err
	}
	inv, err := s.repo.CreateInvitation(ctx, &domain.WorkspaceInvitation{
		WorkspaceID: workspaceID,
		Token:       uuid.NewString(),
		Role:        role,
		InvitedBy:   inviterID,
		ExpiresAt:   time.Now().Add(invitationTTL),
	})
	if err != nil {
		return "", err
	}
	return inv.Token, nil
}

// JoinByToken validates an invitation token and adds the user to the workspace.
func (s *WorkspaceService) JoinByToken(ctx context.Context, token, userID string) (*domain.WorkspaceMember, error) {
	if token == "" || userID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "token and user_id are required")
	}

	inv, err := s.repo.GetInvitationByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if inv.AcceptedAt != nil {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "invitation has already been used")
	}
	if time.Now().After(inv.ExpiresAt) {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "invitation has expired")
	}

	member, err := s.repo.AddMember(ctx, inv.WorkspaceID, userID, inv.Role, inv.InvitedBy)
	if err != nil {
		return nil, err
	}

	// Mark the invitation as accepted. For link-based invites this is a
	// best-effort operation; the join already succeeded.
	if err := s.repo.AcceptInvitation(ctx, token); err != nil {
		s.log.Warn().Err(err).Str("token", token).Msg("failed to mark invitation as accepted")
	}

	s.publishMemberJoined(member)
	return member, nil
}

// ListMembers returns all members of a workspace. The requester must be a member.
func (s *WorkspaceService) ListMembers(ctx context.Context, workspaceID, requesterID string) ([]domain.WorkspaceMember, error) {
	if workspaceID == "" || requesterID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "workspace_id and requester_id are required")
	}
	if _, err := s.repo.GetMember(ctx, workspaceID, requesterID); err != nil {
		return nil, apperrors.New(apperrors.CodePermissionDenied, "not a member of this workspace")
	}
	return s.repo.ListMembers(ctx, workspaceID)
}

// UpdateMemberRole changes the role of a workspace member. Requester must be
// admin or owner. The workspace owner's role cannot be changed.
func (s *WorkspaceService) UpdateMemberRole(ctx context.Context, workspaceID, userID, role, requesterID string) error {
	if workspaceID == "" || userID == "" || requesterID == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "workspace_id, user_id, and requester_id are required")
	}
	if !domain.IsValidRole(role) {
		return apperrors.New(apperrors.CodeInvalidArgument, "invalid role")
	}
	if err := s.requireAdminOrOwner(ctx, workspaceID, requesterID); err != nil {
		return err
	}

	// Prevent demoting the workspace owner.
	ws, err := s.repo.GetByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.OwnerID == userID {
		return apperrors.New(apperrors.CodePermissionDenied, "cannot change the role of the workspace owner")
	}

	return s.repo.UpdateMemberRole(ctx, workspaceID, userID, role)
}

// RemoveMember removes a user from a workspace. The requester must be admin/owner,
// or the user may remove themselves (self-leave). The workspace owner cannot be
// removed.
func (s *WorkspaceService) RemoveMember(ctx context.Context, workspaceID, userID, requesterID string) error {
	if workspaceID == "" || userID == "" || requesterID == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "workspace_id, user_id, and requester_id are required")
	}

	ws, err := s.repo.GetByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.OwnerID == userID {
		return apperrors.New(apperrors.CodePermissionDenied, "the workspace owner cannot be removed")
	}

	// Allow self-leave; otherwise require admin/owner.
	if userID != requesterID {
		if err := s.requireAdminOrOwner(ctx, workspaceID, requesterID); err != nil {
			return err
		}
	}

	return s.repo.RemoveMember(ctx, workspaceID, userID)
}

// ListWorkspaces returns all workspaces the user is a member of.
func (s *WorkspaceService) ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error) {
	if userID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "user_id is required")
	}
	return s.repo.ListByMember(ctx, userID)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (s *WorkspaceService) requireAdminOrOwner(ctx context.Context, workspaceID, userID string) error {
	m, err := s.repo.GetMember(ctx, workspaceID, userID)
	if err != nil {
		return apperrors.New(apperrors.CodePermissionDenied, "not a member of this workspace")
	}
	if !domain.CanManageMembers(m.Role) {
		return apperrors.New(apperrors.CodePermissionDenied, "admin or owner role required")
	}
	return nil
}

func (s *WorkspaceService) publishMemberJoined(m *domain.WorkspaceMember) {
	if s.nc == nil {
		s.log.Warn().Msg("NATS unavailable: skipping workspace.member.joined event")
		return
	}
	payload, err := json.Marshal(memberJoinedEvent{
		WorkspaceID: m.WorkspaceID,
		UserID:      m.UserID,
		Role:        m.Role,
		JoinedAt:    m.JoinedAt,
	})
	if err != nil {
		s.log.Error().Err(err).Msg("failed to marshal workspace.member.joined event")
		return
	}
	if err := s.nc.Publish("workspace.member.joined", payload); err != nil {
		s.log.Warn().Err(err).Msg("failed to publish workspace.member.joined event")
	}
}

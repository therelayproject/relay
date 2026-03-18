// Package service implements the business logic for the channel service.
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/relay-im/relay/services/channel-service/internal/domain"
	"github.com/relay-im/relay/services/channel-service/internal/repository"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// ChannelRepository describes the persistence operations the channel service needs.
type ChannelRepository interface {
	Create(ctx context.Context, workspaceID, name, slug, description, channelType, createdBy string) (*domain.Channel, error)
	GetByID(ctx context.Context, id string) (*domain.Channel, error)
	ListByWorkspace(ctx context.Context, workspaceID string, includePublic bool, requesterID string) ([]domain.Channel, error)
	Update(ctx context.Context, id, name, description, topic string) (*domain.Channel, error)
	Archive(ctx context.Context, id string) error
	AddMember(ctx context.Context, channelID, userID, role string) (*domain.ChannelMember, error)
	RemoveMember(ctx context.Context, channelID, userID string) error
	GetMember(ctx context.Context, channelID, userID string) (*domain.ChannelMember, error)
	ListMembers(ctx context.Context, channelID string) ([]domain.ChannelMember, error)
}

// ChannelService implements channel business logic.
type ChannelService struct {
	repo ChannelRepository
	nc   *nats.Conn // may be nil; NATS events are best-effort
	log  zerolog.Logger
}

// NewChannelService constructs a ChannelService.
// nc may be nil; events will be silently skipped.
func NewChannelService(repo *repository.ChannelRepo, nc *nats.Conn, log zerolog.Logger) *ChannelService {
	return &ChannelService{repo: repo, nc: nc, log: log}
}

// ── Channel CRUD ──────────────────────────────────────────────────────────────

// CreateChannel validates input, creates the channel, and adds the creator as owner.
func (s *ChannelService) CreateChannel(
	ctx context.Context,
	workspaceID, name, description, channelType, creatorID string,
) (*domain.Channel, error) {
	if workspaceID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "workspaceId is required")
	}
	if name = strings.TrimSpace(name); name == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "name is required")
	}
	if len(name) > 80 {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "name must be 80 characters or fewer")
	}

	ct := domain.ChannelType(channelType)
	if ct == "" {
		ct = domain.ChannelTypePublic
	}
	if ct != domain.ChannelTypePublic && ct != domain.ChannelTypePrivate && ct != domain.ChannelTypeDM {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "type must be public, private, or dm")
	}

	slug := toSlug(name)
	if slug == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "name produces an invalid slug")
	}

	ch, err := s.repo.Create(ctx, workspaceID, name, slug, description, string(ct), creatorID)
	if err != nil {
		return nil, err
	}

	// Add creator as owner member.
	if _, err := s.repo.AddMember(ctx, ch.ID, creatorID, string(domain.ChannelRoleOwner)); err != nil {
		// Non-fatal: channel was created; log and continue.
		s.log.Warn().Err(err).Str("channel_id", ch.ID).Msg("failed to add creator as member")
	}

	s.publishEvent("channel.created", map[string]any{
		"channel_id":   ch.ID,
		"workspace_id": ch.WorkspaceID,
		"name":         ch.Name,
		"type":         string(ch.Type),
		"created_by":   ch.CreatedBy,
		"created_at":   ch.CreatedAt.Format(time.RFC3339),
	})

	return ch, nil
}

// GetChannel retrieves a channel by ID, enforcing access for private channels.
func (s *ChannelService) GetChannel(ctx context.Context, id, requesterID string) (*domain.Channel, error) {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ch.IsArchived {
		return nil, apperrors.New(apperrors.CodeNotFound, "channel not found")
	}
	if ch.Type == domain.ChannelTypePrivate || ch.Type == domain.ChannelTypeDM {
		if _, err := s.repo.GetMember(ctx, id, requesterID); err != nil {
			return nil, apperrors.New(apperrors.CodePermissionDenied, "you are not a member of this channel")
		}
	}
	return ch, nil
}

// BrowseChannels returns public channels plus any private channels the requester belongs to.
func (s *ChannelService) BrowseChannels(
	ctx context.Context,
	workspaceID, requesterID string,
) ([]domain.Channel, error) {
	if workspaceID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "workspaceId is required")
	}
	return s.repo.ListByWorkspace(ctx, workspaceID, true, requesterID)
}

// UpdateChannel updates channel metadata, enforcing owner/admin role.
func (s *ChannelService) UpdateChannel(
	ctx context.Context,
	id, name, description, topic, requesterID string,
) (*domain.Channel, error) {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ch.IsArchived {
		return nil, apperrors.New(apperrors.CodePermissionDenied, "cannot update an archived channel")
	}
	if err := s.requireAdminOrOwner(ctx, id, requesterID); err != nil {
		return nil, err
	}

	if name = strings.TrimSpace(name); name == "" {
		name = ch.Name // keep existing if not provided
	}
	if len(name) > 80 {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "name must be 80 characters or fewer")
	}

	return s.repo.Update(ctx, id, name, description, topic)
}

// ArchiveChannel archives a channel, enforcing owner/admin role.
func (s *ChannelService) ArchiveChannel(ctx context.Context, id, requesterID string) error {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ch.IsArchived {
		return nil // idempotent
	}
	if err := s.requireAdminOrOwner(ctx, id, requesterID); err != nil {
		return err
	}
	return s.repo.Archive(ctx, id)
}

// ── Membership operations ─────────────────────────────────────────────────────

// JoinChannel adds the user as a member of a public channel.
func (s *ChannelService) JoinChannel(ctx context.Context, channelID, userID string) error {
	ch, err := s.repo.GetByID(ctx, channelID)
	if err != nil {
		return err
	}
	if ch.IsArchived {
		return apperrors.New(apperrors.CodePermissionDenied, "cannot join an archived channel")
	}
	if ch.Type != domain.ChannelTypePublic {
		return apperrors.New(apperrors.CodePermissionDenied, "only public channels can be joined directly")
	}

	// Idempotent: already a member is not an error for a join operation.
	_, err = s.repo.AddMember(ctx, channelID, userID, string(domain.ChannelRoleMember))
	if err != nil {
		var ae *apperrors.AppError
		if isAppErr(err, &ae) && ae.Code == apperrors.CodeAlreadyExists {
			return nil
		}
		return err
	}

	s.publishEvent("channel.member.added", map[string]any{
		"channel_id": channelID,
		"user_id":    userID,
		"role":       string(domain.ChannelRoleMember),
		"joined_at":  time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

// LeaveChannel removes the user from a channel.
func (s *ChannelService) LeaveChannel(ctx context.Context, channelID, userID string) error {
	if _, err := s.repo.GetByID(ctx, channelID); err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, channelID, userID)
}

// AddMember adds a user to a private channel; requires the requester to be admin/owner.
func (s *ChannelService) AddMember(
	ctx context.Context,
	channelID, userID, requesterID string,
) error {
	ch, err := s.repo.GetByID(ctx, channelID)
	if err != nil {
		return err
	}
	if ch.IsArchived {
		return apperrors.New(apperrors.CodePermissionDenied, "cannot add members to an archived channel")
	}
	// For private/dm channels, adding members requires admin.
	if ch.Type == domain.ChannelTypePrivate || ch.Type == domain.ChannelTypeDM {
		if err := s.requireAdminOrOwner(ctx, channelID, requesterID); err != nil {
			return err
		}
	}

	m, err := s.repo.AddMember(ctx, channelID, userID, string(domain.ChannelRoleMember))
	if err != nil {
		return err
	}

	s.publishEvent("channel.member.added", map[string]any{
		"channel_id": channelID,
		"user_id":    userID,
		"role":       string(m.Role),
		"joined_at":  m.JoinedAt.Format(time.RFC3339),
	})
	return nil
}

// RemoveMember removes a user from a channel.
// The requester must be admin/owner, or removing themselves.
func (s *ChannelService) RemoveMember(
	ctx context.Context,
	channelID, userID, requesterID string,
) error {
	if _, err := s.repo.GetByID(ctx, channelID); err != nil {
		return err
	}
	if userID != requesterID {
		if err := s.requireAdminOrOwner(ctx, channelID, requesterID); err != nil {
			return err
		}
	}
	return s.repo.RemoveMember(ctx, channelID, userID)
}

// ListMembers returns all members of a channel, checking access for private channels.
func (s *ChannelService) ListMembers(
	ctx context.Context,
	channelID, requesterID string,
) ([]domain.ChannelMember, error) {
	ch, err := s.repo.GetByID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if ch.Type == domain.ChannelTypePrivate || ch.Type == domain.ChannelTypeDM {
		if _, err := s.repo.GetMember(ctx, channelID, requesterID); err != nil {
			return nil, apperrors.New(apperrors.CodePermissionDenied, "you are not a member of this channel")
		}
	}
	return s.repo.ListMembers(ctx, channelID)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// requireAdminOrOwner returns PermissionDenied if requesterID is not admin/owner.
func (s *ChannelService) requireAdminOrOwner(ctx context.Context, channelID, requesterID string) error {
	m, err := s.repo.GetMember(ctx, channelID, requesterID)
	if err != nil {
		return apperrors.New(apperrors.CodePermissionDenied, "you are not a member of this channel")
	}
	if m.Role != domain.ChannelRoleOwner && m.Role != domain.ChannelRoleAdmin {
		return apperrors.New(apperrors.CodePermissionDenied, "insufficient permissions")
	}
	return nil
}

// publishEvent publishes a NATS event best-effort (errors are logged and ignored).
func (s *ChannelService) publishEvent(subject string, payload map[string]any) {
	if s.nc == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		s.log.Warn().Err(err).Str("subject", subject).Msg("failed to marshal event payload")
		return
	}
	if err := s.nc.Publish(subject, data); err != nil {
		s.log.Warn().Err(err).Str("subject", subject).Msg("failed to publish NATS event")
	}
}

// toSlug converts a channel name to a URL-safe lowercase slug.
func toSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	// Replace non-alphanumeric characters with hyphens.
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 80 {
		slug = slug[:80]
		slug = strings.TrimRight(slug, "-")
	}
	return slug
}

// isAppErr unwraps err into *apperrors.AppError if possible.
func isAppErr(err error, ae **apperrors.AppError) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*apperrors.AppError); ok {
		*ae = e
		return true
	}
	return false
}

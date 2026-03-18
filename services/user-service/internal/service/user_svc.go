// Package service implements the user-service business logic.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/relay-im/relay/services/user-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// UserRepository describes the persistence operations the service needs.
type UserRepository interface {
	Create(ctx context.Context, userID, displayName, email string) (*domain.Profile, error)
	GetByID(ctx context.Context, userID string) (*domain.Profile, error)
	GetByIDs(ctx context.Context, userIDs []string) ([]domain.Profile, error)
	UpdateProfile(ctx context.Context, userID, displayName, avatarURL, timezone string) (*domain.Profile, error)
	SetStatus(ctx context.Context, userID, statusText, statusEmoji string, expiresAt *time.Time) error
	SetAvatarURL(ctx context.Context, userID, avatarURL string) error
}

// UserService orchestrates user profile operations.
type UserService struct {
	repo    UserRepository
	storage StorageService
}

// New constructs a UserService.
func New(repo UserRepository, storage StorageService) *UserService {
	return &UserService{repo: repo, storage: storage}
}

// CreateProfile creates a new profile for a freshly registered user.
func (s *UserService) CreateProfile(ctx context.Context, userID, displayName, email string) (*domain.Profile, error) {
	if userID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "user_id is required")
	}
	if displayName == "" {
		displayName = nameFromEmail(email)
	}
	return s.repo.Create(ctx, userID, displayName, email)
}

// GetProfile returns the profile for the given user ID.
func (s *UserService) GetProfile(ctx context.Context, userID string) (*domain.Profile, error) {
	if userID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "user_id is required")
	}
	return s.repo.GetByID(ctx, userID)
}

// GetProfiles returns profiles for a batch of user IDs (max 100).
func (s *UserService) GetProfiles(ctx context.Context, userIDs []string) ([]domain.Profile, error) {
	if len(userIDs) == 0 {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "at least one user_id is required")
	}
	if len(userIDs) > 100 {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "batch size must not exceed 100")
	}
	return s.repo.GetByIDs(ctx, userIDs)
}

// UpdateProfile applies display_name, avatar_url, and timezone changes.
func (s *UserService) UpdateProfile(ctx context.Context, userID, displayName, avatarURL, timezone string) (*domain.Profile, error) {
	if userID == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "user_id is required")
	}
	if displayName == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "display_name must not be empty")
	}
	if timezone == "" {
		timezone = "UTC"
	}
	return s.repo.UpdateProfile(ctx, userID, displayName, avatarURL, timezone)
}

// SetStatus sets the user's status text, emoji, and optional expiry.
func (s *UserService) SetStatus(ctx context.Context, update domain.StatusUpdate) error {
	if update.UserID == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "user_id is required")
	}
	return s.repo.SetStatus(ctx, update.UserID, update.StatusText, update.StatusEmoji, update.StatusExpiresAt)
}

// UploadAvatar stores the avatar image in MinIO and updates the profile.
func (s *UserService) UploadAvatar(ctx context.Context, userID string, data []byte, contentType string) (string, error) {
	if userID == "" {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "user_id is required")
	}
	if len(data) == 0 {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "avatar data is empty")
	}
	if len(data) > 5*1024*1024 {
		return "", apperrors.New(apperrors.CodeInvalidArgument, "avatar must be smaller than 5 MB")
	}

	url, err := s.storage.UploadAvatar(ctx, userID, data, contentType)
	if err != nil {
		return "", err
	}
	if err := s.repo.SetAvatarURL(ctx, userID, url); err != nil {
		return "", err
	}
	return url, nil
}

// nameFromEmail derives a display name from an email local part.
func nameFromEmail(email string) string {
	local := email
	if at := strings.IndexByte(email, '@'); at > 0 {
		local = email[:at]
	}
	replacer := strings.NewReplacer(".", " ", "_", " ", "-", " ")
	return strings.Title(strings.ToLower(replacer.Replace(local))) //nolint:staticcheck
}

// StorageService is the interface for avatar object storage.
type StorageService interface {
	UploadAvatar(ctx context.Context, userID string, data []byte, contentType string) (publicURL string, err error)
}

// NoopStorageService is an alias for NoopStorage kept for backward compatibility.
// Prefer NoopStorage directly for new code.
type NoopStorageService = NoopStorage

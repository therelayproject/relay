// Package service contains the user-service business logic.
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relay-im/relay/services/user-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// ── mock repository ───────────────────────────────────────────────────────────

type mockUserRepo struct {
	createFn        func(ctx context.Context, userID, displayName, email string) (*domain.Profile, error)
	getByIDFn       func(ctx context.Context, userID string) (*domain.Profile, error)
	getByIDsFn      func(ctx context.Context, userIDs []string) ([]domain.Profile, error)
	updateProfileFn func(ctx context.Context, userID, displayName, avatarURL, timezone string) (*domain.Profile, error)
	setStatusFn     func(ctx context.Context, userID, statusText, statusEmoji string, expiresAt *time.Time) error
	setAvatarURLFn  func(ctx context.Context, userID, avatarURL string) error
}

func (m *mockUserRepo) Create(ctx context.Context, userID, displayName, email string) (*domain.Profile, error) {
	if m.createFn != nil {
		return m.createFn(ctx, userID, displayName, email)
	}
	return &domain.Profile{UserID: userID, DisplayName: displayName}, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, userID string) (*domain.Profile, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, userID)
	}
	return &domain.Profile{UserID: userID}, nil
}

func (m *mockUserRepo) GetByIDs(ctx context.Context, userIDs []string) ([]domain.Profile, error) {
	if m.getByIDsFn != nil {
		return m.getByIDsFn(ctx, userIDs)
	}
	profiles := make([]domain.Profile, len(userIDs))
	for i, id := range userIDs {
		profiles[i] = domain.Profile{UserID: id}
	}
	return profiles, nil
}

func (m *mockUserRepo) UpdateProfile(ctx context.Context, userID, displayName, avatarURL, timezone string) (*domain.Profile, error) {
	if m.updateProfileFn != nil {
		return m.updateProfileFn(ctx, userID, displayName, avatarURL, timezone)
	}
	return &domain.Profile{UserID: userID, DisplayName: displayName, Timezone: timezone}, nil
}

func (m *mockUserRepo) SetStatus(ctx context.Context, userID, statusText, statusEmoji string, expiresAt *time.Time) error {
	if m.setStatusFn != nil {
		return m.setStatusFn(ctx, userID, statusText, statusEmoji, expiresAt)
	}
	return nil
}

func (m *mockUserRepo) SetAvatarURL(ctx context.Context, userID, avatarURL string) error {
	if m.setAvatarURLFn != nil {
		return m.setAvatarURLFn(ctx, userID, avatarURL)
	}
	return nil
}

// ── mock storage ──────────────────────────────────────────────────────────────

type mockStorage struct {
	uploadFn func(ctx context.Context, userID string, data []byte, contentType string) (string, error)
}

func (s *mockStorage) UploadAvatar(ctx context.Context, userID string, data []byte, contentType string) (string, error) {
	if s.uploadFn != nil {
		return s.uploadFn(ctx, userID, data, contentType)
	}
	return "https://cdn.example.com/avatars/" + userID, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newSvc(repo UserRepository) *UserService {
	return New(repo, &NoopStorage{})
}

func assertUserCode(t *testing.T, err error, want apperrors.Code) {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Errorf("expected *apperrors.AppError, got %T: %v", err, err)
		return
	}
	if ae.Code != want {
		t.Errorf("error code: got %q, want %q", ae.Code, want)
	}
}

// ── CreateProfile ─────────────────────────────────────────────────────────────

func TestCreateProfile_EmptyUserID(t *testing.T) {
	svc := newSvc(&mockUserRepo{})
	_, err := svc.CreateProfile(context.Background(), "", "Alice", "alice@example.com")
	if err == nil {
		t.Fatal("expected error for empty user ID, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestCreateProfile_DeriveDisplayNameFromEmail(t *testing.T) {
	var capturedDisplayName string
	repo := &mockUserRepo{
		createFn: func(_ context.Context, userID, displayName, email string) (*domain.Profile, error) {
			capturedDisplayName = displayName
			return &domain.Profile{UserID: userID, DisplayName: displayName}, nil
		},
	}
	svc := newSvc(repo)
	_, err := svc.CreateProfile(context.Background(), "u1", "", "john.doe@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedDisplayName == "" {
		t.Error("expected display name derived from email, got empty string")
	}
}

func TestCreateProfile_ExplicitDisplayName(t *testing.T) {
	var capturedDisplayName string
	repo := &mockUserRepo{
		createFn: func(_ context.Context, userID, displayName, email string) (*domain.Profile, error) {
			capturedDisplayName = displayName
			return &domain.Profile{UserID: userID, DisplayName: displayName}, nil
		},
	}
	svc := newSvc(repo)
	_, err := svc.CreateProfile(context.Background(), "u1", "Alice", "alice@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedDisplayName != "Alice" {
		t.Errorf("expected displayName=Alice, got %q", capturedDisplayName)
	}
}

// ── GetProfile ────────────────────────────────────────────────────────────────

func TestGetProfile_EmptyUserID(t *testing.T) {
	svc := newSvc(&mockUserRepo{})
	_, err := svc.GetProfile(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty user ID, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestGetProfile_PropagatesRepoError(t *testing.T) {
	notFound := apperrors.New(apperrors.CodeNotFound, "user not found")
	repo := &mockUserRepo{
		getByIDFn: func(_ context.Context, _ string) (*domain.Profile, error) { return nil, notFound },
	}
	svc := newSvc(repo)
	_, err := svc.GetProfile(context.Background(), "u-missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUserCode(t, err, apperrors.CodeNotFound)
}

func TestGetProfile_Success(t *testing.T) {
	repo := &mockUserRepo{
		getByIDFn: func(_ context.Context, userID string) (*domain.Profile, error) {
			return &domain.Profile{UserID: userID, DisplayName: "Alice"}, nil
		},
	}
	svc := newSvc(repo)
	p, err := svc.GetProfile(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.UserID != "u1" || p.DisplayName != "Alice" {
		t.Errorf("unexpected profile: %+v", p)
	}
}

// ── GetProfiles ───────────────────────────────────────────────────────────────

func TestGetProfiles_EmptyList(t *testing.T) {
	svc := newSvc(&mockUserRepo{})
	_, err := svc.GetProfiles(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty list, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestGetProfiles_TooManyIDs(t *testing.T) {
	svc := newSvc(&mockUserRepo{})
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = "u"
	}
	_, err := svc.GetProfiles(context.Background(), ids)
	if err == nil {
		t.Fatal("expected error for batch > 100, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestGetProfiles_ExactLimit(t *testing.T) {
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = "u"
	}
	svc := newSvc(&mockUserRepo{})
	_, err := svc.GetProfiles(context.Background(), ids)
	// Should reach repo (no validation error). If the mock returns fine, no error.
	if err != nil {
		var ae *apperrors.AppError
		if errors.As(err, &ae) && ae.Code == apperrors.CodeInvalidArgument {
			t.Fatal("100 IDs should NOT be rejected by validation")
		}
	}
}

// ── UpdateProfile ─────────────────────────────────────────────────────────────

func TestUpdateProfile_EmptyUserID(t *testing.T) {
	svc := newSvc(&mockUserRepo{})
	_, err := svc.UpdateProfile(context.Background(), "", "Alice", "", "UTC")
	if err == nil {
		t.Fatal("expected error for empty user ID, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUpdateProfile_EmptyDisplayName(t *testing.T) {
	svc := newSvc(&mockUserRepo{})
	_, err := svc.UpdateProfile(context.Background(), "u1", "", "", "UTC")
	if err == nil {
		t.Fatal("expected error for empty display name, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUpdateProfile_DefaultsTimezoneToUTC(t *testing.T) {
	var capturedTZ string
	repo := &mockUserRepo{
		updateProfileFn: func(_ context.Context, _, _, _, tz string) (*domain.Profile, error) {
			capturedTZ = tz
			return &domain.Profile{}, nil
		},
	}
	svc := newSvc(repo)
	_, err := svc.UpdateProfile(context.Background(), "u1", "Alice", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedTZ != "UTC" {
		t.Errorf("expected timezone=UTC, got %q", capturedTZ)
	}
}

// ── SetStatus ─────────────────────────────────────────────────────────────────

func TestSetStatus_EmptyUserID(t *testing.T) {
	svc := newSvc(&mockUserRepo{})
	err := svc.SetStatus(context.Background(), domain.StatusUpdate{UserID: ""})
	if err == nil {
		t.Fatal("expected error for empty user ID, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestSetStatus_Success(t *testing.T) {
	called := false
	repo := &mockUserRepo{
		setStatusFn: func(_ context.Context, userID, _, _ string, _ *time.Time) error {
			called = true
			return nil
		},
	}
	svc := newSvc(repo)
	err := svc.SetStatus(context.Background(), domain.StatusUpdate{UserID: "u1", StatusText: "coding"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected SetStatus to call repo")
	}
}

// ── UploadAvatar ──────────────────────────────────────────────────────────────

func TestUploadAvatar_EmptyUserID(t *testing.T) {
	svc := New(&mockUserRepo{}, &NoopStorage{})
	_, err := svc.UploadAvatar(context.Background(), "", []byte("data"), "image/jpeg")
	if err == nil {
		t.Fatal("expected error for empty user ID, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUploadAvatar_EmptyData(t *testing.T) {
	svc := New(&mockUserRepo{}, &NoopStorage{})
	_, err := svc.UploadAvatar(context.Background(), "u1", nil, "image/jpeg")
	if err == nil {
		t.Fatal("expected error for empty data, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUploadAvatar_DataTooLarge(t *testing.T) {
	svc := New(&mockUserRepo{}, &NoopStorage{})
	big := make([]byte, 5*1024*1024+1)
	_, err := svc.UploadAvatar(context.Background(), "u1", big, "image/jpeg")
	if err == nil {
		t.Fatal("expected error for data > 5MB, got nil")
	}
	assertUserCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUploadAvatar_Success(t *testing.T) {
	avatarURLSet := false
	repo := &mockUserRepo{
		setAvatarURLFn: func(_ context.Context, _, _ string) error {
			avatarURLSet = true
			return nil
		},
	}
	storage := &mockStorage{
		uploadFn: func(_ context.Context, userID string, _ []byte, _ string) (string, error) {
			return "https://cdn.example.com/avatars/" + userID, nil
		},
	}
	svc := New(repo, storage)
	url, err := svc.UploadAvatar(context.Background(), "u1", []byte("fake-image"), "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty URL")
	}
	if !avatarURLSet {
		t.Error("expected repo.SetAvatarURL to be called")
	}
}

func TestUploadAvatar_StorageError(t *testing.T) {
	storage := &mockStorage{
		uploadFn: func(_ context.Context, _ string, _ []byte, _ string) (string, error) {
			return "", errors.New("storage failure")
		},
	}
	svc := New(&mockUserRepo{}, storage)
	_, err := svc.UploadAvatar(context.Background(), "u1", []byte("data"), "image/jpeg")
	if err == nil {
		t.Fatal("expected error when storage fails, got nil")
	}
}

func TestUploadAvatar_RepoError(t *testing.T) {
	// Storage succeeds but repo.SetAvatarURL fails.
	storage := &mockStorage{
		uploadFn: func(_ context.Context, userID string, _ []byte, _ string) (string, error) {
			return "https://cdn.example.com/avatars/" + userID, nil
		},
	}
	repo := &mockUserRepo{
		setAvatarURLFn: func(_ context.Context, _, _ string) error {
			return errors.New("db error")
		},
	}
	svc := New(repo, storage)
	_, err := svc.UploadAvatar(context.Background(), "u1", []byte("img"), "image/png")
	if err == nil {
		t.Fatal("expected error when repo.SetAvatarURL fails, got nil")
	}
}

// ── NoopStorage ───────────────────────────────────────────────────────────────

func TestNoopStorage_UploadAvatar(t *testing.T) {
	s := &NoopStorage{}
	url, err := s.UploadAvatar(context.Background(), "user-abc", []byte("data"), "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty URL from NoopStorage")
	}
}

// ── nameFromEmail ─────────────────────────────────────────────────────────────

func TestNameFromEmail(t *testing.T) {
	cases := []struct {
		email string
		want  string
	}{
		{"alice@example.com", "Alice"},
		{"john.doe@example.com", "John Doe"},
		{"first_last@example.com", "First Last"},
		{"user-name@example.com", "User Name"},
		{"noatsign", "Noatsign"},
	}
	for _, c := range cases {
		got := nameFromEmail(c.email)
		if got != c.want {
			t.Errorf("nameFromEmail(%q) = %q, want %q", c.email, got, c.want)
		}
	}
}

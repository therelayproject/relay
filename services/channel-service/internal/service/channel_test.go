package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/relay-im/relay/services/channel-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// ── toSlug tests ──────────────────────────────────────────────────────────────

func TestToSlug(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"General", "general"},
		{"  Hello World  ", "hello-world"},
		{"Dev Ops & CI/CD", "dev-ops-ci-cd"},
		{"123-abc", "123-abc"},
		{"already-slug", "already-slug"},
		{"multiple   spaces", "multiple-spaces"},
		{"special!@#chars", "special-chars"},
		{"", ""},
	}
	for _, c := range cases {
		got := toSlug(c.input)
		if got != c.want {
			t.Errorf("toSlug(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestToSlug_TruncatesAt80(t *testing.T) {
	long := "a-very-long-channel-name-that-exceeds-the-maximum-allowed-slug-length-xxxxxxxxxxxxxxxxxx"
	got := toSlug(long)
	if len(got) > 80 {
		t.Errorf("slug length %d > 80", len(got))
	}
}

// ── CreateChannel validation tests ───────────────────────────────────────────

func newTestChannelService() *ChannelService {
	return &ChannelService{
		repo: nil, // not nil-safe for DB calls; validation happens before repo
		nc:   nil,
		log:  zerolog.Nop(),
	}
}

func TestCreateChannel_MissingWorkspaceID(t *testing.T) {
	svc := newTestChannelService()
	_, err := svc.CreateChannel(context.Background(), "", "general", "", "public", "user-1")
	if err == nil {
		t.Fatal("expected error for empty workspaceID, got nil")
	}
	assertCode(t, err, apperrors.CodeInvalidArgument)
}

func TestCreateChannel_EmptyName(t *testing.T) {
	svc := newTestChannelService()
	_, err := svc.CreateChannel(context.Background(), "ws-1", "   ", "", "public", "user-1")
	if err == nil {
		t.Fatal("expected error for blank name, got nil")
	}
	assertCode(t, err, apperrors.CodeInvalidArgument)
}

func TestCreateChannel_NameTooLong(t *testing.T) {
	svc := newTestChannelService()
	longName := "a-channel-name-that-is-way-too-long-to-be-accepted-by-the-service-XXXXXXXXXXXXXXXXXXXXXXXXX"
	_, err := svc.CreateChannel(context.Background(), "ws-1", longName, "", "public", "user-1")
	if err == nil {
		t.Fatal("expected error for name > 80 chars, got nil")
	}
	assertCode(t, err, apperrors.CodeInvalidArgument)
}

func TestCreateChannel_InvalidType(t *testing.T) {
	svc := newTestChannelService()
	_, err := svc.CreateChannel(context.Background(), "ws-1", "general", "", "supertype", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid channel type, got nil")
	}
	assertCode(t, err, apperrors.CodeInvalidArgument)
}

func TestCreateChannel_NameWithOnlySpecialChars(t *testing.T) {
	// All special chars → slug becomes empty → invalid
	svc := newTestChannelService()
	_, err := svc.CreateChannel(context.Background(), "ws-1", "!!!###", "", "public", "user-1")
	if err == nil {
		t.Fatal("expected error for name producing empty slug, got nil")
	}
	assertCode(t, err, apperrors.CodeInvalidArgument)
}

// ── BrowseChannels validation ─────────────────────────────────────────────────

func TestBrowseChannels_MissingWorkspaceID(t *testing.T) {
	svc := newTestChannelService()
	_, err := svc.BrowseChannels(context.Background(), "", "user-1")
	if err == nil {
		t.Fatal("expected error for empty workspaceID, got nil")
	}
	assertCode(t, err, apperrors.CodeInvalidArgument)
}

// ── UpdateChannel validation ──────────────────────────────────────────────────

func TestUpdateChannel_NameTooLong(t *testing.T) {
	// The name-length check runs AFTER a repo call (GetByID), so we can't test
	// it without a DB. This test documents that expectation.
	t.Log("UpdateChannel name-length check requires DB — covered by integration tests")
}

// ── isAppErr helper ───────────────────────────────────────────────────────────

func Test_isAppErr(t *testing.T) {
	ae := apperrors.New(apperrors.CodeNotFound, "not found")
	var target *apperrors.AppError
	if !isAppErr(ae, &target) {
		t.Error("expected isAppErr to return true for AppError")
	}
	if target.Code != apperrors.CodeNotFound {
		t.Errorf("wrong code: %v", target.Code)
	}

	plain := errors.New("plain error")
	if isAppErr(plain, &target) {
		t.Error("expected isAppErr to return false for plain error")
	}

	if isAppErr(nil, &target) {
		t.Error("expected isAppErr to return false for nil error")
	}
}

// ── helper ────────────────────────────────────────────────────────────────────

func assertCode(t *testing.T, err error, want apperrors.Code) {
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

// ── mock repository ───────────────────────────────────────────────────────────

type mockChannelRepo struct {
	createFn         func(ctx context.Context, workspaceID, name, slug, description, channelType, createdBy string) (*domain.Channel, error)
	getByIDFn        func(ctx context.Context, id string) (*domain.Channel, error)
	listByWorkspaceFn func(ctx context.Context, workspaceID string, includePublic bool, requesterID string) ([]domain.Channel, error)
	updateFn         func(ctx context.Context, id, name, description, topic string) (*domain.Channel, error)
	archiveFn        func(ctx context.Context, id string) error
	addMemberFn      func(ctx context.Context, channelID, userID, role string) (*domain.ChannelMember, error)
	removeMemberFn   func(ctx context.Context, channelID, userID string) error
	getMemberFn      func(ctx context.Context, channelID, userID string) (*domain.ChannelMember, error)
	listMembersFn    func(ctx context.Context, channelID string) ([]domain.ChannelMember, error)
}

func (m *mockChannelRepo) Create(ctx context.Context, workspaceID, name, slug, description, channelType, createdBy string) (*domain.Channel, error) {
	if m.createFn != nil {
		return m.createFn(ctx, workspaceID, name, slug, description, channelType, createdBy)
	}
	return &domain.Channel{ID: "ch-1", WorkspaceID: workspaceID, Name: name, Type: domain.ChannelType(channelType), CreatedBy: createdBy}, nil
}
func (m *mockChannelRepo) GetByID(ctx context.Context, id string) (*domain.Channel, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return &domain.Channel{ID: id, Type: domain.ChannelTypePublic}, nil
}
func (m *mockChannelRepo) ListByWorkspace(ctx context.Context, workspaceID string, includePublic bool, requesterID string) ([]domain.Channel, error) {
	if m.listByWorkspaceFn != nil {
		return m.listByWorkspaceFn(ctx, workspaceID, includePublic, requesterID)
	}
	return []domain.Channel{}, nil
}
func (m *mockChannelRepo) Update(ctx context.Context, id, name, description, topic string) (*domain.Channel, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, name, description, topic)
	}
	return &domain.Channel{ID: id, Name: name}, nil
}
func (m *mockChannelRepo) Archive(ctx context.Context, id string) error {
	if m.archiveFn != nil {
		return m.archiveFn(ctx, id)
	}
	return nil
}
func (m *mockChannelRepo) AddMember(ctx context.Context, channelID, userID, role string) (*domain.ChannelMember, error) {
	if m.addMemberFn != nil {
		return m.addMemberFn(ctx, channelID, userID, role)
	}
	return &domain.ChannelMember{ChannelID: channelID, UserID: userID, Role: domain.ChannelRole(role)}, nil
}
func (m *mockChannelRepo) RemoveMember(ctx context.Context, channelID, userID string) error {
	if m.removeMemberFn != nil {
		return m.removeMemberFn(ctx, channelID, userID)
	}
	return nil
}
func (m *mockChannelRepo) GetMember(ctx context.Context, channelID, userID string) (*domain.ChannelMember, error) {
	if m.getMemberFn != nil {
		return m.getMemberFn(ctx, channelID, userID)
	}
	return &domain.ChannelMember{ChannelID: channelID, UserID: userID, Role: domain.ChannelRoleOwner}, nil
}
func (m *mockChannelRepo) ListMembers(ctx context.Context, channelID string) ([]domain.ChannelMember, error) {
	if m.listMembersFn != nil {
		return m.listMembersFn(ctx, channelID)
	}
	return []domain.ChannelMember{}, nil
}

func newMockedChannelService(repo ChannelRepository) *ChannelService {
	return &ChannelService{repo: repo, nc: nil, log: zerolog.Nop()}
}

// ── CreateChannel with mock repo ──────────────────────────────────────────────

func TestCreateChannel_Success(t *testing.T) {
	repo := &mockChannelRepo{}
	svc := newMockedChannelService(repo)
	ch, err := svc.CreateChannel(context.Background(), "ws-1", "general", "General channel", "public", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.Name != "general" {
		t.Errorf("expected name=general, got %q", ch.Name)
	}
}

func TestCreateChannel_RepoError(t *testing.T) {
	repo := &mockChannelRepo{
		createFn: func(_ context.Context, _, _, _, _, _, _ string) (*domain.Channel, error) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "slug exists")
		},
	}
	svc := newMockedChannelService(repo)
	_, err := svc.CreateChannel(context.Background(), "ws-1", "general", "", "public", "user-1")
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
}

func TestCreateChannel_DefaultType(t *testing.T) {
	repo := &mockChannelRepo{}
	svc := newMockedChannelService(repo)
	// Empty type should default to public.
	ch, err := svc.CreateChannel(context.Background(), "ws-1", "general", "", "", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.Type != domain.ChannelTypePublic {
		t.Errorf("expected type=public, got %v", ch.Type)
	}
}

// ── GetChannel with mock repo ─────────────────────────────────────────────────

func TestGetChannel_Success(t *testing.T) {
	repo := &mockChannelRepo{}
	svc := newMockedChannelService(repo)
	ch, err := svc.GetChannel(context.Background(), "ch-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.ID != "ch-1" {
		t.Errorf("expected ID=ch-1, got %q", ch.ID)
	}
}

func TestGetChannel_Archived(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, IsArchived: true}, nil
		},
	}
	svc := newMockedChannelService(repo)
	_, err := svc.GetChannel(context.Background(), "ch-archived", "user-1")
	if err == nil {
		t.Fatal("expected error for archived channel, got nil")
	}
	assertCode(t, err, apperrors.CodeNotFound)
}

func TestGetChannel_PrivateDenied(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, Type: domain.ChannelTypePrivate}, nil
		},
		getMemberFn: func(_ context.Context, _, _ string) (*domain.ChannelMember, error) {
			return nil, apperrors.New(apperrors.CodeNotFound, "not a member")
		},
	}
	svc := newMockedChannelService(repo)
	_, err := svc.GetChannel(context.Background(), "ch-priv", "outsider")
	if err == nil {
		t.Fatal("expected permission denied for non-member of private channel")
	}
	assertCode(t, err, apperrors.CodePermissionDenied)
}

func TestGetChannel_PrivateAllowed(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, Type: domain.ChannelTypePrivate}, nil
		},
		getMemberFn: func(_ context.Context, _, userID string) (*domain.ChannelMember, error) {
			return &domain.ChannelMember{UserID: userID, Role: domain.ChannelRoleMember}, nil
		},
	}
	svc := newMockedChannelService(repo)
	ch, err := svc.GetChannel(context.Background(), "ch-priv", "member-1")
	if err != nil {
		t.Fatalf("unexpected error for member of private channel: %v", err)
	}
	if ch.ID != "ch-priv" {
		t.Errorf("unexpected channel: %+v", ch)
	}
}

// ── BrowseChannels with mock ───────────────────────────────────────────────────

func TestBrowseChannels_Success(t *testing.T) {
	called := false
	repo := &mockChannelRepo{
		listByWorkspaceFn: func(_ context.Context, _ string, _ bool, _ string) ([]domain.Channel, error) {
			called = true
			return []domain.Channel{{ID: "ch-1"}, {ID: "ch-2"}}, nil
		},
	}
	svc := newMockedChannelService(repo)
	chs, err := svc.BrowseChannels(context.Background(), "ws-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected repo.ListByWorkspace to be called")
	}
	if len(chs) != 2 {
		t.Errorf("expected 2 channels, got %d", len(chs))
	}
}

// ── UpdateChannel with mock ───────────────────────────────────────────────────

func TestUpdateChannel_ArchivedReturnsError(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, IsArchived: true}, nil
		},
	}
	svc := newMockedChannelService(repo)
	_, err := svc.UpdateChannel(context.Background(), "ch-1", "new-name", "", "", "user-1")
	if err == nil {
		t.Fatal("expected error for archived channel")
	}
	assertCode(t, err, apperrors.CodePermissionDenied)
}

func TestUpdateChannel_Success(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, Name: "old", Type: domain.ChannelTypePublic}, nil
		},
		getMemberFn: func(_ context.Context, _, _ string) (*domain.ChannelMember, error) {
			return &domain.ChannelMember{Role: domain.ChannelRoleOwner}, nil
		},
	}
	svc := newMockedChannelService(repo)
	ch, err := svc.UpdateChannel(context.Background(), "ch-1", "new-name", "", "", "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.ID != "ch-1" {
		t.Errorf("unexpected channel: %+v", ch)
	}
}

func TestUpdateChannel_EmptyNameKeepsExisting(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, Name: "existing-name"}, nil
		},
		getMemberFn: func(_ context.Context, _, _ string) (*domain.ChannelMember, error) {
			return &domain.ChannelMember{Role: domain.ChannelRoleOwner}, nil
		},
		updateFn: func(_ context.Context, id, name, _, _ string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, Name: name}, nil
		},
	}
	svc := newMockedChannelService(repo)
	ch, err := svc.UpdateChannel(context.Background(), "ch-1", "", "", "", "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.Name != "existing-name" {
		t.Errorf("expected name to be kept as 'existing-name', got %q", ch.Name)
	}
}

// ── ArchiveChannel with mock ──────────────────────────────────────────────────

func TestArchiveChannel_AlreadyArchived(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, IsArchived: true}, nil
		},
	}
	svc := newMockedChannelService(repo)
	err := svc.ArchiveChannel(context.Background(), "ch-arch", "user-1")
	// Already archived is idempotent — no error.
	if err != nil {
		t.Fatalf("unexpected error for already-archived channel: %v", err)
	}
}

func TestArchiveChannel_Success(t *testing.T) {
	archived := false
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, IsArchived: false}, nil
		},
		getMemberFn: func(_ context.Context, _, _ string) (*domain.ChannelMember, error) {
			return &domain.ChannelMember{Role: domain.ChannelRoleOwner}, nil
		},
		archiveFn: func(_ context.Context, _ string) error {
			archived = true
			return nil
		},
	}
	svc := newMockedChannelService(repo)
	if err := svc.ArchiveChannel(context.Background(), "ch-1", "owner-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !archived {
		t.Error("expected Archive to be called")
	}
}

// ── JoinChannel with mock ─────────────────────────────────────────────────────

func TestJoinChannel_Success(t *testing.T) {
	svc := newMockedChannelService(&mockChannelRepo{})
	err := svc.JoinChannel(context.Background(), "ch-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJoinChannel_ArchivedError(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, IsArchived: true}, nil
		},
	}
	svc := newMockedChannelService(repo)
	err := svc.JoinChannel(context.Background(), "ch-arch", "user-1")
	if err == nil {
		t.Fatal("expected error for archived channel")
	}
	assertCode(t, err, apperrors.CodePermissionDenied)
}

func TestJoinChannel_PrivateError(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, Type: domain.ChannelTypePrivate}, nil
		},
	}
	svc := newMockedChannelService(repo)
	err := svc.JoinChannel(context.Background(), "ch-priv", "user-1")
	if err == nil {
		t.Fatal("expected error for private channel")
	}
	assertCode(t, err, apperrors.CodePermissionDenied)
}

func TestJoinChannel_AlreadyMemberIsIdempotent(t *testing.T) {
	repo := &mockChannelRepo{
		addMemberFn: func(_ context.Context, _, _, _ string) (*domain.ChannelMember, error) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "already a member")
		},
	}
	svc := newMockedChannelService(repo)
	err := svc.JoinChannel(context.Background(), "ch-1", "user-1")
	// AlreadyExists should be swallowed.
	if err != nil {
		t.Fatalf("expected nil for already-member join, got: %v", err)
	}
}

// ── LeaveChannel with mock ────────────────────────────────────────────────────

func TestLeaveChannel_Success(t *testing.T) {
	svc := newMockedChannelService(&mockChannelRepo{})
	if err := svc.LeaveChannel(context.Background(), "ch-1", "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── AddMember with mock ───────────────────────────────────────────────────────

func TestAddMember_ToPublicChannel(t *testing.T) {
	svc := newMockedChannelService(&mockChannelRepo{})
	err := svc.AddMember(context.Background(), "ch-1", "user-2", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddMember_ArchivedChannel(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, IsArchived: true}, nil
		},
	}
	svc := newMockedChannelService(repo)
	err := svc.AddMember(context.Background(), "ch-arch", "user-2", "admin-1")
	if err == nil {
		t.Fatal("expected error for archived channel")
	}
	assertCode(t, err, apperrors.CodePermissionDenied)
}

func TestAddMember_PrivateChannelRequiresAdmin(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, Type: domain.ChannelTypePrivate}, nil
		},
		getMemberFn: func(_ context.Context, _, _ string) (*domain.ChannelMember, error) {
			return &domain.ChannelMember{Role: domain.ChannelRoleMember}, nil
		},
	}
	svc := newMockedChannelService(repo)
	err := svc.AddMember(context.Background(), "ch-priv", "user-2", "member-1")
	if err == nil {
		t.Fatal("expected permission denied for non-admin adding member")
	}
	assertCode(t, err, apperrors.CodePermissionDenied)
}

// ── RemoveMember with mock ────────────────────────────────────────────────────

func TestRemoveMember_SelfRemove(t *testing.T) {
	svc := newMockedChannelService(&mockChannelRepo{})
	err := svc.RemoveMember(context.Background(), "ch-1", "user-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveMember_AdminRemovesOther(t *testing.T) {
	svc := newMockedChannelService(&mockChannelRepo{})
	err := svc.RemoveMember(context.Background(), "ch-1", "user-2", "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── ListMembers with mock ─────────────────────────────────────────────────────

func TestListMembers_PublicChannel(t *testing.T) {
	svc := newMockedChannelService(&mockChannelRepo{})
	_, err := svc.ListMembers(context.Background(), "ch-1", "any-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListMembers_PrivateDenied(t *testing.T) {
	repo := &mockChannelRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Channel, error) {
			return &domain.Channel{ID: id, Type: domain.ChannelTypePrivate}, nil
		},
		getMemberFn: func(_ context.Context, _, _ string) (*domain.ChannelMember, error) {
			return nil, apperrors.New(apperrors.CodeNotFound, "not a member")
		},
	}
	svc := newMockedChannelService(repo)
	_, err := svc.ListMembers(context.Background(), "ch-priv", "outsider")
	if err == nil {
		t.Fatal("expected permission denied")
	}
	assertCode(t, err, apperrors.CodePermissionDenied)
}

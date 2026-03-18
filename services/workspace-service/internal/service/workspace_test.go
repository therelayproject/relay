package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relay-im/relay/services/workspace-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
	"github.com/rs/zerolog"
)

func newTestWorkspaceSvc() *WorkspaceService {
	return &WorkspaceService{
		repo: nil, // not used in validation-only tests
		nc:   nil,
		log:  zerolog.Nop(),
	}
}

func assertWsCode(t *testing.T, err error, want apperrors.Code) {
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

// ── CreateWorkspace ───────────────────────────────────────────────────────────

func TestCreateWorkspace_MissingRequiredFields(t *testing.T) {
	svc := newTestWorkspaceSvc()
	cases := []struct{ name, slug, ownerID string }{
		{"", "slug", "owner"},
		{"name", "", "owner"},
		{"name", "slug", ""},
	}
	for _, c := range cases {
		_, err := svc.CreateWorkspace(context.Background(), c.name, c.slug, "", c.ownerID)
		if err == nil {
			t.Fatalf("CreateWorkspace(%q,%q,%q): expected error, got nil", c.name, c.slug, c.ownerID)
		}
		assertWsCode(t, err, apperrors.CodeInvalidArgument)
	}
}

// ── GetWorkspace ──────────────────────────────────────────────────────────────

func TestGetWorkspace_EmptyID(t *testing.T) {
	svc := newTestWorkspaceSvc()
	_, err := svc.GetWorkspace(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty id, got nil")
	}
	assertWsCode(t, err, apperrors.CodeInvalidArgument)
}

// ── UpdateSettings ────────────────────────────────────────────────────────────

func TestUpdateSettings_MissingRequiredFields(t *testing.T) {
	svc := newTestWorkspaceSvc()
	cases := []struct{ id, requesterID string }{
		{"", "user-1"},
		{"ws-1", ""},
	}
	for _, c := range cases {
		_, err := svc.UpdateSettings(context.Background(), c.id, "name", "desc", "", c.requesterID)
		if err == nil {
			t.Fatalf("UpdateSettings(%q,%q): expected error, got nil", c.id, c.requesterID)
		}
		assertWsCode(t, err, apperrors.CodeInvalidArgument)
	}
}

// ── InviteByEmail ─────────────────────────────────────────────────────────────

func TestInviteByEmail_MissingRequiredFields(t *testing.T) {
	svc := newTestWorkspaceSvc()
	cases := []struct{ workspaceID, email, inviterID string }{
		{"", "alice@example.com", "user-1"},
		{"ws-1", "", "user-1"},
		{"ws-1", "alice@example.com", ""},
	}
	for _, c := range cases {
		_, err := svc.InviteByEmail(context.Background(), c.workspaceID, c.email, "member", c.inviterID)
		if err == nil {
			t.Fatalf("InviteByEmail(%q,%q,%q): expected error, got nil", c.workspaceID, c.email, c.inviterID)
		}
		assertWsCode(t, err, apperrors.CodeInvalidArgument)
	}
}

func TestInviteByEmail_InvalidRole(t *testing.T) {
	svc := newTestWorkspaceSvc()
	_, err := svc.InviteByEmail(context.Background(), "ws-1", "alice@example.com", "superuser", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid role, got nil")
	}
	assertWsCode(t, err, apperrors.CodeInvalidArgument)
}

// ── InviteByLink ──────────────────────────────────────────────────────────────

func TestInviteByLink_MissingRequiredFields(t *testing.T) {
	svc := newTestWorkspaceSvc()
	cases := []struct{ workspaceID, inviterID string }{
		{"", "user-1"},
		{"ws-1", ""},
	}
	for _, c := range cases {
		_, err := svc.InviteByLink(context.Background(), c.workspaceID, "member", c.inviterID)
		if err == nil {
			t.Fatalf("InviteByLink(%q,%q): expected error, got nil", c.workspaceID, c.inviterID)
		}
		assertWsCode(t, err, apperrors.CodeInvalidArgument)
	}
}

func TestInviteByLink_InvalidRole(t *testing.T) {
	svc := newTestWorkspaceSvc()
	_, err := svc.InviteByLink(context.Background(), "ws-1", "root", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid role, got nil")
	}
	assertWsCode(t, err, apperrors.CodeInvalidArgument)
}

// ── JoinByToken ───────────────────────────────────────────────────────────────

func TestJoinByToken_MissingTokenOrUserID(t *testing.T) {
	svc := newTestWorkspaceSvc()
	cases := []struct{ token, userID string }{
		{"", "user-1"},
		{"tok", ""},
	}
	for _, c := range cases {
		_, err := svc.JoinByToken(context.Background(), c.token, c.userID)
		if err == nil {
			t.Fatalf("JoinByToken(%q,%q): expected error, got nil", c.token, c.userID)
		}
		assertWsCode(t, err, apperrors.CodeInvalidArgument)
	}
}

// ── ListMembers ───────────────────────────────────────────────────────────────

func TestListMembers_MissingRequiredFields(t *testing.T) {
	svc := newTestWorkspaceSvc()
	cases := []struct{ workspaceID, requesterID string }{
		{"", "user-1"},
		{"ws-1", ""},
	}
	for _, c := range cases {
		_, err := svc.ListMembers(context.Background(), c.workspaceID, c.requesterID)
		if err == nil {
			t.Fatalf("ListMembers(%q,%q): expected error, got nil", c.workspaceID, c.requesterID)
		}
		assertWsCode(t, err, apperrors.CodeInvalidArgument)
	}
}

// ── UpdateMemberRole ──────────────────────────────────────────────────────────

func TestUpdateMemberRole_InvalidRole(t *testing.T) {
	svc := newTestWorkspaceSvc()
	err := svc.UpdateMemberRole(context.Background(), "ws-1", "user-2", "badRole", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid role, got nil")
	}
	assertWsCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUpdateMemberRole_MissingRequiredFields(t *testing.T) {
	svc := newTestWorkspaceSvc()
	cases := []struct{ wsID, userID, requesterID string }{
		{"", "user-2", "user-1"},
		{"ws-1", "", "user-1"},
		{"ws-1", "user-2", ""},
	}
	for _, c := range cases {
		err := svc.UpdateMemberRole(context.Background(), c.wsID, c.userID, "member", c.requesterID)
		if err == nil {
			t.Fatalf("UpdateMemberRole(%q,%q,%q): expected error, got nil", c.wsID, c.userID, c.requesterID)
		}
		assertWsCode(t, err, apperrors.CodeInvalidArgument)
	}
}

// ── RemoveMember ──────────────────────────────────────────────────────────────

func TestRemoveMember_MissingRequiredFields(t *testing.T) {
	svc := newTestWorkspaceSvc()
	cases := []struct{ wsID, userID, requesterID string }{
		{"", "user-2", "user-1"},
		{"ws-1", "", "user-1"},
		{"ws-1", "user-2", ""},
	}
	for _, c := range cases {
		err := svc.RemoveMember(context.Background(), c.wsID, c.userID, c.requesterID)
		if err == nil {
			t.Fatalf("RemoveMember(%q,%q,%q): expected error, got nil", c.wsID, c.userID, c.requesterID)
		}
		assertWsCode(t, err, apperrors.CodeInvalidArgument)
	}
}

// ── ListWorkspaces ────────────────────────────────────────────────────────────

func TestListWorkspaces_EmptyUserID(t *testing.T) {
	svc := newTestWorkspaceSvc()
	_, err := svc.ListWorkspaces(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty user ID, got nil")
	}
	assertWsCode(t, err, apperrors.CodeInvalidArgument)
}

// ── mock repository ───────────────────────────────────────────────────────────

type mockWorkspaceRepo struct {
	createFn             func(ctx context.Context, name, slug, description, ownerID string) (*domain.Workspace, error)
	getByIDFn            func(ctx context.Context, id string) (*domain.Workspace, error)
	updateFn             func(ctx context.Context, id, name, description, iconURL string) (*domain.Workspace, error)
	listByMemberFn       func(ctx context.Context, userID string) ([]domain.Workspace, error)
	addMemberFn          func(ctx context.Context, workspaceID, userID, role, invitedBy string) (*domain.WorkspaceMember, error)
	getMemberFn          func(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceMember, error)
	updateMemberRoleFn   func(ctx context.Context, workspaceID, userID, role string) error
	removeMemberFn       func(ctx context.Context, workspaceID, userID string) error
	listMembersFn        func(ctx context.Context, workspaceID string) ([]domain.WorkspaceMember, error)
	createInvitationFn   func(ctx context.Context, inv *domain.WorkspaceInvitation) (*domain.WorkspaceInvitation, error)
	getInvitationByToken func(ctx context.Context, token string) (*domain.WorkspaceInvitation, error)
	acceptInvitationFn   func(ctx context.Context, token string) error
}

func (m *mockWorkspaceRepo) Create(ctx context.Context, name, slug, description, ownerID string) (*domain.Workspace, error) {
	if m.createFn != nil {
		return m.createFn(ctx, name, slug, description, ownerID)
	}
	return &domain.Workspace{ID: "ws-1", Name: name, Slug: slug, OwnerID: ownerID}, nil
}
func (m *mockWorkspaceRepo) GetByID(ctx context.Context, id string) (*domain.Workspace, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return &domain.Workspace{ID: id, OwnerID: "owner-1"}, nil
}
func (m *mockWorkspaceRepo) Update(ctx context.Context, id, name, description, iconURL string) (*domain.Workspace, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, name, description, iconURL)
	}
	return &domain.Workspace{ID: id, Name: name}, nil
}
func (m *mockWorkspaceRepo) ListByMember(ctx context.Context, userID string) ([]domain.Workspace, error) {
	if m.listByMemberFn != nil {
		return m.listByMemberFn(ctx, userID)
	}
	return []domain.Workspace{}, nil
}
func (m *mockWorkspaceRepo) AddMember(ctx context.Context, workspaceID, userID, role, invitedBy string) (*domain.WorkspaceMember, error) {
	if m.addMemberFn != nil {
		return m.addMemberFn(ctx, workspaceID, userID, role, invitedBy)
	}
	return &domain.WorkspaceMember{WorkspaceID: workspaceID, UserID: userID, Role: role, JoinedAt: time.Now()}, nil
}
func (m *mockWorkspaceRepo) GetMember(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceMember, error) {
	if m.getMemberFn != nil {
		return m.getMemberFn(ctx, workspaceID, userID)
	}
	return &domain.WorkspaceMember{WorkspaceID: workspaceID, UserID: userID, Role: domain.RoleOwner}, nil
}
func (m *mockWorkspaceRepo) UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) error {
	if m.updateMemberRoleFn != nil {
		return m.updateMemberRoleFn(ctx, workspaceID, userID, role)
	}
	return nil
}
func (m *mockWorkspaceRepo) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	if m.removeMemberFn != nil {
		return m.removeMemberFn(ctx, workspaceID, userID)
	}
	return nil
}
func (m *mockWorkspaceRepo) ListMembers(ctx context.Context, workspaceID string) ([]domain.WorkspaceMember, error) {
	if m.listMembersFn != nil {
		return m.listMembersFn(ctx, workspaceID)
	}
	return []domain.WorkspaceMember{}, nil
}
func (m *mockWorkspaceRepo) CreateInvitation(ctx context.Context, inv *domain.WorkspaceInvitation) (*domain.WorkspaceInvitation, error) {
	if m.createInvitationFn != nil {
		return m.createInvitationFn(ctx, inv)
	}
	return inv, nil
}
func (m *mockWorkspaceRepo) GetInvitationByToken(ctx context.Context, token string) (*domain.WorkspaceInvitation, error) {
	if m.getInvitationByToken != nil {
		return m.getInvitationByToken(ctx, token)
	}
	return &domain.WorkspaceInvitation{Token: token, WorkspaceID: "ws-1", Role: domain.RoleMember, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (m *mockWorkspaceRepo) AcceptInvitation(ctx context.Context, token string) error {
	if m.acceptInvitationFn != nil {
		return m.acceptInvitationFn(ctx, token)
	}
	return nil
}

func newMockedWorkspaceSvc(repo WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{repo: repo, nc: nil, log: zerolog.Nop()}
}

// ── CreateWorkspace with mock ─────────────────────────────────────────────────

func TestCreateWorkspace_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	ws, err := svc.CreateWorkspace(context.Background(), "Relay", "relay", "chat app", "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.Name != "Relay" {
		t.Errorf("unexpected name: %q", ws.Name)
	}
}

// ── GetWorkspace with mock ────────────────────────────────────────────────────

func TestGetWorkspace_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	ws, err := svc.GetWorkspace(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.ID != "ws-1" {
		t.Errorf("unexpected ID: %q", ws.ID)
	}
}

// ── UpdateSettings with mock ──────────────────────────────────────────────────

func TestUpdateSettings_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	ws, err := svc.UpdateSettings(context.Background(), "ws-1", "New Name", "desc", "", "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = ws
}

func TestUpdateSettings_EmptyNameError(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	_, err := svc.UpdateSettings(context.Background(), "ws-1", "", "desc", "", "owner-1")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	assertWsCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUpdateSettings_NonMemberDenied(t *testing.T) {
	repo := &mockWorkspaceRepo{
		getMemberFn: func(_ context.Context, _, _ string) (*domain.WorkspaceMember, error) {
			return nil, apperrors.New(apperrors.CodeNotFound, "not a member")
		},
	}
	svc := newMockedWorkspaceSvc(repo)
	_, err := svc.UpdateSettings(context.Background(), "ws-1", "Name", "", "", "non-member")
	if err == nil {
		t.Fatal("expected permission denied")
	}
	assertWsCode(t, err, apperrors.CodePermissionDenied)
}

// ── InviteByEmail with mock ───────────────────────────────────────────────────

func TestInviteByEmail_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	token, err := svc.InviteByEmail(context.Background(), "ws-1", "alice@example.com", domain.RoleMember, "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

// ── InviteByLink with mock ────────────────────────────────────────────────────

func TestInviteByLink_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	token, err := svc.InviteByLink(context.Background(), "ws-1", domain.RoleMember, "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

// ── JoinByToken with mock ─────────────────────────────────────────────────────

func TestJoinByToken_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	member, err := svc.JoinByToken(context.Background(), "valid-token", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = member
}

func TestJoinByToken_ExpiredInvitation(t *testing.T) {
	repo := &mockWorkspaceRepo{
		getInvitationByToken: func(_ context.Context, token string) (*domain.WorkspaceInvitation, error) {
			return &domain.WorkspaceInvitation{Token: token, ExpiresAt: time.Now().Add(-time.Hour)}, nil
		},
	}
	svc := newMockedWorkspaceSvc(repo)
	_, err := svc.JoinByToken(context.Background(), "expired-token", "user-1")
	if err == nil {
		t.Fatal("expected error for expired invitation")
	}
	assertWsCode(t, err, apperrors.CodeInvalidArgument)
}

func TestJoinByToken_AlreadyAccepted(t *testing.T) {
	accepted := time.Now().Add(-time.Minute)
	repo := &mockWorkspaceRepo{
		getInvitationByToken: func(_ context.Context, token string) (*domain.WorkspaceInvitation, error) {
			return &domain.WorkspaceInvitation{Token: token, ExpiresAt: time.Now().Add(time.Hour), AcceptedAt: &accepted}, nil
		},
	}
	svc := newMockedWorkspaceSvc(repo)
	_, err := svc.JoinByToken(context.Background(), "used-token", "user-1")
	if err == nil {
		t.Fatal("expected error for already-accepted invitation")
	}
	assertWsCode(t, err, apperrors.CodeInvalidArgument)
}

// ── ListMembers with mock ─────────────────────────────────────────────────────

func TestListMembers_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	members, err := svc.ListMembers(context.Background(), "ws-1", "member-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = members
}

func TestListMembers_NonMemberDenied(t *testing.T) {
	repo := &mockWorkspaceRepo{
		getMemberFn: func(_ context.Context, _, _ string) (*domain.WorkspaceMember, error) {
			return nil, apperrors.New(apperrors.CodeNotFound, "not a member")
		},
	}
	svc := newMockedWorkspaceSvc(repo)
	_, err := svc.ListMembers(context.Background(), "ws-1", "outsider")
	if err == nil {
		t.Fatal("expected permission denied for non-member")
	}
	assertWsCode(t, err, apperrors.CodePermissionDenied)
}

// ── UpdateMemberRole with mock ────────────────────────────────────────────────

func TestUpdateMemberRole_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	err := svc.UpdateMemberRole(context.Background(), "ws-1", "user-2", domain.RoleAdmin, "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateMemberRole_OwnerCannotBeChanged(t *testing.T) {
	repo := &mockWorkspaceRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Workspace, error) {
			return &domain.Workspace{ID: id, OwnerID: "user-owner"}, nil
		},
	}
	svc := newMockedWorkspaceSvc(repo)
	err := svc.UpdateMemberRole(context.Background(), "ws-1", "user-owner", domain.RoleMember, "owner-1")
	if err == nil {
		t.Fatal("expected error changing owner role")
	}
	assertWsCode(t, err, apperrors.CodePermissionDenied)
}

// ── RemoveMember with mock ────────────────────────────────────────────────────

func TestRemoveMember_SelfLeave(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	err := svc.RemoveMember(context.Background(), "ws-1", "user-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveMember_OwnerCannotBeRemoved(t *testing.T) {
	repo := &mockWorkspaceRepo{
		getByIDFn: func(_ context.Context, id string) (*domain.Workspace, error) {
			return &domain.Workspace{ID: id, OwnerID: "owner"}, nil
		},
	}
	svc := newMockedWorkspaceSvc(repo)
	err := svc.RemoveMember(context.Background(), "ws-1", "owner", "admin-1")
	if err == nil {
		t.Fatal("expected error when removing workspace owner")
	}
	assertWsCode(t, err, apperrors.CodePermissionDenied)
}

// ── ListWorkspaces with mock ──────────────────────────────────────────────────

func TestListWorkspaces_Success(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	wss, err := svc.ListWorkspaces(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = wss
}

// ── publishMemberJoined with nil nc (no-op path) ──────────────────────────────

func TestPublishMemberJoined_NilNATS(t *testing.T) {
	svc := newMockedWorkspaceSvc(&mockWorkspaceRepo{})
	// Should not panic — logs a warning and returns.
	svc.publishMemberJoined(&domain.WorkspaceMember{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		Role:        domain.RoleMember,
		JoinedAt:    time.Now(),
	})
}

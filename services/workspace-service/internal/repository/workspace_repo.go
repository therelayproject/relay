// Package repository provides PostgreSQL-backed persistence for workspaces.
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relay-im/relay/services/workspace-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// WorkspaceRepo handles all database operations for workspaces, members, and invitations.
type WorkspaceRepo struct {
	db *pgxpool.Pool
}

// New returns a new WorkspaceRepo backed by the given connection pool.
func New(db *pgxpool.Pool) *WorkspaceRepo {
	return &WorkspaceRepo{db: db}
}

// ── Workspace ────────────────────────────────────────────────────────────────

// Create inserts a new workspace and adds the owner as a member with the owner role.
func (r *WorkspaceRepo) Create(ctx context.Context, name, slug, description, ownerID string) (*domain.Workspace, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "begin tx", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const wsSQL = `
		INSERT INTO workspaces (name, slug, description, owner_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, slug, description, icon_url, owner_id, allow_guest_invites, created_at, updated_at`

	ws, err := scanWorkspace(tx.QueryRow(ctx, wsSQL, name, slug, nullableString(description), ownerID))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "workspace slug already taken")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, "insert workspace", err)
	}

	const memberSQL = `
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ($1, $2, $3)`
	if _, err = tx.Exec(ctx, memberSQL, ws.ID, ownerID, domain.RoleOwner); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "insert owner member", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "commit tx", err)
	}
	return ws, nil
}

// GetByID fetches a workspace by primary key.
func (r *WorkspaceRepo) GetByID(ctx context.Context, id string) (*domain.Workspace, error) {
	const q = `
		SELECT id, name, slug, description, icon_url, owner_id, allow_guest_invites, created_at, updated_at
		FROM workspaces WHERE id = $1`
	ws, err := scanWorkspace(r.db.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "workspace not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, "get workspace by id", err)
	}
	return ws, nil
}

// GetBySlug fetches a workspace by its URL-safe slug.
func (r *WorkspaceRepo) GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	const q = `
		SELECT id, name, slug, description, icon_url, owner_id, allow_guest_invites, created_at, updated_at
		FROM workspaces WHERE slug = $1`
	ws, err := scanWorkspace(r.db.QueryRow(ctx, q, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "workspace not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, "get workspace by slug", err)
	}
	return ws, nil
}

// Update modifies the mutable fields of a workspace.
func (r *WorkspaceRepo) Update(ctx context.Context, id, name, description, iconURL string) (*domain.Workspace, error) {
	const q = `
		UPDATE workspaces
		SET name = $2, description = $3, icon_url = $4
		WHERE id = $1
		RETURNING id, name, slug, description, icon_url, owner_id, allow_guest_invites, created_at, updated_at`
	ws, err := scanWorkspace(r.db.QueryRow(ctx, q, id, name, nullableString(description), nullableString(iconURL)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "workspace not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, "update workspace", err)
	}
	return ws, nil
}

// ListByMember returns all workspaces the given user belongs to.
func (r *WorkspaceRepo) ListByMember(ctx context.Context, userID string) ([]domain.Workspace, error) {
	const q = `
		SELECT w.id, w.name, w.slug, w.description, w.icon_url, w.owner_id, w.allow_guest_invites, w.created_at, w.updated_at
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		WHERE wm.user_id = $1
		ORDER BY w.created_at ASC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "list workspaces by member", err)
	}
	defer rows.Close()
	return collectWorkspaces(rows)
}

// ── Members ──────────────────────────────────────────────────────────────────

// AddMember inserts a new workspace_members row.
func (r *WorkspaceRepo) AddMember(ctx context.Context, workspaceID, userID, role, invitedBy string) (*domain.WorkspaceMember, error) {
	const q = `
		INSERT INTO workspace_members (workspace_id, user_id, role, invited_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, user_id) DO NOTHING
		RETURNING workspace_id, user_id, role, invited_by, joined_at`
	row := r.db.QueryRow(ctx, q, workspaceID, userID, role, nullableString(invitedBy))
	m, err := scanMember(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already a member – fetch the existing record
			return r.GetMember(ctx, workspaceID, userID)
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, "add member", err)
	}
	return m, nil
}

// GetMember retrieves a single membership record.
func (r *WorkspaceRepo) GetMember(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceMember, error) {
	const q = `
		SELECT workspace_id, user_id, role, invited_by, joined_at
		FROM workspace_members
		WHERE workspace_id = $1 AND user_id = $2`
	m, err := scanMember(r.db.QueryRow(ctx, q, workspaceID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "member not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, "get member", err)
	}
	return m, nil
}

// UpdateMemberRole changes the role of an existing member.
func (r *WorkspaceRepo) UpdateMemberRole(ctx context.Context, workspaceID, userID, role string) error {
	const q = `
		UPDATE workspace_members SET role = $3
		WHERE workspace_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, workspaceID, userID, role)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "update member role", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "member not found")
	}
	return nil
}

// RemoveMember deletes a workspace membership.
func (r *WorkspaceRepo) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	const q = `DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, q, workspaceID, userID)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "remove member", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "member not found")
	}
	return nil
}

// ListMembers returns all members of a workspace ordered by join date.
func (r *WorkspaceRepo) ListMembers(ctx context.Context, workspaceID string) ([]domain.WorkspaceMember, error) {
	const q = `
		SELECT workspace_id, user_id, role, invited_by, joined_at
		FROM workspace_members
		WHERE workspace_id = $1
		ORDER BY joined_at ASC`
	rows, err := r.db.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "list members", err)
	}
	defer rows.Close()
	return collectMembers(rows)
}

// ── Invitations ──────────────────────────────────────────────────────────────

// CreateInvitation persists a new invitation record.
func (r *WorkspaceRepo) CreateInvitation(ctx context.Context, inv *domain.WorkspaceInvitation) (*domain.WorkspaceInvitation, error) {
	const q = `
		INSERT INTO workspace_invitations (workspace_id, email, token, role, invited_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workspace_id, email, token, role, invited_by, expires_at, accepted_at, created_at`
	row := r.db.QueryRow(ctx, q,
		inv.WorkspaceID,
		nullableString(inv.Email),
		inv.Token,
		inv.Role,
		inv.InvitedBy,
		inv.ExpiresAt,
	)
	created, err := scanInvitation(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.New(apperrors.CodeAlreadyExists, "invitation token already exists")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, "create invitation", err)
	}
	return created, nil
}

// GetInvitationByToken retrieves an invitation by its token.
func (r *WorkspaceRepo) GetInvitationByToken(ctx context.Context, token string) (*domain.WorkspaceInvitation, error) {
	const q = `
		SELECT id, workspace_id, email, token, role, invited_by, expires_at, accepted_at, created_at
		FROM workspace_invitations
		WHERE token = $1`
	inv, err := scanInvitation(r.db.QueryRow(ctx, q, token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New(apperrors.CodeNotFound, "invitation not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, "get invitation by token", err)
	}
	return inv, nil
}

// AcceptInvitation stamps the accepted_at field on an invitation.
func (r *WorkspaceRepo) AcceptInvitation(ctx context.Context, token string) error {
	const q = `
		UPDATE workspace_invitations
		SET accepted_at = now()
		WHERE token = $1 AND accepted_at IS NULL`
	tag, err := r.db.Exec(ctx, q, token)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "accept invitation", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.New(apperrors.CodeNotFound, "invitation not found or already accepted")
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanWorkspace(row scanner) (*domain.Workspace, error) {
	var ws domain.Workspace
	var description, iconURL *string
	err := row.Scan(
		&ws.ID,
		&ws.Name,
		&ws.Slug,
		&description,
		&iconURL,
		&ws.OwnerID,
		&ws.AllowGuestInvites,
		&ws.CreatedAt,
		&ws.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if description != nil {
		ws.Description = *description
	}
	if iconURL != nil {
		ws.IconURL = *iconURL
	}
	return &ws, nil
}

func collectWorkspaces(rows pgx.Rows) ([]domain.Workspace, error) {
	var out []domain.Workspace
	for rows.Next() {
		ws, err := scanWorkspace(rows)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "scan workspace row", err)
		}
		out = append(out, *ws)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "iterate workspace rows", err)
	}
	return out, nil
}

func scanMember(row scanner) (*domain.WorkspaceMember, error) {
	var m domain.WorkspaceMember
	var invitedBy *string
	err := row.Scan(&m.WorkspaceID, &m.UserID, &m.Role, &invitedBy, &m.JoinedAt)
	if err != nil {
		return nil, err
	}
	if invitedBy != nil {
		m.InvitedBy = *invitedBy
	}
	return &m, nil
}

func collectMembers(rows pgx.Rows) ([]domain.WorkspaceMember, error) {
	var out []domain.WorkspaceMember
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "scan member row", err)
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "iterate member rows", err)
	}
	return out, nil
}

func scanInvitation(row scanner) (*domain.WorkspaceInvitation, error) {
	var inv domain.WorkspaceInvitation
	var email *string
	var acceptedAt *time.Time
	err := row.Scan(
		&inv.ID,
		&inv.WorkspaceID,
		&email,
		&inv.Token,
		&inv.Role,
		&inv.InvitedBy,
		&inv.ExpiresAt,
		&acceptedAt,
		&inv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if email != nil {
		inv.Email = *email
	}
	inv.AcceptedAt = acceptedAt
	return &inv, nil
}

// nullableString converts an empty string to nil for nullable TEXT columns.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// isUniqueViolation detects PostgreSQL unique-constraint errors (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique constraint")
}

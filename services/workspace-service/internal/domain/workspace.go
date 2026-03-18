// Package domain defines the core business types for the workspace service.
package domain

import "time"

// Role constants for workspace membership.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleGuest  = "guest"
)

// IsValidRole reports whether r is a recognised role string.
func IsValidRole(r string) bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleGuest:
		return true
	}
	return false
}

// CanManageMembers reports whether the role has admin-or-owner privileges.
func CanManageMembers(role string) bool {
	return role == RoleOwner || role == RoleAdmin
}

// Workspace represents a Relay workspace.
type Workspace struct {
	ID                 string
	Name               string
	Slug               string
	Description        string
	IconURL            string
	OwnerID            string
	AllowGuestInvites  bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// WorkspaceMember represents a user's membership in a workspace.
type WorkspaceMember struct {
	WorkspaceID string
	UserID      string
	Role        string
	InvitedBy   string
	JoinedAt    time.Time
}

// WorkspaceInvitation represents a pending invitation to a workspace.
type WorkspaceInvitation struct {
	ID          string
	WorkspaceID string
	Email       string // empty for link-only invitations
	Token       string
	Role        string
	InvitedBy   string
	ExpiresAt   time.Time
	AcceptedAt  *time.Time
	CreatedAt   time.Time
}

// Package domain defines the core types for the channel service.
package domain

import "time"

// ChannelType classifies a channel.
type ChannelType string

const (
	ChannelTypePublic  ChannelType = "public"
	ChannelTypePrivate ChannelType = "private"
	ChannelTypeDM      ChannelType = "dm"
)

// ChannelRole classifies a member's role within a channel.
type ChannelRole string

const (
	ChannelRoleOwner  ChannelRole = "owner"
	ChannelRoleAdmin  ChannelRole = "admin"
	ChannelRoleMember ChannelRole = "member"
)

// Channel represents a messaging channel within a workspace.
type Channel struct {
	ID          string
	WorkspaceID string
	Name        string
	Slug        string
	Description string
	Topic       string
	Type        ChannelType
	IsArchived  bool
	CreatedBy   string
	MemberCount int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ChannelMember represents a user's membership in a channel.
type ChannelMember struct {
	ChannelID  string
	UserID     string
	Role       ChannelRole
	LastReadAt *time.Time
	JoinedAt   time.Time
}

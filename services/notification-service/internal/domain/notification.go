// Package domain defines business types for the notification service.
package domain

import "time"

// Level controls which events trigger a notification.
type Level string

const (
	LevelAll      Level = "all"
	LevelMentions Level = "mentions"
	LevelNothing  Level = "nothing"
)

// Preference stores a user's notification settings for a given scope.
type Preference struct {
	ID        string
	UserID    string
	Scope     string // "global", workspace_id, channel_id
	Level     Level
	Muted     bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PushToken is a device push registration.
type PushToken struct {
	ID        string
	UserID    string
	Platform  string // ios | android | web
	Token     string
	CreatedAt time.Time
}

// DND is a Do Not Disturb setting for a user.
type DND struct {
	UserID    string
	Until     time.Time
	CreatedAt time.Time
}

// Notification is an in-app notification record.
type Notification struct {
	ID        string
	UserID    string
	Type      string
	Title     string
	Body      string
	ActionURL string
	ReadAt    *time.Time
	CreatedAt time.Time
}

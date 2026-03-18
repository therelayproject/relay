// Package domain defines the core business types for the user service.
package domain

import "time"

// Profile represents a user's profile in the user_profiles table.
// UserID is a cross-service reference to auth-service's users.id.
type Profile struct {
	UserID          string
	DisplayName     string
	AvatarURL       string
	Timezone        string
	Locale          string
	StatusEmoji     string
	StatusText      string
	StatusExpiresAt *time.Time
	IsActive        bool
	DeactivatedAt   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// StatusUpdate carries the fields for a status change operation.
type StatusUpdate struct {
	UserID          string
	StatusEmoji     string
	StatusText      string
	StatusExpiresAt *time.Time
}

// NotificationPreference stores per-scope notification settings.
type NotificationPreference struct {
	UserID string
	Scope  string // 'global', 'workspace:{id}', 'channel:{id}'
	Level  string // 'all', 'mentions', 'nothing'
	Muted  bool
}

// PushToken is a device push notification token.
type PushToken struct {
	ID        string
	UserID    string
	Platform  string // 'ios', 'android', 'web'
	Token     string
	CreatedAt time.Time
}

// DNDSchedule represents a user's Do Not Disturb setting.
type DNDSchedule struct {
	UserID    string
	Until     *time.Time // nil = indefinite
	CreatedAt time.Time
}

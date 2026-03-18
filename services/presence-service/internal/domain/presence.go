// Package domain defines the core business types for the presence service.
package domain

import "time"

// Status represents a user's online status.
type Status string

const (
	StatusOnline  Status = "online"
	StatusAway    Status = "away"
	StatusOffline Status = "offline"
)

// Presence holds the current presence state for one user.
type Presence struct {
	UserID     string
	Status     Status
	LastSeenAt time.Time
}

// CustomStatus is a user-set display status with optional emoji and expiry (PRES-02).
type CustomStatus struct {
	UserID    string
	Emoji     string
	Text      string
	ExpiresAt *time.Time
	UpdatedAt time.Time
}

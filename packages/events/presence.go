package events

// PresenceStatus is the set of valid user presence states.
type PresenceStatus string

const (
	PresenceOnline  PresenceStatus = "online"
	PresenceAway    PresenceStatus = "away"
	PresenceDND     PresenceStatus = "dnd"
	PresenceOffline PresenceStatus = "offline"
)

// PresenceUpdatedEvent is published by the Messaging Service when a user's
// WebSocket connects or disconnects, or when the user manually sets status.
// Consumed by: API (for /users/:id/presence reads), clients via WebSocket push.
type PresenceUpdatedEvent struct {
	UserID      int64          `json:"user_id"`
	WorkspaceID int64          `json:"workspace_id"`
	Status      PresenceStatus `json:"status"`
}

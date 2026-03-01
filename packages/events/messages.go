// Package events defines the shared NATS event types published and consumed
// across Relay services. Both publishers and subscribers import this package
// to guarantee payload compatibility.
//
// NATS subject convention:
//
//	relay.<domain>.<verb>
//	  e.g. relay.messages.created
//	       relay.messages.deleted
//	       relay.presence.updated
//	       relay.notifications.push
//	       relay.search.index
//	       relay.integrations.action
//	       relay.federation.outbound
package events

// Subject constants — import these instead of hard-coding strings.
const (
	SubjectMessageCreated    = "relay.messages.created"
	SubjectMessageDeleted    = "relay.messages.deleted"
	SubjectMessageUpdated    = "relay.messages.updated"
	SubjectPresenceUpdated   = "relay.presence.updated"
	SubjectNotificationPush  = "relay.notifications.push"
	SubjectSearchIndex       = "relay.search.index"
	SubjectIntegrationAction = "relay.integrations.action"
	SubjectFederationOutbound = "relay.federation.outbound"
)

// MessageCreatedEvent is published by the Messaging Service when a new message
// is persisted. Consumed by: Notification, Search, Integration, Federation.
type MessageCreatedEvent struct {
	MessageID   int64  `json:"message_id"`
	WorkspaceID int64  `json:"workspace_id"`
	ChannelID   int64  `json:"channel_id"`
	AuthorID    int64  `json:"author_id"`
	Content     string `json:"content"`
	// Blocks is the raw Block Kit JSON for the Integration service to forward
	// to app webhooks unchanged.
	Blocks []byte `json:"blocks,omitempty"`
}

// MessageDeletedEvent is published when a message is soft-deleted.
// Consumed by: Search (remove from index), Integration.
type MessageDeletedEvent struct {
	MessageID   int64 `json:"message_id"`
	WorkspaceID int64 `json:"workspace_id"`
	ChannelID   int64 `json:"channel_id"`
}

// MessageUpdatedEvent is published when a message is edited.
// Consumed by: Search (re-index), Integration.
type MessageUpdatedEvent struct {
	MessageID   int64  `json:"message_id"`
	WorkspaceID int64  `json:"workspace_id"`
	ChannelID   int64  `json:"channel_id"`
	NewContent  string `json:"new_content"`
}

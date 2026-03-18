// Package domain defines the core business types for the message service.
package domain

import (
	"encoding/json"
	"time"
)

// Message represents a single message in a channel or thread.
type Message struct {
	ID             string
	ChannelID      string
	AuthorID       string
	Body           string
	BodyParsed     json.RawMessage // parsed markdown/mention AST
	ThreadID       *string         // nil = top-level; set to root message ID for replies
	ParentID       *string         // immediate parent message ID
	IdempotencyKey *string
	IsEdited       bool
	IsDeleted      bool
	ReplyCount     int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Reaction represents an emoji reaction on a message.
type Reaction struct {
	ID        string
	MessageID string
	ChannelID string
	UserID    string
	Emoji     string
	CreatedAt time.Time
}

// ReactionSummary aggregates a single emoji reaction for display.
type ReactionSummary struct {
	Emoji   string
	Count   int
	UserIDs []string
}

// Pin represents a pinned message in a channel.
type Pin struct {
	ID        string
	ChannelID string
	MessageID string
	PinnedBy  string
	CreatedAt time.Time
}

// ScheduledMessage is a message queued for future delivery.
type ScheduledMessage struct {
	ID        string
	ChannelID string
	AuthorID  string
	Body      string
	SendAt    time.Time
	SentAt    *time.Time
	CreatedAt time.Time
}

// Page holds a cursor-paginated slice of messages.
type Page struct {
	Messages []*Message
	Cursor   string
	HasMore  bool
}

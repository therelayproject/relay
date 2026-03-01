package events

// BlockActionEvent is published when a user interacts with an interactive
// Block Kit component (button click, select option, etc.).
// Published by: Messaging Service.
// Consumed by: Integration Service, which POSTs it to the registered app webhook.
type BlockActionEvent struct {
	ActionID    string `json:"action_id"`    // the block's action_id field
	BlockID     string `json:"block_id"`
	MessageID   int64  `json:"message_id"`
	ChannelID   int64  `json:"channel_id"`
	WorkspaceID int64  `json:"workspace_id"`
	UserID      int64  `json:"user_id"`
	// Payload is the raw action payload forwarded unchanged to the app webhook.
	Payload []byte `json:"payload"`
}

// SearchIndexEvent is published when content should be indexed or re-indexed.
// Published by: Messaging Service (message upserts/deletes).
// Consumed by: Search Service.
type SearchIndexEvent struct {
	// Op is "upsert" or "delete".
	Op          string `json:"op"`
	MessageID   int64  `json:"message_id"`
	WorkspaceID int64  `json:"workspace_id"`
	ChannelID   int64  `json:"channel_id"`
	Content     string `json:"content,omitempty"` // empty on delete
}

package service

import (
	"encoding/json"
	"testing"
)

// ── handleMessageCreated tests ────────────────────────────────────────────────

// handleMessageCreated is an unexported method but we can test it indirectly by
// building the expected envelope JSON and verifying the notification service
// would handle the mention payload without panicking.

func TestHandleMessageCreated_NoMentions(t *testing.T) {
	svc := &NotificationService{}
	payload := map[string]any{
		"type": "message.created",
		"payload": map[string]any{
			"channel_id": "ch-1",
			"author_id":  "user-1",
			"body_parsed": map[string]any{
				"mentions": []string{},
			},
		},
	}
	data, _ := json.Marshal(payload)
	// Should not panic even with nil db/nc.
	svc.handleMessageCreated(nil, data) //nolint:staticcheck
}

func TestHandleMessageCreated_InvalidJSON(t *testing.T) {
	svc := &NotificationService{}
	// Should not panic.
	svc.handleMessageCreated(nil, []byte("bad json"))
}

// ── publish nil-safety ────────────────────────────────────────────────────────

func TestPublish_NilNATSConn(t *testing.T) {
	svc := &NotificationService{nc: nil}
	// Should not panic when nc is nil.
	svc.publish("test.subject", map[string]any{"key": "value"})
}

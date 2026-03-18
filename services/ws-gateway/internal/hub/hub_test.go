package hub

import (
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestHub() *Hub {
	// nc=nil: we test internal methods directly without running the event loop,
	// so NATS is not needed.
	return New(nil, zerolog.Nop())
}

// makeTestClient creates a client with a buffered send channel and no real conn.
func makeTestClient(id, userID, workspaceID string, channelIDs []string) *Client {
	chMap := make(map[string]bool, len(channelIDs))
	for _, ch := range channelIDs {
		chMap[ch] = true
	}
	return &Client{
		ID:          id,
		UserID:      userID,
		WorkspaceID: workspaceID,
		ChannelIDs:  chMap,
		send:        make(chan []byte, 256),
	}
}

// ── addClient / removeClient ──────────────────────────────────────────────────

func TestHub_AddClientRegistersIndexes(t *testing.T) {
	h := newTestHub()
	c := makeTestClient("c1", "user-1", "ws-1", []string{"ch-a", "ch-b"})

	h.addClient(c)

	h.mu.RLock()
	defer h.mu.RUnlock()

	if _, ok := h.clients["c1"]; !ok {
		t.Error("client not in clients map")
	}
	if _, ok := h.userIndex["user-1"]["c1"]; !ok {
		t.Error("client not in userIndex")
	}
	if _, ok := h.channelIdx["ch-a"]["c1"]; !ok {
		t.Error("client not in channelIdx for ch-a")
	}
	if _, ok := h.channelIdx["ch-b"]["c1"]; !ok {
		t.Error("client not in channelIdx for ch-b")
	}
}

func TestHub_RemoveClientClearsIndexes(t *testing.T) {
	h := newTestHub()
	c := makeTestClient("c1", "user-1", "ws-1", []string{"ch-x"})

	h.addClient(c)
	h.removeClient(c)

	h.mu.RLock()
	defer h.mu.RUnlock()

	if _, ok := h.clients["c1"]; ok {
		t.Error("client still in clients map after removal")
	}
	if _, ok := h.userIndex["user-1"]; ok {
		t.Error("userIndex entry should be removed when last client removed")
	}
	if _, ok := h.channelIdx["ch-x"]; ok {
		t.Error("channelIdx entry should be removed when last client removed")
	}
}

func TestHub_RemoveClientIdempotent(t *testing.T) {
	h := newTestHub()
	c := makeTestClient("c2", "user-2", "ws-1", nil)
	h.addClient(c)
	h.removeClient(c)
	// second remove should not panic (client already gone)
	h.removeClient(c)
}

func TestHub_MultipleClientsForSameUser(t *testing.T) {
	h := newTestHub()
	c1 := makeTestClient("c1", "user-shared", "ws-1", nil)
	c2 := makeTestClient("c2", "user-shared", "ws-1", nil)

	h.addClient(c1)
	h.addClient(c2)

	h.removeClient(c1)

	h.mu.RLock()
	defer h.mu.RUnlock()
	// c2 is still registered; userIndex for user-shared should still exist
	if _, ok := h.userIndex["user-shared"]; !ok {
		t.Error("userIndex should still have user-shared when c2 remains")
	}
}

// ── SubscribeChannel / UnsubscribeChannel ─────────────────────────────────────

func TestHub_SubscribeChannel(t *testing.T) {
	h := newTestHub()
	c := makeTestClient("c1", "user-1", "ws-1", nil)
	h.addClient(c)

	h.SubscribeChannel("c1", "ch-new")

	h.mu.RLock()
	defer h.mu.RUnlock()
	if _, ok := h.channelIdx["ch-new"]["c1"]; !ok {
		t.Error("client should be subscribed to ch-new")
	}
}

func TestHub_UnsubscribeChannel(t *testing.T) {
	h := newTestHub()
	c := makeTestClient("c1", "user-1", "ws-1", []string{"ch-sub"})
	h.addClient(c)

	h.UnsubscribeChannel("c1", "ch-sub")

	h.mu.RLock()
	defer h.mu.RUnlock()
	if _, ok := h.channelIdx["ch-sub"]["c1"]; ok {
		t.Error("client should have been unsubscribed from ch-sub")
	}
}

func TestHub_SubscribeNonexistentClient(t *testing.T) {
	h := newTestHub()
	// Should not panic for unknown client ID.
	h.SubscribeChannel("ghost", "ch-1")
}

// ── fanOut ────────────────────────────────────────────────────────────────────

func TestHub_FanOutByChannel(t *testing.T) {
	h := newTestHub()
	c1 := makeTestClient("c1", "user-1", "ws-1", []string{"ch-msg"})
	c2 := makeTestClient("c2", "user-2", "ws-1", []string{"ch-msg"})
	c3 := makeTestClient("c3", "user-3", "ws-1", []string{"ch-other"})

	h.addClient(c1)
	h.addClient(c2)
	h.addClient(c3)

	payload := []byte(`{"type":"message.created"}`)
	h.fanOut(&broadcastMsg{channelID: "ch-msg", payload: payload})

	recv := func(ch <-chan []byte) bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}

	if !recv(c1.send) {
		t.Error("c1 should receive channel broadcast")
	}
	if !recv(c2.send) {
		t.Error("c2 should receive channel broadcast")
	}
	if recv(c3.send) {
		t.Error("c3 should NOT receive broadcast for different channel")
	}
}

func TestHub_FanOutByUserID(t *testing.T) {
	h := newTestHub()
	c1 := makeTestClient("c1", "user-target", "ws-1", nil)
	c2 := makeTestClient("c2", "user-other", "ws-1", nil)
	h.addClient(c1)
	h.addClient(c2)

	payload := []byte(`{"type":"notification"}`)
	h.fanOut(&broadcastMsg{userID: "user-target", payload: payload})

	select {
	case <-c1.send:
		// correct
	default:
		t.Error("user-targeted message should reach c1")
	}
	select {
	case <-c2.send:
		t.Error("c2 should NOT receive user-targeted message")
	default:
		// correct
	}
}

func TestHub_FanOutDropsSlowClient(t *testing.T) {
	h := newTestHub()
	// Drain the send channel capacity (fill it up) so the select default branch fires.
	c := makeTestClient("c1", "user-1", "ws-1", []string{"ch-1"})
	// Fill the buffer.
	for i := 0; i < cap(c.send); i++ {
		c.send <- []byte("fill")
	}
	h.addClient(c)

	// fanOut should not block (drops the message via default branch).
	h.fanOut(&broadcastMsg{channelID: "ch-1", payload: []byte(`{}`)})
}

// ── onNATSMessage ─────────────────────────────────────────────────────────────

func TestHub_OnNATSMessage_EnqueuesBroadcast(t *testing.T) {
	h := newTestHub()

	payloadJSON, _ := json.Marshal(map[string]any{
		"channel_id": "ch-1",
		"author_id":  "user-1",
	})
	msg, _ := json.Marshal(map[string]any{
		"type":    "message.created",
		"payload": json.RawMessage(payloadJSON),
		"ts":      "123",
	})

	h.onNATSMessage("message.created", msg)

	select {
	case bm := <-h.broadcast:
		if bm.channelID != "ch-1" {
			t.Errorf("expected channelID=ch-1, got %q", bm.channelID)
		}
	default:
		t.Fatal("expected broadcast message to be enqueued")
	}
}

func TestHub_OnNATSMessage_InvalidJSON(t *testing.T) {
	h := newTestHub()
	// Should not panic.
	h.onNATSMessage("some.event", []byte("not-json"))
}

func TestHub_OnNATSMessage_UserIDExtraction(t *testing.T) {
	h := newTestHub()

	payloadJSON, _ := json.Marshal(map[string]any{
		"user_id": "user-abc",
	})
	msg, _ := json.Marshal(map[string]any{
		"type":    "presence.changed",
		"payload": json.RawMessage(payloadJSON),
		"ts":      "123",
	})

	h.onNATSMessage("presence.changed", msg)

	select {
	case bm := <-h.broadcast:
		if bm.userID != "user-abc" {
			t.Errorf("expected userID=user-abc, got %q", bm.userID)
		}
	default:
		t.Fatal("expected broadcast message enqueued")
	}
}

// ── NewClient ─────────────────────────────────────────────────────────────────

func TestNewClient(t *testing.T) {
	h := newTestHub()
	c := NewClient("id1", "user-1", "ws-1", []string{"ch-a", "ch-b"}, nil, h)

	if c.ID != "id1" || c.UserID != "user-1" || c.WorkspaceID != "ws-1" {
		t.Errorf("unexpected client fields: %+v", c)
	}
	if !c.ChannelIDs["ch-a"] || !c.ChannelIDs["ch-b"] {
		t.Error("channel IDs not set correctly")
	}
	if cap(c.send) != 256 {
		t.Errorf("expected send capacity 256, got %d", cap(c.send))
	}
}

// ── Register / Unregister (channel-send paths) ────────────────────────────────
//
// Run requires a live NATS connection, so we drive the hub channels directly.

func drainRegister(h *Hub) *Client {
	select {
	case c := <-h.register:
		return c
	default:
		return nil
	}
}

func drainUnregister(h *Hub) *Client {
	select {
	case c := <-h.unregister:
		return c
	default:
		return nil
	}
}

func TestHub_Register_SendsToChannel(t *testing.T) {
	h := newTestHub()
	c := makeTestClient("c-reg", "user-1", "ws-1", nil)

	// register channel is buffered (256) so this won't block.
	h.Register(c)
	got := drainRegister(h)
	if got == nil {
		t.Fatal("client was not placed on register channel")
	}
	if got.ID != "c-reg" {
		t.Errorf("expected client ID c-reg, got %q", got.ID)
	}
}

func TestHub_Unregister_SendsToChannel(t *testing.T) {
	h := newTestHub()
	c := makeTestClient("c-unreg", "user-2", "ws-1", nil)

	// unregister channel is buffered (256) so this won't block.
	h.Unregister(c)
	got := drainUnregister(h)
	if got == nil {
		t.Fatal("client was not placed on unregister channel")
	}
	if got.ID != "c-unreg" {
		t.Errorf("expected client ID c-unreg, got %q", got.ID)
	}
}

// ── onNATSMessage: remaining event types ─────────────────────────────────────

func TestHub_OnNATSMessage_ChannelUpdated(t *testing.T) {
	h := newTestHub()

	payloadJSON, _ := json.Marshal(map[string]any{
		"workspace_id": "ws-1",
	})
	msg, _ := json.Marshal(map[string]any{
		"type":    "channel.updated",
		"payload": json.RawMessage(payloadJSON),
		"ts":      "456",
	})

	h.onNATSMessage("channel.updated", msg)

	select {
	case bm := <-h.broadcast:
		// channel.updated has no channel_id/user_id — both should be empty.
		if bm.channelID != "" || bm.userID != "" {
			t.Errorf("expected empty channelID and userID, got channelID=%q userID=%q", bm.channelID, bm.userID)
		}
	default:
		t.Fatal("expected broadcast for channel.updated")
	}
}

func TestHub_OnNATSMessage_EmptyPayload(t *testing.T) {
	h := newTestHub()
	msg, _ := json.Marshal(map[string]any{
		"type":    "message.deleted",
		"payload": map[string]any{},
		"ts":      "789",
	})
	h.onNATSMessage("message.deleted", msg)
	// Should enqueue with empty IDs — that's fine for fire-and-forget events.
	select {
	case <-h.broadcast:
		// ok
	default:
		t.Fatal("expected broadcast enqueued even for empty payload")
	}
}

// ── fanOut: both channelID and userID set ─────────────────────────────────────

func TestHub_FanOutByChannelTakesPrecedence(t *testing.T) {
	// When channelID is set, fanOut routes by channel even if userID is also set.
	h := newTestHub()
	c1 := makeTestClient("c1", "user-1", "ws-1", []string{"ch-prio"})
	c2 := makeTestClient("c2", "user-2", "ws-1", []string{"ch-prio"})
	h.addClient(c1)
	h.addClient(c2)

	payload := []byte(`{"type":"test"}`)
	h.fanOut(&broadcastMsg{channelID: "ch-prio", userID: "user-1", payload: payload})

	// Both should receive because they share the channel (userID branch skipped).
	select {
	case <-c1.send:
	default:
		t.Error("c1 should receive channel broadcast")
	}
	select {
	case <-c2.send:
	default:
		t.Error("c2 should receive channel broadcast")
	}
}

// ── UnsubscribeChannel for nonexistent channel ────────────────────────────────

func TestHub_UnsubscribeNonexistentChannel(t *testing.T) {
	h := newTestHub()
	c := makeTestClient("c1", "user-1", "ws-1", nil)
	h.addClient(c)
	// Should not panic when channel isn't in index.
	h.UnsubscribeChannel("c1", "ch-never-subscribed")
}

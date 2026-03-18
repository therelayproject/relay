// Package hub manages WebSocket connections and NATS-driven fan-out.
package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	nats "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

// Client represents a single WebSocket connection.
type Client struct {
	ID          string
	UserID      string
	WorkspaceID string
	ChannelIDs  map[string]bool
	send        chan []byte
	conn        *websocket.Conn
	hub         *Hub
	mu          sync.Mutex
}

// Hub manages all active WebSocket clients and NATS fan-out.
type Hub struct {
	clients    map[string]*Client // client ID -> client
	userIndex  map[string]map[string]*Client // userID -> clientID -> client
	channelIdx map[string]map[string]*Client // channelID -> clientID -> client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *broadcastMsg
	nc         *nats.Conn
	log        zerolog.Logger
	mu         sync.RWMutex
}

type broadcastMsg struct {
	channelID string
	userID    string // if set, deliver only to this user (presence etc.)
	payload   []byte
}

// New creates and returns a Hub.
func New(nc *nats.Conn, log zerolog.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		userIndex:  make(map[string]map[string]*Client),
		channelIdx: make(map[string]map[string]*Client),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan *broadcastMsg, 4096),
		nc:         nc,
		log:        log,
	}
}

// Run starts the hub event loop.  Call in a goroutine.
func (h *Hub) Run(ctx context.Context) {
	// Subscribe to all Relay NATS subjects the gateway needs to fan out.
	subjects := []string{
		"message.created",
		"message.updated",
		"message.deleted",
		"reaction.added",
		"reaction.removed",
		"message.pinned",
		"presence.changed",
		"typing.started",
		"typing.stopped",
		"channel.updated",
		"channel.archived",
		"member.joined",
		"member.left",
	}

	subs := make([]*nats.Subscription, 0, len(subjects))
	for _, subj := range subjects {
		s := subj // capture
		sub, err := h.nc.Subscribe(s, func(msg *nats.Msg) {
			h.onNATSMessage(s, msg.Data)
		})
		if err != nil {
			h.log.Error().Err(err).Str("subject", s).Msg("nats subscribe failed")
			continue
		}
		subs = append(subs, sub)
	}

	defer func() {
		for _, sub := range subs {
			_ = sub.Unsubscribe()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.addClient(client)
		case client := <-h.unregister:
			h.removeClient(client)
		case msg := <-h.broadcast:
			h.fanOut(msg)
		}
	}
}

// Register adds a new client to the hub.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// SubscribeChannel adds channelID to a client's subscription set.
func (h *Hub) SubscribeChannel(clientID, channelID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[clientID]
	if !ok {
		return
	}
	c.mu.Lock()
	c.ChannelIDs[channelID] = true
	c.mu.Unlock()
	if h.channelIdx[channelID] == nil {
		h.channelIdx[channelID] = make(map[string]*Client)
	}
	h.channelIdx[channelID][clientID] = c
}

// UnsubscribeChannel removes channelID from a client's subscription set.
func (h *Hub) UnsubscribeChannel(clientID, channelID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[clientID]
	if !ok {
		return
	}
	c.mu.Lock()
	delete(c.ChannelIDs, channelID)
	c.mu.Unlock()
	if m, ok := h.channelIdx[channelID]; ok {
		delete(m, clientID)
	}
}

// BroadcastTyping publishes a typing event to NATS (which fans out via hub).
func (h *Hub) BroadcastTyping(channelID, userID string, started bool) {
	subj := "typing.stopped"
	if started {
		subj = "typing.started"
	}
	payload, _ := json.Marshal(map[string]any{
		"type": subj,
		"payload": map[string]any{
			"channel_id": channelID,
			"user_id":    userID,
		},
		"ts": fmt.Sprintf("%d", time.Now().UnixMilli()),
	})
	_ = h.nc.Publish(subj, payload)
}

// ── internal ──────────────────────────────────────────────────────────────────

func (h *Hub) addClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.ID] = c
	if h.userIndex[c.UserID] == nil {
		h.userIndex[c.UserID] = make(map[string]*Client)
	}
	h.userIndex[c.UserID][c.ID] = c
	for ch := range c.ChannelIDs {
		if h.channelIdx[ch] == nil {
			h.channelIdx[ch] = make(map[string]*Client)
		}
		h.channelIdx[ch][c.ID] = c
	}
	h.log.Debug().Str("client", c.ID).Str("user", c.UserID).Msg("client registered")
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c.ID]; !ok {
		return
	}
	delete(h.clients, c.ID)
	if m, ok := h.userIndex[c.UserID]; ok {
		delete(m, c.ID)
		if len(m) == 0 {
			delete(h.userIndex, c.UserID)
		}
	}
	for ch := range c.ChannelIDs {
		if m, ok := h.channelIdx[ch]; ok {
			delete(m, c.ID)
			if len(m) == 0 {
				delete(h.channelIdx, ch)
			}
		}
	}
	close(c.send)
	h.log.Debug().Str("client", c.ID).Msg("client unregistered")
}

// onNATSMessage receives events from NATS, extracts channel_id, and enqueues a broadcast.
func (h *Hub) onNATSMessage(subject string, data []byte) {
	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
		Ts      string          `json:"ts"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return
	}

	channelID, _ := payload["channel_id"].(string)
	userID, _ := payload["user_id"].(string)

	// Build the outbound WS event.
	event, _ := json.Marshal(map[string]any{
		"type":    subject,
		"payload": payload,
		"ts":      time.Now().UnixMilli(),
	})

	h.broadcast <- &broadcastMsg{
		channelID: channelID,
		userID:    userID,
		payload:   event,
	}
}

func (h *Hub) fanOut(msg *broadcastMsg) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Presence events are broadcast to all clients in the user's workspace.
	// For simplicity we broadcast to all clients who share a channel with the user.
	// A production system would use workspace-level subscriptions.

	sent := make(map[string]bool)

	if msg.channelID != "" {
		if clients, ok := h.channelIdx[msg.channelID]; ok {
			for id, c := range clients {
				if sent[id] {
					continue
				}
				select {
				case c.send <- msg.payload:
					sent[id] = true
				default:
					// Slow client — drop.
				}
			}
		}
	}

	// Presence/typing events addressed to a specific user's connections.
	if msg.userID != "" && msg.channelID == "" {
		if clients, ok := h.userIndex[msg.userID]; ok {
			for id, c := range clients {
				if sent[id] {
					continue
				}
				select {
				case c.send <- msg.payload:
					sent[id] = true
				default:
				}
			}
		}
	}
}

// NewClient creates a Client wired to the hub.
func NewClient(id, userID, workspaceID string, channelIDs []string, conn *websocket.Conn, hub *Hub) *Client {
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
		conn:        conn,
		hub:         hub,
	}
}

// WritePump drains c.send into the WebSocket.  Call in a goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ReadPump reads client messages and dispatches them.  Call in a goroutine.
func (c *Client) ReadPump() {
	defer c.hub.Unregister(c)

	c.conn.SetReadLimit(32768)
	c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg struct {
			Type        string `json:"type"`
			ChannelID   string `json:"channel_id"`
			WorkspaceID string `json:"workspace_id"`
			LastMsgID   string `json:"last_message_id"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			if msg.ChannelID != "" {
				c.hub.SubscribeChannel(c.ID, msg.ChannelID)
			}
		case "unsubscribe":
			if msg.ChannelID != "" {
				c.hub.UnsubscribeChannel(c.ID, msg.ChannelID)
			}
		case "typing.start":
			c.hub.BroadcastTyping(msg.ChannelID, c.UserID, true)
		case "typing.stop":
			c.hub.BroadcastTyping(msg.ChannelID, c.UserID, false)
		case "presence.heartbeat":
			// Handled by presence-service; gateway just resets read deadline.
			c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		}
	}
}

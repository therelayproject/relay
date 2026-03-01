package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// EventType identifies the kind of real-time event.
type EventType string

const (
	EventTypeMessage         EventType = "message_created"
	EventTypeMessageUpdated  EventType = "message_updated"
	EventTypeMessageDeleted  EventType = "message_deleted"
	EventTypeReactionAdded   EventType = "reaction_added"
	EventTypePresenceUpdated EventType = "presence_updated"
	EventTypeTypingStart     EventType = "typing_start"
)

// Event is a real-time event received over the WebSocket connection.
type Event struct {
	Type    EventType       `json:"type"`
	Message *Message        `json:"message,omitempty"`
	Raw     json.RawMessage `json:"raw,omitempty"`
}

// Handler is a callback invoked when an event of a registered type is received.
type Handler func(Event)

// EventsClient manages the WebSocket event stream.
type EventsClient struct {
	client   *Client
	conn     *websocket.Conn
	handlers map[EventType][]Handler
	cancel   context.CancelFunc
}

// On registers a handler for the given event type.
// Call Connect() after registering all handlers.
func (e *EventsClient) On(t EventType, h Handler) {
	if e.handlers == nil {
		e.handlers = make(map[EventType][]Handler)
	}
	e.handlers[t] = append(e.handlers[t], h)
}

// Connect opens the WebSocket event stream.
// It blocks until the context is cancelled or the connection is closed.
func (e *EventsClient) Connect(ctx context.Context) error {
	wsURL := strings.Replace(e.client.baseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL += "/api/v1/ws"

	header := http.Header{}
	header.Set("Authorization", "Bearer "+e.client.token)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return err
	}
	e.conn = conn

	ctx, e.cancel = context.WithCancel(ctx)
	go e.readLoop(ctx)
	return nil
}

func (e *EventsClient) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, msg, err := e.conn.ReadMessage()
		if err != nil {
			return
		}

		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			continue
		}

		for _, h := range e.handlers[event.Type] {
			h(event)
		}
	}
}

func (e *EventsClient) close() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.conn != nil {
		e.conn.Close()
	}
}

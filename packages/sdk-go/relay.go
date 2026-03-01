// Package relay is the official Go SDK for the Relay API.
// It is intended for bot developers and integration authors.
//
// # Quick start
//
//	client, err := relay.NewClient("https://chat.example.com",
//	    relay.WithToken("your-bot-token"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Send a message
//	msg, err := client.Messages.Send(ctx, relay.SendMessageInput{
//	    ChannelID: "1234567890",
//	    Content:   "Hello from a bot!",
//	})
//
//	// Stream events
//	client.Events.On(relay.EventTypeMessage, func(e relay.Event) {
//	    fmt.Println(e.Message.Content)
//	})
package relay

import (
	"fmt"
	"net/http"
	"time"
)

// Client is the top-level SDK entry point.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client

	// Messages provides message send/edit/delete operations.
	Messages *MessagesClient
	// Events provides real-time event streaming over WebSocket.
	Events *EventsClient
}

// Option is a functional option for configuring a Client.
type Option func(*Client)

// WithToken sets the bot token used for authentication.
func WithToken(token string) Option {
	return func(c *Client) { c.token = token }
}

// WithHTTPClient replaces the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient creates a new Relay API client.
// baseURL is the root URL of the Relay server, e.g. "https://chat.example.com".
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("relay: baseURL is required")
	}
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	c.Messages = &MessagesClient{client: c}
	c.Events = &EventsClient{client: c}
	return c, nil
}

// Close shuts down any open connections (WebSocket event stream).
func (c *Client) Close() {
	if c.Events != nil {
		c.Events.close()
	}
}

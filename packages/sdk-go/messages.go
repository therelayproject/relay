package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// MessagesClient provides message operations.
type MessagesClient struct{ client *Client }

// Message is a Relay message.
type Message struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	AuthorID  string `json:"author_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// SendMessageInput is the input for sending a message.
type SendMessageInput struct {
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	// Blocks is an optional Block Kit payload. If set, Content is used as fallback text.
	Blocks json.RawMessage `json:"blocks,omitempty"`
}

// Send sends a message to a channel.
func (m *MessagesClient) Send(ctx context.Context, input SendMessageInput) (*Message, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.client.baseURL+"/api/v1/channels/"+input.ChannelID+"/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.client.token)

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("relay: send message: status %d", resp.StatusCode)
	}

	var msg Message
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

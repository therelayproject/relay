// Package authclient provides an internal HTTP client for the Auth Service.
// Use this when a service needs to call Auth synchronously — for example,
// to revoke a token or look up a user by token during a WebSocket handshake
// (where you can't use the HTTP middleware because the upgrade happens before
// the handler chain runs).
//
// For normal REST handlers, prefer the middleware.RequireAuth middleware instead
// of calling Auth directly — it validates the JWT locally without a network hop.
package authclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Client is a thin HTTP client for the Auth Service internal API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates an AuthClient using the AUTH_SERVICE_URL environment variable.
// Defaults to http://auth:8080 (the docker-compose service name).
func New() *Client {
	url := os.Getenv("AUTH_SERVICE_URL")
	if url == "" {
		url = "http://auth:8080"
	}
	return &Client{
		baseURL: url,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

type validateResponse struct {
	Valid        bool  `json:"valid"`
	UserID       int64 `json:"user_id"`
	WorkspaceID  int64 `json:"workspace_id"`
}

// ValidateToken calls Auth Service to check a raw JWT string.
// Returns (userID, workspaceID, nil) on success.
// Use this only for WebSocket upgrade paths; HTTP handlers should use
// the middleware.RequireAuth middleware instead.
func (c *Client) ValidateToken(ctx context.Context, token string) (userID, workspaceID int64, err error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/validate", nil)
	req.Header.Set("X-Token", token)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("auth validate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("auth validate: status %d", resp.StatusCode)
	}

	var body validateResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, 0, fmt.Errorf("auth validate decode: %w", err)
	}
	if !body.Valid {
		return 0, 0, fmt.Errorf("auth validate: token invalid")
	}
	return body.UserID, body.WorkspaceID, nil
}

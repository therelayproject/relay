// Package handler contains the WebSocket upgrade handler for ws-gateway.
package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/relay-im/relay/services/ws-gateway/internal/hub"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Origin validation should use an allowlist in production.
		return true
	},
}

// GatewayHandler handles WebSocket upgrades.
type GatewayHandler struct {
	hub       *hub.Hub
	jwtSecret []byte
	log       zerolog.Logger
}

// NewGatewayHandler creates a GatewayHandler.
func NewGatewayHandler(h *hub.Hub, jwtSecret string, log zerolog.Logger) *GatewayHandler {
	return &GatewayHandler{hub: h, jwtSecret: []byte(jwtSecret), log: log}
}

// Connect handles GET /gateway/connect?token={ws_token}
func (h *GatewayHandler) Connect(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		// Also accept Bearer header for flexibility.
		raw := r.Header.Get("Authorization")
		if strings.HasPrefix(raw, "Bearer ") {
			token = strings.TrimPrefix(raw, "Bearer ")
		}
	}
	if token == "" {
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}

	userID, workspaceID, channelIDs, err := h.validateToken(token)
	if err != nil {
		h.log.Debug().Err(err).Msg("ws token validation failed")
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}

	clientID := uuid.New().String()
	client := hub.NewClient(clientID, userID, workspaceID, channelIDs, conn, h.hub)
	h.hub.Register(client)

	// Send a connected event.
	hello, _ := json.Marshal(map[string]any{
		"type": "connected",
		"payload": map[string]any{
			"client_id":    clientID,
			"user_id":      userID,
			"workspace_id": workspaceID,
		},
	})
	conn.WriteMessage(websocket.TextMessage, hello) //nolint:errcheck

	go client.WritePump()
	client.ReadPump() // blocks until disconnect
}

// wsClaims is the JWT payload for ws_token.
type wsClaims struct {
	UserID      string   `json:"sub"`
	WorkspaceID string   `json:"wid"`
	ChannelIDs  []string `json:"cids"`
	jwt.RegisteredClaims
}

func (h *GatewayHandler) validateToken(tokenStr string) (userID, workspaceID string, channelIDs []string, err error) {
	claims := &wsClaims{}
	_, err = jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return h.jwtSecret, nil
	})
	if err != nil {
		return "", "", nil, err
	}
	return claims.UserID, claims.WorkspaceID, claims.ChannelIDs, nil
}

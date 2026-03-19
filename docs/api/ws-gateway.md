# WebSocket Gateway API

**Port:** 8006 (HTTP / WebSocket)

The WebSocket gateway provides real-time message delivery to connected clients. Clients establish a long-lived WebSocket connection authenticated with a JWT. The gateway distributes messages from NATS to subscribed clients.

## Authentication

WebSocket connections require a short-lived JWT (referred to as `ws_token`) containing workspace and channel membership claims.

The token may be provided in two ways:
1. **Query parameter:** `GET /gateway/connect?token=<ws_token>`
2. **HTTP header:** `Authorization: Bearer <ws_token>` on the upgrade request

JWT claims validated at connection time:
- `sub` — user ID
- `wid` — workspace ID
- `cids` — list of channel IDs the client is subscribed to

## HTTP Endpoints

### `GET /gateway/connect`

Upgrade an HTTP connection to a WebSocket connection.

- **Auth:** JWT required (via `token` query param or `Authorization` header)
- **Protocol:** HTTP → WebSocket upgrade
- **Query params:** `token` (string, optional if using Bearer header)
- **On success:** WebSocket connection established; server sends a `connected` message:
  ```json
  {
    "type": "connected",
    "payload": {
      "client_id": "string",
      "user_id": "string",
      "workspace_id": "string"
    }
  }
  ```

### `GET /healthz`

Health check endpoint.

- **Auth:** None
- **Response `200`:**
  ```json
  { "status": "ok" }
  ```

## WebSocket Protocol

### Connection Lifecycle

1. Client connects to `/gateway/connect` with a valid JWT.
2. Server validates the token, assigns a `client_id`, and sends a `connected` event.
3. The connection remains open; the server pushes events as they occur.
4. Clients may close the connection at any time; the server cleans up the client registration.

### Server-Sent Message Types

All messages are JSON-encoded objects with a `type` field.

#### `connected`

Sent immediately after a successful connection upgrade.

```json
{
  "type": "connected",
  "payload": {
    "client_id": "string",
    "user_id": "string",
    "workspace_id": "string"
  }
}
```

#### `message.created`

Broadcast when a new message is sent in a subscribed channel.

```json
{
  "type": "message.created",
  "payload": {
    "id": "string",
    "channel_id": "string",
    "author_id": "string",
    "body": "string",
    "thread_id": "string",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

#### `message.updated`

Broadcast when a message is edited.

```json
{
  "type": "message.updated",
  "payload": {
    "id": "string",
    "channel_id": "string",
    "body": "string",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

#### `message.deleted`

Broadcast when a message is deleted.

```json
{
  "type": "message.deleted",
  "payload": {
    "id": "string",
    "channel_id": "string"
  }
}
```

#### `presence.updated`

Broadcast when a user's presence status changes in the workspace.

```json
{
  "type": "presence.updated",
  "payload": {
    "user_id": "string",
    "status": "online",
    "last_seen": "2024-01-01T00:00:00Z"
  }
}
```

### Client-Sent Messages

The gateway is primarily push-only. Clients may send a ping to check liveness:

```json
{ "type": "ping" }
```

Server responds:

```json
{ "type": "pong" }
```

## NATS Integration

The gateway subscribes to NATS subjects and fans out events to connected clients:

| NATS Subject | WebSocket Event |
|-------------|-----------------|
| `message.created.<workspace_id>` | `message.created` |
| `message.updated.<workspace_id>` | `message.updated` |
| `message.deleted.<workspace_id>` | `message.deleted` |
| `presence.updated.<workspace_id>` | `presence.updated` |

A NATS hub manages multi-server distribution so clients connected to different gateway instances all receive events.

## Connection Limits & Deadlines

- Per-message write deadlines are enforced to detect stale connections.
- Clients that fail to respond to pings are disconnected.
- Connections are multiplexed across all channels declared in the JWT `cids` claim.

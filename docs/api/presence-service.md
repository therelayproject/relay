# Presence Service API

**Port:** 8007 (HTTP)

The presence service tracks online/offline status, custom user statuses, and workspace-level presence. Presence state is stored in Redis; updates are broadcast via NATS.

## Authentication

Most endpoints require a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)`. `GET /api/v1/users/{user_id}/status` is publicly accessible.

## REST Endpoints

### Heartbeat

#### `POST /api/v1/presence/heartbeat`

Mark the authenticated user as online within a workspace. Clients should send this every 30–60 seconds to maintain presence.

- **Auth:** JWT required
- **Request body:**
  ```json
  { "workspace_id": "string" }
  ```
- **Response `200`:**
  ```json
  { "ok": true }
  ```

### Workspace Presence

#### `GET /api/v1/workspaces/{workspace_id}/presence`

Get current online/offline status for all members of a workspace.

- **Auth:** JWT required
- **Path params:** `workspace_id`
- **Response `200`:**
  ```json
  {
    "presence": [
      {
        "user_id": "string",
        "status": "online",
        "last_seen": "2024-01-01T00:00:00Z"
      }
    ]
  }
  ```
  `status` values: `online`, `away`, `dnd`, `offline`

### Custom Status

#### `PUT /api/v1/users/me/status`

Set a custom status with an emoji and text, optionally expiring at a given time.

- **Auth:** JWT required
- **Request body:**
  ```json
  {
    "emoji": "🌴",
    "text": "On vacation",
    "expires_at": "2024-01-07T00:00:00Z"
  }
  ```
  `expires_at` is optional. Omit to set the status indefinitely.
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `DELETE /api/v1/users/me/status`

Clear the authenticated user's custom status.

- **Auth:** JWT required
- **Response:** `204 No Content`

#### `GET /api/v1/users/{user_id}/status`

Get a user's current custom status.

- **Auth:** None (public)
- **Path params:** `user_id`
- **Response `200`:**
  ```json
  {
    "status": {
      "emoji": "🌴",
      "text": "On vacation",
      "expires_at": "2024-01-07T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  }
  ```
  Returns `{ "status": null }` when no custom status is set.

## gRPC Service

**Proto:** `proto/presence/v1/presence.proto`

```protobuf
service PresenceService {
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc GetPresence(GetPresenceRequest) returns (GetPresenceResponse);
  rpc GetBulkPresence(GetBulkPresenceRequest) returns (GetBulkPresenceResponse);
}
```

### Messages

```protobuf
enum PresenceStatus {
  PRESENCE_STATUS_UNSPECIFIED = 0;
  PRESENCE_STATUS_ONLINE = 1;
  PRESENCE_STATUS_AWAY = 2;
  PRESENCE_STATUS_DND = 3;
  PRESENCE_STATUS_OFFLINE = 4;
}

message HeartbeatRequest {
  string user_id = 1;
  string session_id = 2;
}

message HeartbeatResponse {}

message GetPresenceRequest {
  string user_id = 1;
}

message GetPresenceResponse {
  Presence presence = 1;
}

message Presence {
  string user_id = 1;
  PresenceStatus status = 2;
  google.protobuf.Timestamp last_seen = 3;
}

message GetBulkPresenceRequest {
  repeated string user_ids = 1;
}

message GetBulkPresenceResponse {
  repeated Presence presences = 1;
}
```

## Infrastructure

- **Redis** — presence state storage with TTL-based expiry
- **NATS** — presence change events broadcast to subscribers (e.g., ws-gateway)

## Error Responses

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `NOT_FOUND` (404), `INVALID_ARGUMENT` (400), `INTERNAL` (500).

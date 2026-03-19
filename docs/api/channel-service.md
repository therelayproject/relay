# Channel Service API

**Port:** 8004 (HTTP)

The channel service manages channels within workspaces, including membership, archival, and browsing. Events are published via NATS on channel lifecycle changes.

## Authentication

All endpoints require a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)`. The authenticated user must be a workspace member to access channels within that workspace.

## REST Endpoints

### Channels

#### `POST /api/v1/workspaces/{workspaceId}/channels`

Create a new channel in a workspace.

- **Auth:** JWT required
- **Path params:** `workspaceId`
- **Request body:**
  ```json
  {
    "name": "string",
    "description": "string",
    "type": "public"
  }
  ```
  `type` values: `public`, `private`
- **Response `201`:**
  ```json
  {
    "id": "string",
    "workspace_id": "string",
    "name": "string",
    "slug": "string",
    "description": "string",
    "topic": "string",
    "type": "public",
    "is_archived": false,
    "created_by": "string",
    "member_count": 1,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
  ```

#### `GET /api/v1/workspaces/{workspaceId}/channels`

List and browse all channels in a workspace.

- **Auth:** JWT required
- **Path params:** `workspaceId`
- **Response `200`:**
  ```json
  {
    "channels": [{ "...": "same shape as POST response" }]
  }
  ```

#### `GET /api/v1/workspaces/{workspaceId}/channels/{id}`

Get a single channel by ID.

- **Auth:** JWT required
- **Path params:** `workspaceId`, `id` — channel ID
- **Response `200`:**
  ```json
  {
    "id": "string",
    "workspace_id": "string",
    "name": "string",
    "slug": "string",
    "description": "string",
    "topic": "string",
    "type": "public",
    "is_archived": false,
    "created_by": "string",
    "member_count": 1,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
  ```

#### `PATCH /api/v1/workspaces/{workspaceId}/channels/{id}`

Update channel metadata. Only channel owner/admin may call this.

- **Auth:** JWT required
- **Path params:** `workspaceId`, `id`
- **Request body:**
  ```json
  {
    "name": "string",
    "description": "string",
    "topic": "string"
  }
  ```
- **Response `200`:**
  ```json
  { "...": "same shape as GET response" }
  ```

#### `DELETE /api/v1/workspaces/{workspaceId}/channels/{id}`

Archive a channel (soft delete). Only channel owner/admin may call this.

- **Auth:** JWT required
- **Path params:** `workspaceId`, `id`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

### Channel Membership

#### `POST /api/v1/workspaces/{workspaceId}/channels/{id}/join`

Join a public channel.

- **Auth:** JWT required
- **Path params:** `workspaceId`, `id`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `POST /api/v1/workspaces/{workspaceId}/channels/{id}/leave`

Leave a channel.

- **Auth:** JWT required
- **Path params:** `workspaceId`, `id`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `GET /api/v1/workspaces/{workspaceId}/channels/{id}/members`

List all members of a channel.

- **Auth:** JWT required
- **Path params:** `workspaceId`, `id`
- **Response `200`:**
  ```json
  {
    "members": [
      {
        "channel_id": "string",
        "user_id": "string",
        "role": "member",
        "last_read_at": "2024-01-01T00:00:00Z",
        "joined_at": "2024-01-01T00:00:00Z"
      }
    ]
  }
  ```

#### `POST /api/v1/workspaces/{workspaceId}/channels/{id}/members`

Add a user to a channel (admin/owner action).

- **Auth:** JWT required
- **Path params:** `workspaceId`, `id`
- **Request body:**
  ```json
  { "user_id": "string" }
  ```
- **Response:** `201 Created`

#### `DELETE /api/v1/workspaces/{workspaceId}/channels/{id}/members/{userId}`

Remove a member from a channel.

- **Auth:** JWT required
- **Path params:** `workspaceId`, `id`, `userId`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

## NATS Events

| Event | Trigger |
|-------|---------|
| `channel.created` | New channel created |
| `channel.updated` | Channel metadata updated |
| `channel.archived` | Channel archived |
| `channel.member.joined` | User joined channel |
| `channel.member.left` | User left or was removed |

## Error Responses

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404), `CONFLICT` (409), `INVALID_ARGUMENT` (400), `INTERNAL` (500).

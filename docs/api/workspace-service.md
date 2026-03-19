# Workspace Service API

**Port:** 8003 (HTTP)

The workspace service manages workspaces, membership, roles, and invitations. Events are published via NATS on creation and updates.

## Authentication

Most endpoints require a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)`. Role-based access: only workspace owners/admins can manage members.

## REST Endpoints

### Workspaces

#### `POST /api/v1/workspaces`

Create a new workspace. The authenticated user becomes the owner.

- **Auth:** JWT required
- **Request body:**
  ```json
  {
    "name": "string",
    "slug": "string",
    "description": "string"
  }
  ```
- **Response `201`:**
  ```json
  {
    "id": "string",
    "name": "string",
    "slug": "string",
    "description": "string",
    "icon_url": "string",
    "owner_id": "string",
    "allow_guest_invites": false,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
  ```

#### `GET /api/v1/workspaces`

List workspaces the authenticated user is a member of.

- **Auth:** JWT required
- **Response `200`:**
  ```json
  [{ "...": "same shape as POST response" }]
  ```

#### `GET /api/v1/workspaces/{id}`

Get a single workspace by ID.

- **Auth:** None (public)
- **Path params:** `id` — workspace ID
- **Response `200`:**
  ```json
  { "...": "same shape as POST response" }
  ```

#### `PATCH /api/v1/workspaces/{id}`

Update workspace settings. Only owner/admin may call this.

- **Auth:** JWT required (owner/admin)
- **Path params:** `id` — workspace ID
- **Request body:**
  ```json
  {
    "name": "string",
    "description": "string",
    "icon_url": "string"
  }
  ```
- **Response `200`:**
  ```json
  { "...": "same shape as POST response" }
  ```

### Membership

#### `POST /api/v1/workspaces/join`

Join a workspace using an invitation token.

- **Auth:** JWT required
- **Request body:**
  ```json
  { "token": "string" }
  ```
- **Response `200`:**
  ```json
  {
    "workspace_id": "string",
    "user_id": "string",
    "role": "member",
    "invited_by": "string",
    "joined_at": "2024-01-01T00:00:00Z"
  }
  ```

#### `GET /api/v1/workspaces/{id}/members`

List all members of a workspace.

- **Auth:** JWT required
- **Path params:** `id` — workspace ID
- **Response `200`:**
  ```json
  [
    {
      "workspace_id": "string",
      "user_id": "string",
      "role": "member",
      "invited_by": "string",
      "joined_at": "2024-01-01T00:00:00Z"
    }
  ]
  ```

#### `PATCH /api/v1/workspaces/{id}/members/{userId}`

Update a member's role. Only owner/admin may call this.

- **Auth:** JWT required (owner/admin)
- **Path params:** `id` — workspace ID, `userId` — target user ID
- **Request body:**
  ```json
  { "role": "admin" }
  ```
- **Response:** `204 No Content`

#### `DELETE /api/v1/workspaces/{id}/members/{userId}`

Remove a member from the workspace. Only owner/admin may call this.

- **Auth:** JWT required (owner/admin)
- **Path params:** `id` — workspace ID, `userId` — target user ID
- **Response:** `204 No Content`

### Invitations

#### `POST /api/v1/workspaces/{id}/invitations`

Create an invitation link for a given email address. Only owner/admin may call this.

- **Auth:** JWT required (owner/admin)
- **Path params:** `id` — workspace ID
- **Request body:**
  ```json
  {
    "email": "string",
    "role": "member"
  }
  ```
- **Response `201`:**
  ```json
  { "token": "string" }
  ```

## gRPC Service

**Proto:** `proto/workspace/v1/workspace.proto`

```protobuf
service WorkspaceService {
  rpc CreateWorkspace(CreateWorkspaceRequest) returns (CreateWorkspaceResponse);
  rpc GetWorkspace(GetWorkspaceRequest) returns (GetWorkspaceResponse);
  rpc InviteMember(InviteMemberRequest) returns (InviteMemberResponse);
  rpc RemoveMember(RemoveMemberRequest) returns (RemoveMemberResponse);
}
```

### Messages

```protobuf
message CreateWorkspaceRequest {
  string name = 1;
  string slug = 2;
  string owner_id = 3;
}

message CreateWorkspaceResponse {
  Workspace workspace = 1;
}

message GetWorkspaceRequest {
  string workspace_id = 1;
}

message GetWorkspaceResponse {
  Workspace workspace = 1;
}

message InviteMemberRequest {
  string workspace_id = 1;
  string email = 2;
  string role = 3;
}

message InviteMemberResponse {
  string invite_token = 1;
}

message RemoveMemberRequest {
  string workspace_id = 1;
  string user_id = 2;
}

message RemoveMemberResponse {}
```

## NATS Events

| Event | Trigger |
|-------|---------|
| `workspace.created` | New workspace created |
| `workspace.updated` | Workspace settings updated |
| `workspace.member.joined` | Member joined via invitation |
| `workspace.member.removed` | Member removed |

## Error Responses

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404), `CONFLICT` (409), `INVALID_ARGUMENT` (400), `INTERNAL` (500).

# User Service API

**Port:** 8002 (HTTP) / 9002 (gRPC)

The user service manages user profiles, statuses, and avatars.

## Authentication

All endpoints require a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)`. Users may only modify their own profile (owner check enforced at the handler level).

Rate limit: 200 req/sec sustained, 400 burst.

## REST Endpoints

#### `GET /api/v1/users/{id}`

Retrieve a user profile by ID.

- **Auth:** JWT required
- **Path params:** `id` — user ID
- **Response `200`:**
  ```json
  {
    "user": {
      "user_id": "string",
      "display_name": "string",
      "avatar_url": "string",
      "status_text": "string",
      "status_emoji": "string",
      "timezone": "string",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  }
  ```

#### `GET /api/v1/users`

Retrieve multiple user profiles by ID list.

- **Auth:** JWT required
- **Query params:** `ids` — comma-separated user IDs (e.g. `ids=u1,u2,u3`)
- **Response `200`:**
  ```json
  {
    "users": [
      {
        "user_id": "string",
        "display_name": "string",
        "avatar_url": "string",
        "status_text": "string",
        "status_emoji": "string",
        "timezone": "string",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
      }
    ]
  }
  ```

#### `PATCH /api/v1/users/{id}`

Update a user's profile. Only the authenticated user can update their own profile.

- **Auth:** JWT required (owner only)
- **Path params:** `id` — user ID
- **Request body:**
  ```json
  {
    "display_name": "string",
    "avatar_url": "string",
    "timezone": "string"
  }
  ```
- **Response `200`:**
  ```json
  {
    "user": { "...": "same as GET response" }
  }
  ```

#### `PUT /api/v1/users/{id}/status`

Set a user's status text and emoji. Only the authenticated user can set their own status.

- **Auth:** JWT required (owner only)
- **Path params:** `id` — user ID
- **Request body:**
  ```json
  {
    "status_text": "string",
    "status_emoji": "string"
  }
  ```
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `POST /api/v1/users/{id}/avatar`

Upload a profile avatar image. Only the authenticated user can upload their own avatar.

- **Auth:** JWT required (owner only)
- **Path params:** `id` — user ID
- **Request body:** `multipart/form-data` — field name: `avatar`, max size: 8 MiB
- **Response `200`:**
  ```json
  { "avatar_url": "string" }
  ```

## gRPC Service

**Proto:** `proto/user/v1/user.proto`

```protobuf
service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc UpdateProfile(UpdateProfileRequest) returns (UpdateProfileResponse);
  rpc SetStatus(SetStatusRequest) returns (SetStatusResponse);
}
```

### Messages

#### `GetUserRequest` / `GetUserResponse`

```protobuf
message GetUserRequest {
  string user_id = 1;
}

message GetUserResponse {
  User user = 1;
}

message User {
  string id = 1;
  string email = 2;
  string display_name = 3;
  string avatar_url = 4;
  string status_text = 5;
  string status_emoji = 6;
  google.protobuf.Timestamp created_at = 7;
}
```

#### `UpdateProfileRequest` / `UpdateProfileResponse`

```protobuf
message UpdateProfileRequest {
  string user_id = 1;
  string display_name = 2;
  string avatar_url = 3;
}

message UpdateProfileResponse {
  User user = 1;
}
```

#### `SetStatusRequest` / `SetStatusResponse`

```protobuf
message SetStatusRequest {
  string user_id = 1;
  string status_text = 2;
  string status_emoji = 3;
}

message SetStatusResponse {}
```

## Error Responses

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404), `INVALID_ARGUMENT` (400), `INTERNAL` (500).

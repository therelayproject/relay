# Message Service API

**Port:** 8005 (HTTP)

The message service handles sending, editing, and deleting messages; threaded replies; emoji reactions; and pinning. Read endpoints are public; write endpoints require authentication.

## Authentication

Write operations require a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)` applied selectively — read endpoints (list, thread, reactions, pins) are publicly accessible. Rate limit: 200 req/sec sustained, 400 burst.

## REST Endpoints

### Messages

#### `POST /api/v1/channels/{channel_id}/messages`

Send a message to a channel (or reply in a thread).

- **Auth:** JWT required
- **Path params:** `channel_id`
- **Request body:**
  ```json
  {
    "body": "string",
    "thread_id": "string",
    "parent_id": "string",
    "idempotency_key": "string"
  }
  ```
  `thread_id` and `parent_id` are optional; omit both for a top-level message. `idempotency_key` prevents duplicate sends.
- **Response `201`:**
  ```json
  {
    "id": "string",
    "channel_id": "string",
    "author_id": "string",
    "body": "string",
    "thread_id": "string",
    "parent_id": "string",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
  ```

#### `GET /api/v1/channels/{channel_id}/messages`

List messages in a channel, newest first with cursor-based pagination.

- **Auth:** None (public)
- **Path params:** `channel_id`
- **Query params:**
  - `limit` (integer, default 50)
  - `cursor` (opaque string from previous response)
- **Response `200`:**
  ```json
  {
    "data": [{ "...": "same shape as POST response" }],
    "cursor": "string",
    "has_more": true
  }
  ```

#### `GET /api/v1/channels/{channel_id}/messages/{message_id}/thread`

List replies in a message thread.

- **Auth:** None (public)
- **Path params:** `channel_id`, `message_id` — root message ID
- **Query params:**
  - `limit` (integer, default 50)
  - `cursor` (string)
- **Response `200`:**
  ```json
  {
    "data": [{ "...": "same shape as message" }],
    "cursor": "string",
    "has_more": true
  }
  ```

#### `PATCH /api/v1/channels/{channel_id}/messages/{message_id}`

Edit the body of a message. Only the message author may edit.

- **Auth:** JWT required (author only)
- **Path params:** `channel_id`, `message_id`
- **Request body:**
  ```json
  { "body": "string" }
  ```
- **Response `200`:**
  ```json
  { "...": "same shape as message" }
  ```

#### `DELETE /api/v1/channels/{channel_id}/messages/{message_id}`

Delete a message. Only the message author or a channel admin may delete.

- **Auth:** JWT required
- **Path params:** `channel_id`, `message_id`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

### Reactions

#### `POST /api/v1/channels/{channel_id}/messages/{message_id}/reactions`

Add an emoji reaction to a message.

- **Auth:** JWT required
- **Path params:** `channel_id`, `message_id`
- **Request body:**
  ```json
  { "emoji": "👍" }
  ```
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `DELETE /api/v1/channels/{channel_id}/messages/{message_id}/reactions/{emoji}`

Remove the authenticated user's reaction from a message.

- **Auth:** JWT required
- **Path params:** `channel_id`, `message_id`, `emoji` — URL-encoded emoji
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `GET /api/v1/channels/{channel_id}/messages/{message_id}/reactions`

List all reactions on a message.

- **Auth:** None (public)
- **Path params:** `channel_id`, `message_id`
- **Response `200`:**
  ```json
  {
    "reactions": [
      {
        "emoji": "👍",
        "count": 3,
        "user_ids": ["string"]
      }
    ]
  }
  ```

### Pins

#### `POST /api/v1/channels/{channel_id}/pins/{message_id}`

Pin a message in a channel.

- **Auth:** JWT required
- **Path params:** `channel_id`, `message_id`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `DELETE /api/v1/channels/{channel_id}/pins/{message_id}`

Unpin a message.

- **Auth:** None
- **Path params:** `channel_id`, `message_id`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `GET /api/v1/channels/{channel_id}/pins`

List all pinned messages in a channel.

- **Auth:** None (public)
- **Path params:** `channel_id`
- **Response `200`:**
  ```json
  {
    "data": [
      {
        "id": "string",
        "channel_id": "string",
        "message_id": "string",
        "pinned_by": "string",
        "created_at": "2024-01-01T00:00:00Z"
      }
    ]
  }
  ```

## gRPC Service

**Proto:** `proto/message/v1/message.proto`

```protobuf
service MessageService {
  rpc SendMessage(SendMessageRequest) returns (SendMessageResponse);
  rpc GetMessage(GetMessageRequest) returns (GetMessageResponse);
  rpc DeleteMessage(DeleteMessageRequest) returns (DeleteMessageResponse);
  rpc AddReaction(AddReactionRequest) returns (AddReactionResponse);
}
```

### Messages

```protobuf
message SendMessageRequest {
  string channel_id = 1;
  string sender_id = 2;
  string body = 3;
  string thread_id = 4;
}

message SendMessageResponse {
  Message message = 1;
}

message Message {
  string id = 1;
  string channel_id = 2;
  string sender_id = 3;
  string body = 4;
  string thread_id = 5;
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp updated_at = 7;
  bool deleted = 8;
}

message AddReactionRequest {
  string message_id = 1;
  string user_id = 2;
  string emoji = 3;
}

message AddReactionResponse {}
```

## Error Responses

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404), `CONFLICT` (409 — duplicate idempotency key), `INVALID_ARGUMENT` (400), `INTERNAL` (500).

# Notification Service API

**Port:** 8008 (HTTP)

The notification service manages user notification preferences, push tokens, and do-not-disturb scheduling. It consumes NATS events (e.g. mention notifications) and dispatches push notifications.

## Authentication

All endpoints require a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)`.

## REST Endpoints

### Notification Preferences

#### `GET /api/v1/users/me/notification-preferences`

Get the authenticated user's notification preferences.

- **Auth:** JWT required
- **Response `200`:**
  ```json
  {
    "preferences": [
      {
        "scope": "string",
        "level": "MENTIONS",
        "muted": false
      }
    ]
  }
  ```
  `level` values: `ALL`, `MENTIONS`, `NONE`

  `scope` identifies the entity (workspace ID, channel ID, or `"global"`).

#### `PUT /api/v1/users/me/notification-preferences/{scope}`

Create or update notification preferences for a specific scope.

- **Auth:** JWT required
- **Path params:** `scope` — workspace ID, channel ID, or `"global"`
- **Request body:**
  ```json
  {
    "level": "MENTIONS",
    "muted": false
  }
  ```
- **Response `200`:**
  ```json
  {
    "scope": "string",
    "level": "MENTIONS",
    "muted": false
  }
  ```

### Push Tokens

#### `POST /api/v1/users/me/push-tokens`

Register a mobile push notification token.

- **Auth:** JWT required
- **Request body:**
  ```json
  {
    "platform": "ios",
    "token": "string"
  }
  ```
  `platform` values: `ios`, `android`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `DELETE /api/v1/users/me/push-tokens/{platform}`

Remove a push token for a platform.

- **Auth:** JWT required
- **Path params:** `platform` — `ios` or `android`
- **Response `200`:**
  ```json
  { "ok": true }
  ```

### Do Not Disturb

#### `POST /api/v1/users/me/dnd`

Enable do-not-disturb mode until a specified time.

- **Auth:** JWT required
- **Request body:**
  ```json
  { "until": "2024-01-01T09:00:00Z" }
  ```
  `until` is an RFC3339 timestamp. Pass a past timestamp or omit to clear DND.
- **Response `200`:**
  ```json
  { "ok": true }
  ```

## NATS Consumers

The notification service subscribes to NATS events to deliver notifications:

| Subject | Action |
|---------|--------|
| `message.mention` | Send mention notification based on user preferences |
| `message.created` | Evaluate ALL-level subscriptions and dispatch |

## Error Responses

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `NOT_FOUND` (404), `INVALID_ARGUMENT` (400), `INTERNAL` (500).

# Relay API Documentation

This directory contains API documentation for all Relay backend services.

## Services

| Service | Port (HTTP) | Port (gRPC) | File |
|---------|-------------|-------------|------|
| [Auth Service](auth-service.md) | 8001 | 9001 | Registration, login, OAuth, MFA, sessions |
| [User Service](user-service.md) | 8002 | 9002 | User profiles, avatars, status |
| [Workspace Service](workspace-service.md) | 8003 | — | Workspaces, membership, invitations |
| [Channel Service](channel-service.md) | 8004 | — | Channels, channel membership |
| [Message Service](message-service.md) | 8005 | — | Messages, threads, reactions, pins |
| [WebSocket Gateway](ws-gateway.md) | 8006 | — | Real-time WebSocket event delivery |
| [Presence Service](presence-service.md) | 8007 | — | Online/offline status, custom status |
| [Notification Service](notification-service.md) | 8008 | — | Push tokens, preferences, DND |
| [Search Service](search-service.md) | 8009 | — | Full-text message search |
| [File Service](file-service.md) | 8010 | — | File upload and presigned download URLs |

## Cross-Cutting Concerns

### Authentication

All authenticated endpoints use JWT Bearer tokens:

```
Authorization: Bearer <access_token>
```

Tokens are issued by the auth service. The `access_token` is short-lived; use the refresh endpoint to rotate tokens without re-authenticating.

### Middleware Stack

All services apply the following middleware in order:

1. **RequestID** — injects a unique trace ID into every request
2. **Logger** — structured request/response logging
3. **RateLimit** — 200 req/sec sustained, 400 burst (on most services)
4. **Auth** — JWT validation (on protected routes)

### Error Format

All services return errors in this format:

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

| Code | HTTP Status |
|------|-------------|
| `INVALID_ARGUMENT` | 400 |
| `UNAUTHORIZED` | 401 |
| `FORBIDDEN` | 403 |
| `NOT_FOUND` | 404 |
| `CONFLICT` | 409 |
| `PAYLOAD_TOO_LARGE` | 413 |
| `INTERNAL` | 500 |

### Infrastructure Dependencies

| Dependency | Used By |
|-----------|---------|
| PostgreSQL | All persistent services |
| Redis | auth-service (sessions), user-service (cache), presence-service (state) |
| NATS | workspace, channel, message, notification, search, ws-gateway |
| Elasticsearch | search-service |
| MinIO (S3) | file-service |

### Proto Definitions

gRPC service definitions live in `proto/`:

```
proto/
  auth/v1/auth.proto
  user/v1/user.proto
  workspace/v1/workspace.proto
  message/v1/message.proto
  presence/v1/presence.proto
```

See `buf.yaml` and `buf.gen.yaml` for code generation configuration.

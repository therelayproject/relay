# Service Integration Guide

How the 9 Relay services are wired together in code.

## The two integration primitives

| Primitive | When to use | Package |
|-----------|------------|---------|
| **NATS event** | Fire-and-forget, fan-out, async | `packages/events` + `packages/serviceutil/natsconn` |
| **Internal HTTP** | Synchronous, need a response | `packages/serviceutil/authclient` (or plain `net/http`) |

PostgreSQL is accessed directly by whichever service owns that data — there is no shared DB client library, each service manages its own connection pool via `DATABASE_URL`.

---

## Step 1 — Add the shared packages to your service's go.mod

Because the repo uses a Go workspace (`go.work`), you don't need to publish the
packages to a registry. Just add the `require` directive:

```go
// services/messaging/go.mod
require (
    github.com/therelayproject/relay/packages/events    v0.0.0
    github.com/therelayproject/relay/packages/serviceutil v0.0.0
)
```

The workspace's `replace` directives resolve the module paths to local disk.

---

## Step 2 — Connect to NATS at startup

Every service that uses NATS does this in `cmd/main.go`:

```go
import "github.com/therelayproject/relay/packages/serviceutil/natsconn"

nc, err := natsconn.Connect() // reads NATS_URL env var
if err != nil {
    log.Fatal().Err(err).Msg("failed to connect to nats")
}
defer nc.Drain() // flush pending messages on shutdown
```

---

## Step 3 — Publishing an event (Messaging Service example)

```go
// services/messaging/internal/bus/bus.go
package bus

import (
    "encoding/json"
    "github.com/nats-io/nats.go"
    "github.com/therelayproject/relay/packages/events"
)

type Bus struct{ nc *nats.Conn }

func New(nc *nats.Conn) *Bus { return &Bus{nc: nc} }

func (b *Bus) MessageCreated(e events.MessageCreatedEvent) error {
    data, err := json.Marshal(e)
    if err != nil {
        return err
    }
    return b.nc.Publish(events.SubjectMessageCreated, data)
}
```

Called from the handler after the DB write succeeds:

```go
// services/messaging/internal/handler/message.go
func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
    // 1. parse body, validate
    // 2. write to PostgreSQL
    msg, err := h.db.InsertMessage(r.Context(), ...)

    // 3. publish — if NATS is down the message is still saved; event is best-effort
    _ = h.bus.MessageCreated(events.MessageCreatedEvent{
        MessageID:   msg.ID,
        WorkspaceID: msg.WorkspaceID,
        ChannelID:   msg.ChannelID,
        AuthorID:    msg.AuthorID,
        Content:     msg.Content,
    })

    // 4. respond to client
    json.NewEncoder(w).Encode(msg)
}
```

---

## Step 4 — Subscribing to an event (Notification Service example)

```go
// services/notification/internal/subscriber/subscriber.go
package subscriber

import (
    "encoding/json"
    "github.com/nats-io/nats.go"
    "github.com/therelayproject/relay/packages/events"
)

type Subscriber struct {
    nc      *nats.Conn
    pusher  *push.Pusher // your push notification logic
}

func (s *Subscriber) Start() error {
    _, err := s.nc.Subscribe(events.SubjectMessageCreated, s.onMessageCreated)
    return err
}

func (s *Subscriber) onMessageCreated(msg *nats.Msg) {
    var e events.MessageCreatedEvent
    if err := json.Unmarshal(msg.Data, &e); err != nil {
        return // log and discard malformed events
    }

    // look up which users in this workspace have notifications enabled
    // and aren't currently active (presence check via Redis)
    recipients := s.db.GetNotifiableUsers(e.WorkspaceID, e.ChannelID)
    for _, userID := range recipients {
        s.pusher.Send(userID, e)
    }
}
```

Called once at startup, before the HTTP server starts:

```go
// services/notification/cmd/main.go
sub := subscriber.New(nc, db, pusher)
if err := sub.Start(); err != nil {
    log.Fatal().Err(err).Msg("subscribe failed")
}
```

---

## Step 5 — Protecting a handler with JWT auth

Any service that exposes routes the API Gateway proxies to needs auth middleware:

```go
import "github.com/therelayproject/relay/packages/serviceutil/middleware"

secret := []byte(os.Getenv("JWT_SECRET"))
auth   := middleware.RequireAuth(secret)

mux.Handle("POST /messages", auth(http.HandlerFunc(handler.CreateMessage)))
```

Reading the claims inside the handler:

```go
func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
    claims := middleware.ClaimsFrom(r.Context())
    // claims.UserID, claims.WorkspaceID, claims.Role are populated
}
```

---

## Step 6 — Internal service-to-service HTTP

For synchronous calls between services (not covered by NATS), use plain `net/http`
with the service's Docker Compose hostname:

```go
// services/messaging/internal/handler/ws.go
// WebSocket upgrade — can't use HTTP middleware here, so call Auth directly.
import "github.com/therelayproject/relay/packages/serviceutil/authclient"

ac := authclient.New() // reads AUTH_SERVICE_URL env var

userID, workspaceID, err := ac.ValidateToken(r.Context(), tokenFromQuery)
if err != nil {
    http.Error(w, "unauthorized", http.StatusUnauthorized)
    return
}
```

For other service-to-service calls (e.g. Media Service calling Auth), create a
similar thin client in `packages/serviceutil/<servicename>client/`.

---

## Environment variables each service needs

All services share a common set of env vars (see `.env.example`):

| Variable | Used by | Purpose |
|---|---|---|
| `DATABASE_URL` | all except Federation | PostgreSQL connection string |
| `REDIS_URL` | Auth, API, Messaging | Redis connection string |
| `NATS_URL` | Messaging, Notification, Search, Integration, Federation | NATS address |
| `JWT_SECRET` | Auth (sign), all others (verify) | HMAC secret |
| `AUTH_SERVICE_URL` | Messaging (WS upgrade) | Internal Auth address |
| `PORT` | all | HTTP listen port (default per service) |

Services only read the vars they need — they don't crash on unknown vars.

---

## Adding a new inter-service event

1. Add the event struct and subject constant to `packages/events/`.
2. Publish from the source service using `nc.Publish(events.SubjectFoo, data)`.
3. Subscribe in the consuming service using `nc.Subscribe(events.SubjectFoo, handler)`.
4. No other changes needed — NATS is a dumb pipe, there's no schema registry.

Keep event structs append-only (add fields, never remove or rename) so old
subscribers don't break when a publisher is updated.

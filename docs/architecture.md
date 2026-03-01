# OpenSlack — Open-Source Slack Alternative
## System Architecture Design

---

## 1. Goals & Principles

- **Self-hostable**: runs on a single VM or Kubernetes cluster
- **Federated**: cross-org invites, shared channels, and DMs across separately hosted servers via signed server-to-server API (see §13)
- **Horizontally scalable**: stateless services behind load balancers
- **Pluggable**: apps/bots/webhooks via a published integration API
- **Open standards**: OAuth 2.0, WebSocket, S3-compatible storage, SMTP/APNS/FCM

---

## 2. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          CLIENTS                                │
│         Flutter (single codebase → 6 platform targets)         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  Web     │  │ Desktop  │  │  Mobile  │  │  CLI / Bots   │  │
│  │(Flutter) │  │Win/Mac/  │  │ iOS /    │  │  / Webhooks   │  │
│  │          │  │Linux     │  │ Android  │  │               │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬────────┘  │
└───────┼─────────────┼─────────────┼────────────────┼───────────┘
        │             │             │                │
        ▼             ▼             ▼                ▼
┌───────────────────────────────────────────────────────────────┐
│                      API GATEWAY / EDGE                       │
│         (Nginx / Caddy + rate limiting + TLS termination)     │
│                                                               │
│   REST API  ──────────────────────────────────────────────    │
│   WebSocket Gateway (ws://)  ──────────────────────────────   │
│   gRPC (internal services)  ───────────────────────────────   │
└──────────────────────────────┬────────────────────────────────┘
                               │
        ┌──────────────────────┼──────────────────────┐
        ▼                      ▼                       ▼
┌──────────────┐   ┌───────────────────┐   ┌─────────────────┐
│  Auth Service│   │  Messaging Service │   │  API Service    │
│              │   │  (Realtime Core)   │   │  (REST/GraphQL) │
│  - Register  │   │                   │   │                 │
│  - Login     │   │  - WS connections │   │  - Workspaces   │
│  - OAuth 2.0 │   │  - Pub/Sub fan-out│   │  - Channels     │
│  - JWT/Tokens│   │  - Presence       │   │  - Users/Roles  │
│  - MFA/TOTP  │   │  - Typing indics. │   │  - Files        │
│  - SSO/SAML  │   │  - Block validate │   │  - Search       │
│  - Perm.matrix   │  - Action dispatch│   │  - Perm.middleware
└──────┬───────┘   └────────┬──────────┘   └────────┬────────┘
       │                    │                        │
       └──────────┬─────────┘                        │
                  │                                   │
        ┌─────────▼──────────────────────────────────▼──────────┐
        │                   MESSAGE BROKER                       │
        │              (NATS JetStream / Kafka)                  │
        │                                                        │
        │  Topics: messages, events, notifications, presence     │
        └────┬──────────────┬───────────────┬───────────────────┘
             │              │               │
     ┌───────▼──────┐ ┌─────▼──────┐ ┌────▼───────────┐
     │ Notification │ │  Search    │ │  Integration   │
     │   Service    │ │  Service   │ │  Service       │
     │              │ │            │ │                │
     │ - Push (FCM) │ │ - Indexing │ │ - Webhooks     │  ┌──────────────────┐
     │ - Push (APNS)│ │ - Full-text│ │ - Slash cmds   │  │  Media Service   │
     │ - Email/SMTP │ │ (Meilisrch)│ │ - OAuth apps   │  │                  │
     │ - Desktop WS │ │ /Typesense)│ │ - Action router│  │ - Room lifecycle │
     └──────────────┘ └────────────┘ │ - Block Kit val│  │ - Token issuing  │
                                      └────────────────┘  │ - Participant st.│
                                                           └────────┬─────────┘
                                                                    │
                                          ┌─────────────────────────▼──────────┐
                                          │         Livekit SFU Server          │
                                          │  (WebRTC media plane — audio/video) │
                                          ├────────────────────────────────────┤
                                          │         Coturn (TURN/STUN)          │
                                          │  (NAT traversal for self-hosters)  │
                                          └────────────────────────────────────┘

┌───────────────────────────────────────────────────────────────────────────────┐
│                        FEDERATION LAYER  (§13)                                │
│                                                                               │
│   ┌─────────────────────────────────────────────────────────────────────┐    │
│   │  Federation Service (Go)                                            │    │
│   │  - Server identity & Ed25519 keypair                                │    │
│   │  - .well-known/openslack discovery endpoint                         │    │
│   │  - Outbound S2S: signed HTTP POST to remote federation endpoints    │    │
│   │  - Inbound S2S: verify signatures, relay to NATS, fan-out locally   │    │
│   │  - Trust registry (allowed / blocked / pending servers)             │    │
│   │  - Domain policy enforcement (internal / external / blocked users)  │    │
│   └───────────────────────────┬─────────────────────────────────────────┘    │
│                               │  HTTPS + HTTP Signatures (Ed25519)           │
│                               ▼                                               │
│          ┌──────────────────────────────────────────┐                        │
│          │  Remote OpenSlack Server(s)              │                        │
│          │  (separately hosted, any version)        │                        │
│          └──────────────────────────────────────────┘                        │
└───────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Service Breakdown

### 3.1 Auth Service
| Concern        | Choice                              |
|----------------|-------------------------------------|
| Framework      | Go (Fiber) or Node.js (Fastify)     |
| Token format   | JWT (access 15 min) + Refresh token |
| Session store  | Redis (token allowlist)             |
| OAuth providers| GitHub, Google, GitLab (pluggable)  |
| MFA            | TOTP (RFC 6238)                     |
| SSO            | SAML 2.0 / OIDC                     |
| Password hash  | Argon2id                            |

### 3.2 Messaging Service (Realtime Core)
| Concern             | Choice                                         |
|---------------------|------------------------------------------------|
| Language            | Go (excellent concurrency for WS fan-out)      |
| WebSocket library   | gorilla/websocket or nhooyr.io/websocket       |
| Connection registry | Redis (per-pod WS map + cross-pod pub/sub)     |
| Message ordering    | Per-channel monotonic sequence (Snowflake ID)  |
| Fan-out strategy    | Pub/sub via NATS subjects per channel          |
| Presence            | Heartbeat + Redis TTL keys                     |
| Threads             | Nested message model (parent_id FK)            |

### 3.3 API Service (CRUD)
| Concern            | Choice                                                         |
|--------------------|----------------------------------------------------------------|
| Protocol           | REST + optional GraphQL (for clients)                         |
| Framework          | Go (Chi) or TypeScript (Hono/Fastify)                         |
| Validation         | JSON Schema / Zod                                             |
| Rate limiting      | Redis sliding window per user/IP                              |
| Permission middleware | `PermissionGuard` on every route — resolves caller role in workspace, checks against permission matrix, short-circuits with 403 |
| Cursor pagination  | All list endpoints accept `before` / `after` (Snowflake ID) + `limit` (max 100); return `next_cursor` and `prev_cursor` |

### 3.4 Notification Service
| Concern       | Choice                              |
|---------------|-------------------------------------|
| Push (mobile) | Firebase FCM + Apple APNS           |
| Email         | SMTP relay (Postfix / AWS SES)      |
| Desktop       | Push via WebSocket on reconnect     |
| Preferences   | Per-user per-channel DND schedules  |
| Queue         | NATS JetStream (at-least-once)      |

### 3.5 Search Service
| Concern        | Choice                                     |
|----------------|--------------------------------------------|
| Engine         | Meilisearch (self-hostable, fast, simple)  |
| Fallback       | Typesense or OpenSearch for large deploys  |
| Indexed data   | Messages, files metadata, users, channels  |
| Indexing path  | Async consumer from message broker         |
| Access control | Workspace-scoped index namespaces          |

### 3.6 File Service
| Concern        | Choice                                          |
|----------------|-------------------------------------------------|
| Storage backend| S3-compatible (MinIO self-hosted or AWS S3)    |
| Upload flow    | Presigned URL (client → S3 direct upload)      |
| Thumbnails     | On-demand via libvips / sharp in Lambda-style  |
| Virus scanning | ClamAV async scan post-upload                  |
| Quotas         | Per-workspace storage limits in metadata DB    |

### 3.7 Integration Service
| Concern           | Choice                                                                           |
|-------------------|----------------------------------------------------------------------------------|
| Webhooks          | Outbound HTTP POST with HMAC-SHA256 sig                                         |
| Slash commands    | POST payload to registered endpoint                                              |
| Bot API           | REST + WebSocket (same as client API)                                            |
| App manifest      | JSON schema — declares block types used, scopes required, action endpoints       |
| OAuth apps        | Authorization code flow with fine-grained scopes (see §3.8)                     |
| Block Kit validation | Validates `blocks` JSONB on inbound app messages against block schema registry |
| Action routing    | When user interacts with a block element (button, select), messaging service emits `block_action` event to NATS; integration service routes it to the app's `action_endpoint` via HTTP POST |
| Action response   | Apps respond within 3 s with a message update payload or an ephemeral reply; timeout returns 200 OK and drops the action |
| Socket Mode       | Apps that cannot expose a public endpoint subscribe to actions/events over a dedicated WebSocket connection (separate from client WS gateway) |

---

## 3.9 Media Service — Voice & Video

A dedicated Go service that owns the **control plane** for all calls.
Livekit owns the **media plane** (the actual audio/video streams).

### Responsibilities
| Concern               | Detail                                                                  |
|-----------------------|-------------------------------------------------------------------------|
| Room lifecycle        | Create / end rooms; enforce workspace permissions before creation       |
| Token issuing         | Generate short-lived Livekit JWT access tokens per participant          |
| Participant state     | Track join/leave/mute/video-on/screen-share in DB and Redis             |
| Call signaling        | Publish `room.*` events to NATS → messaging service fans out to clients |
| Call invitations      | Push `call.incoming` WS event to invited users (DM calls)              |
| Recording (Phase 3)   | Trigger Livekit Egress API for cloud recording; store output in MinIO  |

### SFU: Livekit
| Concern          | Detail                                                                       |
|------------------|------------------------------------------------------------------------------|
| What it does     | Routes encoded media tracks between participants without decoding (SFU model)|
| Signaling        | Livekit's own WebRTC signaling over WebSocket (not our WS gateway)          |
| Auth             | Each participant connects with a JWT issued by Media Service                |
| Clustering       | Redis-based Livekit cluster for multi-node deployments                      |
| Ports            | TCP 443 (HTTPS/WSS signaling) + UDP 40000–50000 (media, must be open)      |

### TURN/STUN: Coturn
Clients behind symmetric NAT (corporate firewalls, mobile carrier NAT) cannot
reach Livekit's UDP ports directly. Coturn relays media traffic over TCP/UDP 3478/5349.

| Concern             | Detail                                                               |
|---------------------|----------------------------------------------------------------------|
| Role                | Relay server for ICE candidates that fail direct UDP connection      |
| Credentials         | Ephemeral HMAC credentials, generated per-session by Media Service  |
| TLS                 | TURNS (TURN over TLS) on port 5349                                  |
| Self-hosted default | Bundled in Docker Compose and Helm chart — no external dependency   |

---

## 3.8 Block Kit System (Critical Gap Fix)

The Block Kit is a structured JSON component model for rich interactive messages.
It is the foundation for all third-party app UI.

### Block Type Registry

```
Block (union type)
├── SectionBlock     { text: Text, accessory?: Element }
├── ActionsBlock     { elements: Element[] }           ← interactive
├── InputBlock       { label: Text, element: InputElement, block_id: string }
├── ImageBlock       { image_url, alt_text, title? }
├── ContextBlock     { elements: (Text | Image)[] }
├── DividerBlock     { }
└── HeaderBlock      { text: PlainText }

Element (interactive)
├── ButtonElement    { text, action_id, value, style?: primary|danger }
├── StaticSelect     { placeholder, action_id, options: Option[] }
├── OverflowMenu     { action_id, options: Option[] }
├── DatePicker       { action_id, initial_date? }
└── PlainTextInput   { action_id, placeholder, multiline? }

Text
├── PlainText        { type: "plain_text", text, emoji? }
└── MrkdwnText       { type: "mrkdwn", text }
```

### Validation Pipeline (server-side)

```
App POSTs message with blocks[]
        │
        ▼
Integration Service
  ├── JSON Schema validation (block-schema.json)
  ├── Depth limit check (max 3 levels)
  ├── Element count check (max 25 elements/block)
  └── action_id uniqueness within message
        │
        ▼ (valid)
Messaging Service stores blocks JSONB → fan-out to clients
```

### Flutter Block Renderer

```dart
// core/blocks/block_renderer.dart
class BlockRenderer extends StatelessWidget {
  final List<Block> blocks;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: blocks.map(_renderBlock).toList(),
    );
  }

  Widget _renderBlock(Block block) => switch (block) {
    SectionBlock b  => SectionBlockWidget(block: b),
    ActionsBlock b  => ActionsBlockWidget(block: b),
    InputBlock b    => InputBlockWidget(block: b),
    ImageBlock b    => ImageBlockWidget(block: b),
    ContextBlock b  => ContextBlockWidget(block: b),
    DividerBlock _  => const Divider(),
    HeaderBlock b   => HeaderBlockWidget(block: b),
  };
}

// Action elements dispatch via BlockActionNotifier (Riverpod)
// which sends a WS event: { type: "block_action", action_id, value, message_id }
```

---

## 4. Data Layer

```
┌─────────────────────────────────────────────────────────┐
│                    PRIMARY DATABASE                      │
│                  PostgreSQL (primary)                    │
│                                                         │
│  Schemas:                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │workspaces│  │ channels │  │  users   │             │
│  ├──────────┤  ├──────────┤  ├──────────┤             │
│  │messages  │  │ threads  │  │ members  │             │
│  ├──────────┤  ├──────────┤  ├──────────┤             │
│  │reactions │  │  files   │  │  apps    │             │
│  └──────────┘  └──────────┘  └──────────┘             │
│                                                         │
│  Read replicas for API reads                            │
│  Streaming replication → replica                        │
└────────────────────────┬────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
   ┌──────────┐   ┌──────────┐   ┌──────────────┐
   │  Redis   │   │  NATS    │   │  Meilisearch │
   │          │   │JetStream │   │              │
   │- Sessions│   │          │   │- Message idx │
   │- Presence│   │- Events  │   │- File idx    │
   │- Rate lim│   │- Notifs  │   │- User idx    │
   │- WS map  │   │- DLQ     │   │              │
   │- Cache   │   │          │   │              │
   └──────────┘   └──────────┘   └──────────────┘
```

### Key Schema Decisions

```sql
-- ─────────────────────────────────────────────
-- CRITICAL FIX 1: Member roles & guest access
-- ─────────────────────────────────────────────

CREATE TYPE member_role AS ENUM ('owner', 'admin', 'member', 'guest');

CREATE TABLE workspace_members (
  workspace_id  BIGINT NOT NULL REFERENCES workspaces(id),
  user_id       BIGINT NOT NULL REFERENCES users(id),
  role          member_role NOT NULL DEFAULT 'member',
  joined_at     TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (workspace_id, user_id)
);

-- Guests are restricted to specific channels only
CREATE TABLE guest_channel_access (
  workspace_id  BIGINT NOT NULL,
  user_id       BIGINT NOT NULL,
  channel_id    BIGINT NOT NULL REFERENCES channels(id),
  FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members(workspace_id, user_id),
  PRIMARY KEY (workspace_id, user_id, channel_id)
);

-- Permission matrix (enforced in PermissionGuard middleware)
-- role        | read_public | post | invite | manage_channel | admin_settings
-- ------------|-------------|------|--------|----------------|---------------
-- owner       | ✓           | ✓    | ✓      | ✓              | ✓
-- admin       | ✓           | ✓    | ✓      | ✓              | ✗ (workspace-delete only)
-- member      | ✓           | ✓    | ✓      | ✗              | ✗
-- guest       | ✓ (allowed  | ✓    | ✗      | ✗              | ✗
--             |  channels)  |      |        |                |

-- ─────────────────────────────────────────────
-- Workspaces (multi-tenant root)
-- ─────────────────────────────────────────────
CREATE TABLE workspaces (
  id          BIGINT PRIMARY KEY,  -- Snowflake ID
  slug        TEXT UNIQUE NOT NULL,
  name        TEXT NOT NULL,
  plan        TEXT DEFAULT 'community',
  created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ─────────────────────────────────────────────
-- CRITICAL FIX 2: Block Kit columns on messages
-- ─────────────────────────────────────────────

CREATE TABLE messages (
  id            BIGINT PRIMARY KEY,        -- Snowflake (time-ordered)
  channel_id    BIGINT NOT NULL REFERENCES channels(id),
  parent_id     BIGINT REFERENCES messages(id),  -- NULL = top-level
  user_id       BIGINT NOT NULL REFERENCES users(id),
  app_id        BIGINT REFERENCES apps(id),      -- NULL = human message

  -- Plain text fallback (always required for notifications/search)
  content       TEXT,

  -- Block Kit payload (NULL for plain messages)
  -- Validated against block schema registry on write
  blocks        JSONB,

  -- 'markdown' | 'blocks' | 'system'
  content_type  TEXT NOT NULL DEFAULT 'markdown',

  reply_count       INT NOT NULL DEFAULT 0,
  edited_at         TIMESTAMPTZ,
  deleted_at        TIMESTAMPTZ,   -- soft delete
  created_at        TIMESTAMPTZ DEFAULT NOW(),

  CONSTRAINT must_have_content CHECK (content IS NOT NULL OR blocks IS NOT NULL)
);

CREATE INDEX ON messages (channel_id, id DESC);
CREATE INDEX ON messages (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX ON messages USING GIN (blocks) WHERE blocks IS NOT NULL;

-- ─────────────────────────────────────────────
-- Voice & Video: rooms and participants
-- ─────────────────────────────────────────────

CREATE TYPE room_type   AS ENUM ('huddle', 'call');
CREATE TYPE room_status AS ENUM ('active', 'ended');

CREATE TABLE rooms (
  id              BIGINT PRIMARY KEY,               -- Snowflake
  workspace_id    BIGINT NOT NULL REFERENCES workspaces(id),
  channel_id      BIGINT REFERENCES channels(id),   -- NULL for DM calls
  type            room_type   NOT NULL DEFAULT 'huddle',
  status          room_status NOT NULL DEFAULT 'active',
  livekit_room_id TEXT NOT NULL UNIQUE,             -- room name in Livekit
  created_by      BIGINT NOT NULL REFERENCES users(id),
  started_at      TIMESTAMPTZ DEFAULT NOW(),
  ended_at        TIMESTAMPTZ
);

CREATE INDEX ON rooms (channel_id) WHERE status = 'active';

CREATE TABLE room_participants (
  room_id           BIGINT NOT NULL REFERENCES rooms(id),
  user_id           BIGINT NOT NULL REFERENCES users(id),
  joined_at         TIMESTAMPTZ DEFAULT NOW(),
  left_at           TIMESTAMPTZ,
  is_muted          BOOLEAN NOT NULL DEFAULT FALSE,
  is_video_on       BOOLEAN NOT NULL DEFAULT FALSE,
  is_screen_sharing BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (room_id, user_id)
);

-- ─────────────────────────────────────────────
-- Cross-org Federation schema (§13)
-- ─────────────────────────────────────────────

-- This server's identity (one row, created on first boot)
CREATE TABLE server_identity (
  id          BIGINT PRIMARY KEY,
  domain      TEXT NOT NULL UNIQUE,       -- e.g. "chat.acme.com"
  public_key  TEXT NOT NULL,              -- Ed25519 PEM (published at .well-known)
  private_key TEXT NOT NULL,              -- Ed25519 PEM (encrypted at rest via Vault/KMS)
  created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Remote servers we have a federation relationship with
CREATE TYPE federation_status AS ENUM ('pending', 'allowed', 'blocked');

CREATE TABLE federation_servers (
  id             BIGINT PRIMARY KEY,
  domain         TEXT NOT NULL UNIQUE,    -- e.g. "chat.partner.com"
  display_name   TEXT,                    -- pulled from their .well-known
  public_key     TEXT,                    -- their Ed25519 PEM (fetched + pinned on first contact)
  status         federation_status NOT NULL DEFAULT 'pending',
  approved_by    BIGINT REFERENCES users(id),
  approved_at    TIMESTAMPTZ,
  last_seen_at   TIMESTAMPTZ,
  created_at     TIMESTAMPTZ DEFAULT NOW()
);

-- Shadow records for users who live on remote servers
CREATE TABLE federated_users (
  id             BIGINT PRIMARY KEY,      -- local Snowflake (for FK use)
  remote_id      TEXT NOT NULL,           -- their user ID string on home server
  home_server    TEXT NOT NULL REFERENCES federation_servers(domain),
  username       TEXT NOT NULL,           -- e.g. "bob"
  global_handle  TEXT NOT NULL UNIQUE,    -- e.g. "bob@chat.partner.com"
  display_name   TEXT,
  avatar_url     TEXT,
  email_domain   TEXT,                    -- domain portion of their email (for policy checks)
  last_synced_at TIMESTAMPTZ,
  UNIQUE (remote_id, home_server)
);

-- Domain-based user classification per workspace
CREATE TYPE domain_classification AS ENUM ('internal', 'external', 'blocked');

CREATE TABLE workspace_domain_policies (
  workspace_id     BIGINT NOT NULL REFERENCES workspaces(id),
  domain           TEXT NOT NULL,          -- e.g. "acme.com", "partner.com", or "*" (catch-all)
  classification   domain_classification NOT NULL DEFAULT 'external',
  sort_order       INT NOT NULL DEFAULT 0, -- lower = evaluated first; "*" always last
  PRIMARY KEY (workspace_id, domain)
);

-- Example rows for Acme Corp workspace:
-- (1, 'acme.com',    'internal',  1)
-- (1, 'partner.com', 'external',  2)   ← invited partners, shown as external
-- (1, '*',           'external',  99)  ← everyone else also external by default

-- Federation settings on workspaces (ALTER existing table)
ALTER TABLE workspaces
  ADD COLUMN federation_enabled         BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN require_admin_approval     BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN external_can_create_channels BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN external_can_dm_internal   BOOLEAN NOT NULL DEFAULT FALSE;

-- Federated channels: channels with cross-server participants
CREATE TABLE federated_channels (
  channel_id        BIGINT NOT NULL REFERENCES channels(id) PRIMARY KEY,
  home_server       TEXT NOT NULL,         -- domain of the server that owns this channel
  remote_channel_id TEXT,                  -- channel ID on home server (null if WE are home)
  federation_id     TEXT NOT NULL UNIQUE   -- stable global ID: "{home_server}/{channel_id}"
);

-- Servers that are members of a federated channel
CREATE TABLE channel_federation_servers (
  channel_id    BIGINT NOT NULL REFERENCES channels(id),
  server_domain TEXT NOT NULL REFERENCES federation_servers(domain),
  status        TEXT NOT NULL DEFAULT 'active',  -- active | left | removed
  joined_at     TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (channel_id, server_domain)
);

-- Audit log for all inbound and outbound federation events
CREATE TABLE federation_audit (
  id            BIGINT PRIMARY KEY,
  direction     TEXT NOT NULL,             -- 'inbound' | 'outbound'
  remote_server TEXT NOT NULL,
  event_type    TEXT NOT NULL,             -- 'message' | 'invite' | 'join' | 'leave' | 'revoke'
  payload       JSONB NOT NULL,
  status        TEXT NOT NULL DEFAULT 'ok', -- ok | rejected | error
  created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- ─────────────────────────────────────────────
-- Ephemeral TURN credentials (short TTL, generated per session)
CREATE TABLE turn_credentials (
  id          BIGINT PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES users(id),
  username    TEXT NOT NULL,
  password    TEXT NOT NULL,   -- HMAC-SHA1 derived
  expires_at  TIMESTAMPTZ NOT NULL,
  created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- ─────────────────────────────────────────────
-- CRITICAL FIX 4: Block Kit app action dispatch
-- ─────────────────────────────────────────────

CREATE TABLE apps (
  id                  BIGINT PRIMARY KEY,
  workspace_id        BIGINT NOT NULL REFERENCES workspaces(id),
  name                TEXT NOT NULL,
  action_endpoint     TEXT,          -- HTTPS URL for interactive callbacks
  socket_mode         BOOLEAN DEFAULT FALSE,
  signing_secret      TEXT NOT NULL, -- HMAC-SHA256 key
  scopes              TEXT[] NOT NULL DEFAULT '{}',
  manifest            JSONB,         -- full app manifest
  created_at          TIMESTAMPTZ DEFAULT NOW()
);

-- Audit trail for every action dispatched to an app
CREATE TABLE action_deliveries (
  id            BIGINT PRIMARY KEY,
  app_id        BIGINT NOT NULL REFERENCES apps(id),
  message_id    BIGINT REFERENCES messages(id),
  action_id     TEXT NOT NULL,       -- block_id + action_id from payload
  user_id       BIGINT NOT NULL REFERENCES users(id),
  payload       JSONB NOT NULL,
  status        TEXT NOT NULL DEFAULT 'pending',  -- pending|delivered|timeout|failed
  delivered_at  TIMESTAMPTZ,
  created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- ─────────────────────────────────────────────
-- CRITICAL FIX 3: Cursor pagination support
-- (unread tracking per member per channel)
-- ─────────────────────────────────────────────

CREATE TABLE channel_members (
  channel_id         BIGINT NOT NULL REFERENCES channels(id),
  user_id            BIGINT NOT NULL REFERENCES users(id),
  last_read_id       BIGINT,   -- Snowflake ID of last read message (cursor)
  mention_count      INT NOT NULL DEFAULT 0,
  notification_level TEXT NOT NULL DEFAULT 'all',  -- all|mentions|nothing
  joined_at          TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (channel_id, user_id)
);
```

### Cursor Pagination API Contract

All message list endpoints use Snowflake-ID cursors. This pairs naturally
with the Drift local cache for offline-first rendering.

```
GET /v1/channels/:id/messages?limit=50&before=<snowflake_id>
GET /v1/channels/:id/messages?limit=50&after=<snowflake_id>

Response:
{
  "messages": [...],
  "next_cursor": "<snowflake_id>",   // null if no older messages
  "prev_cursor": "<snowflake_id>",   // null if at newest
  "has_more":   true
}
```

- `before` → load older history (scroll up)
- `after`  → load newer messages (scroll down / catch-up)
- Initial load uses no cursor → returns newest 50 and a `prev_cursor` for history

---

## 5. Message Flow (Send a Message)

```
Client                WS Gateway         NATS            DB         Notification
  │                       │               │               │              │
  │── WS: send_message ──►│               │               │              │
  │                       ├─PermissionGuard (role check)  │              │
  │                       ├─ Block validator (if blocks≠∅)│              │
  │                       │── INSERT ────────────────────►│              │
  │                       │◄─ message_id ─────────────────│              │
  │                       │── PUBLISH msg ►│               │              │
  │◄── WS: message_ack ───│               │               │              │
  │                       │               │── fan-out ───►│ (other WS    │
  │                       │               │               │  subscribers)│
  │                       │               │── notify ───────────────────►│
  │                       │               │               │  (FCM/APNS)  │
  │                       │               │── index ──────────────────── (Search)
```

## 5b. Interactive Action Flow (Critical Gap Fix)

Triggered when a user clicks a button, selects from a menu, or submits an input
inside a Block Kit message.

```
Flutter Client        WS Gateway      NATS             Integration Svc    App Server
     │                    │             │                     │                │
     │─ WS: block_action ►│             │                     │                │
     │  { action_id,      │             │                     │                │
     │    block_id,       │─PUBLISH────►│                     │                │
     │    value,          │  block_     │                     │                │
     │    message_id }    │  actions    │──────consume────────►│                │
     │                    │             │                     ├─ lookup app    │
     │                    │             │                     ├─ verify scope  │
     │                    │             │                     │                │
     │                    │             │       ── HTTP POST (HMAC signed) ───►│
     │                    │             │       { action_id, user, message,    │
     │                    │             │         channel, workspace }         │
     │                    │             │                     │◄── 200 + response payload
     │                    │             │                     │    (update_message | ephemeral)
     │                    │             │                     │                │
     │                    │◄─PUBLISH────│◄────────────────────│                │
     │◄─ WS: message_updated            │  (fan-out updated   │                │
     │   or ephemeral_msg │             │   blocks to channel)│                │
     │                    │             │                     │                │

Timeout: If app does not respond within 3 s → action_delivery.status = 'timeout'
         Client receives WS: action_ack { status: "timeout" } and re-enables UI.

Socket Mode (apps without public endpoint):
  App maintains a WebSocket to Integration Service.
  Integration Service pushes action payloads over that socket instead of HTTP POST.
```

---

## 5c. Voice & Video Call Flows

### Architecture Overview
```
                ┌──────────────────────────────────────────────────┐
                │                CONTROL PLANE                     │
                │                                                  │
  Flutter ──────► API Gateway ──► Media Service ──► PostgreSQL     │
  Client   REST │                     │           (rooms table)    │
                │                     │                            │
                │                     ├──► NATS (room.* events)   │
                │                     │         │                  │
                │                     │         ▼                  │
                │                     │    Messaging Svc           │
                │                     │    (WS fan-out to          │
                │                     │     channel members)       │
                │                     │                            │
                │                     ├──► Livekit API (create rm) │
                └─────────────────────┼──────────────────────────┘
                                      │
                ┌─────────────────────▼──────────────────────────┐
                │                 MEDIA PLANE                     │
                │                                                 │
  Flutter ──────►          Livekit SFU                           │
  Client  WSS   │   (WebRTC signaling + media routing)           │
  (livekit_client│                                               │
   Dart SDK)    │   Coturn TURN/STUN (NAT traversal relay)       │
                └────────────────────────────────────────────────┘
```

### Flow A — Huddle (Channel Audio Room)
```
User A (Flutter)     Media Service     Livekit      NATS     Other Members
     │                    │               │            │            │
     │─ POST /rooms ──────►│               │            │            │
     │  { channel_id,      │               │            │            │
     │    type: "huddle" } │─ CreateRoom ─►│            │            │
     │                     │◄─ room_name ──│            │            │
     │                     │─ IssueToken ──────────────────────────  │
     │◄─ { token,          │               │            │            │
     │     livekit_url,    │               │            │            │
     │     turn_creds }    │─ PUBLISH ─────────────────►│            │
     │                     │  room.started │            │            │
     │                     │               │            │── WS push ►│
     │                     │               │            │  huddle    │
     │ (connects directly to Livekit via WebRTC)        │  active    │
     │──────────────────────────────────────────────────────────────►│
     │  WSS: wss://livekit.example.com                               │
     │                               │                               │
     │              User B joins:    │                               │
     │                               │ B ─ POST /rooms/:id/join ────►│
     │                               │   ◄─ { token, turn_creds } ──│
     │                               │ B connects to Livekit         │
     │◄────────── participantJoined event (via Livekit SDK) ─────────│
     │
     │  Last person leaves:
     │─ DELETE /rooms/:id ──────────►│
     │                               │─ EndRoom ────►│
     │                               │─ PUBLISH room.ended ─────────►│
     │                               │  UPDATE rooms SET ended_at    │
```

### Flow B — Video Call (Direct Message)
```
Caller (Flutter)     Media Service     NATS     Callee (Flutter)
     │                    │              │              │
     │─ POST /rooms ──────►│              │              │
     │  { dm_user_id,      │              │              │
     │    type: "call" }   │              │              │
     │◄─ { token, url }    │─ PUBLISH ───►│              │
     │                     │  call.       │─ WS push ───►│
     │ (connects Livekit)  │  incoming    │  ringing UI  │
     │                     │              │              │
     │                     │              │  Callee accepts:
     │                     │◄─ POST /rooms/:id/join ─────│
     │                     │─ IssueToken ─────────────────────────► │
     │                     │◄─ { token, turn_creds } ───────────────│
     │                     │              │  (connects Livekit) ────►│
     │                     │              │              │
     │◄──────── participantJoined (Livekit SDK event) ──────────────│
     │          (video/audio tracks available)                       │
     │
     │  Callee rejects / no answer (30 s timeout):
     │                     │─ PUBLISH call.declined ──────────────► │
     │◄─ WS: call_missed   │              │                          │
```

### Flow C — Screen Sharing
```
Sharer (Flutter)                       Livekit SFU          Other Participants
     │                                      │                       │
     │  // Flutter: capture screen track    │                       │
     │  room.localParticipant               │                       │
     │    .setScreenShareEnabled(true) ────►│                       │
     │                                      │── trackPublished ────►│
     │                                      │   (screen track)      │
     │                                      │                       │
     │  // Media Service notified via       │                       │
     │  // Livekit webhook → updates        │                       │
     │  // room_participants.is_screen_     │                       │
     │  // sharing = true → NATS publish    │                       │
     │                                      │                       │
     │  // Viewers subscribe automatically  │                       │
     │  // via Livekit SDK (auto-subscribe) │                       │
     │                                      │◄─ subscribed ─────────│

Platform notes:
  Android/iOS : Livekit SDK uses ReplayKit (iOS) / MediaProjection (Android)
  Desktop     : flutter_webrtc getDisplayMedia() — window or full-screen picker
  Web         : Browser getDisplayMedia() API — tab, window, or screen
```

### Token & Credential Lifecycle
```
Livekit Access Token (JWT)
  ├── Issued by:   Media Service (signed with LIVEKIT_API_SECRET)
  ├── TTL:         1 hour (re-issued on reconnect)
  ├── Grants:      { roomJoin: true, room: "<room_id>",
  │                  canPublish: true, canSubscribe: true }
  └── Revocation:  End room → Livekit invalidates all tokens for that room

TURN Credentials
  ├── Algorithm:   HMAC-SHA1 (Coturn time-limited credential scheme)
  ├── TTL:         1 hour
  ├── Format:      username = "<ttl>:<user_id>", password = HMAC(secret, username)
  └── Stored in:   turn_credentials table (for audit); deleted after expiry
```

### Livekit Webhook → Media Service
Livekit sends HTTP POST webhooks to Media Service for room lifecycle events.
Media Service uses these to keep DB in sync without polling.

```
Event               Action in Media Service
─────────────────   ──────────────────────────────────────────────
room_started        INSERT rooms row, PUBLISH room.started to NATS
participant_joined  INSERT/UPDATE room_participants, PUBLISH to NATS
participant_left    UPDATE room_participants.left_at, PUBLISH to NATS
track_published     UPDATE is_screen_sharing / is_video_on
room_finished       UPDATE rooms.status = 'ended', rooms.ended_at
```

---

## 6. Infrastructure & Deployment

### Docker Compose (Single-Node / Self-Hosted)
```
services:
  # Data layer
  postgres, redis, nats, meilisearch, minio

  # Application services
  auth-service, api-service, messaging-service
  notification-service, search-worker, file-service
  integration-service, media-service

  # Voice & video (bundled — no external dependency)
  livekit          # SFU — WebRTC media routing
  coturn           # TURN/STUN — NAT traversal relay

  # Frontend
  web              # Nginx serving Flutter web build
```

> **Network note:** Livekit requires UDP ports 40000–50000 to be reachable by clients.
> Coturn requires UDP/TCP 3478 and TCP 5349 (TURNS).
> Expose these in `docker-compose.yml` and any firewall rules.

### Kubernetes (Production)
```
Namespaces: openslack-core, openslack-data, openslack-infra

HPA on: messaging-service (CPU + WS connection count)
        api-service (RPS)
        notification-service (queue depth)

StatefulSets: postgres, nats, meilisearch, minio
Deployments:  all application services
```

### Infra Components
| Component       | Self-Hosted Option        | Cloud Option              |
|-----------------|---------------------------|---------------------------|
| Database        | PostgreSQL (Patroni)      | RDS / Supabase            |
| Cache / Pub-Sub | Redis (Sentinel)          | Elasticache / Upstash     |
| Message Broker  | NATS JetStream            | Confluent Kafka           |
| Object Storage  | MinIO                     | S3 / R2                   |
| Search          | Meilisearch               | Typesense Cloud           |
| Email           | Postfix + Dovecot         | SES / Postmark            |
| Observability   | Grafana + Loki + Otel     | Datadog / Honeycomb       |
| **SFU (media)** | **Livekit (self-hosted)** | **Livekit Cloud**         |
| **TURN/STUN**   | **Coturn**                | **Twilio NTS / Cloudflare** |

---

## 7. Flutter Client Architecture

Flutter compiles to native ARM (iOS/Android), native desktop (Windows/macOS/Linux),
and CanvasKit/HTML (web) from a **single Dart codebase**.

### Platform Targets
| Target   | Output                        | Notes                              |
|----------|-------------------------------|------------------------------------|
| Android  | Native APK / AAB              | FCM push natively supported        |
| iOS      | Native .ipa                   | APNs push natively supported       |
| Web      | CanvasKit WASM bundle         | Served by Nginx; no SEO needed     |
| Windows  | Native Win32 exe              | MSIX packaging                     |
| macOS    | Native .app bundle            | Notarized DMG                      |
| Linux    | Native ELF binary             | AppImage / deb / rpm               |

### Key Packages
| Concern                    | Package                                                       |
|----------------------------|---------------------------------------------------------------|
| State management           | `riverpod` 2.x (AsyncNotifier + Provider)                    |
| Navigation                 | `go_router`                                                   |
| WebSocket                  | `web_socket_channel`                                          |
| HTTP client                | `dio` (interceptors for token refresh)                        |
| Local cache (offline)      | `drift` (SQLite, typesafe)                                   |
| Secure token storage       | `flutter_secure_storage`                                      |
| Push notifications         | `firebase_messaging` + `flutter_apns_only`                   |
| File picking/upload        | `file_picker` + direct S3 presigned upload                    |
| Markdown rendering         | `flutter_markdown`                                            |
| Image/file preview         | `cached_network_image` + `photo_view`                         |
| Emoji picker               | `emoji_picker_flutter`                                        |
| Theming                    | Material 3 + custom `ThemeExtension`                          |
| **Virtual scroll (critical)** | `scrollable_positioned_list` — bi-directional jump-to-position without full rebuild |
| **Block renderer (critical)** | Custom `block_kit` internal package (see §3.8)            |
| **WebRTC media (voice/video)** | `livekit_client` — official Livekit Flutter/Dart SDK; wraps `flutter_webrtc` |
| **Screen capture**            | `livekit_client` screen-share API (uses platform screen-capture APIs internally) |

### App Structure
```
apps/flutter/
├── lib/
│   ├── main.dart
│   ├── app/
│   │   ├── router.dart          # go_router route definitions
│   │   └── theme.dart           # Material 3 theme + dark mode
│   ├── core/
│   │   ├── api/                 # Dio client, interceptors
│   │   ├── ws/                  # WebSocket client + reconnect logic
│   │   ├── storage/             # Drift DB schema + DAOs
│   │   ├── notifications/       # FCM / APNs bridge
│   │   ├── blocks/              # [CRITICAL] Block Kit renderer (§3.8)
│   │   │   ├── block_renderer.dart
│   │   │   ├── widgets/         # SectionBlock, ActionsBlock, ImageBlock …
│   │   │   └── block_action_notifier.dart  # dispatches WS block_action events
│   │   └── permissions/         # [CRITICAL] Local role cache + UI guard helpers
│   ├── features/
│   │   ├── auth/                # Login, register, OAuth, MFA
│   │   ├── workspace/           # Workspace switcher, sidebar
│   │   ├── channels/            # Channel list, channel view
│   │   ├── messaging/           # Message list, composer, threads
│   │   │   └── message_list/
│   │   │       ├── message_list_notifier.dart   # [CRITICAL] cursor-paginated provider
│   │   │       └── message_list_view.dart       # [CRITICAL] virtual scroll widget
│   │   ├── dm/                  # Direct messages
│   │   ├── search/              # Global search overlay
│   │   ├── files/               # File browser, viewer
│   │   ├── huddle/              # Huddle bar (always-on audio overlay)
│   │   │   ├── huddle_notifier.dart    # joins/leaves Livekit room
│   │   │   └── huddle_bar_widget.dart  # floating participant strip
│   │   ├── call/                # Full video call screen
│   │   │   ├── call_notifier.dart      # call state (ringing, active, ended)
│   │   │   ├── call_screen.dart        # video grid, mute/camera/hang-up
│   │   │   └── incoming_call_overlay.dart
│   │   └── settings/            # User + workspace settings
│   ├── core/
│   │   └── media/               # Livekit room manager, TURN credential fetch
│   │       ├── livekit_service.dart
│   │       └── turn_credential_provider.dart
│   └── shared/
│       ├── widgets/             # Reusable UI components
│       └── extensions/          # Dart extensions
├── test/
│   ├── unit/                    # Riverpod notifier tests
│   ├── widget/                  # Widget tests
│   └── integration/             # Integration tests (flutter_test)
├── android/
├── ios/
├── web/
├── windows/
├── macos/
└── linux/
```

### State Architecture (Riverpod)
```
UI Widget
  └── watches AsyncNotifierProvider
        ├── reads from Drift (local cache, instant render)
        └── fetches from API / WebSocket (updates cache)

WebSocket events → invalidate providers → UI rebuilds reactively
```

### Virtual Scroll + Cursor Pagination (Critical Gap Fix)

The message list must never hold the full history in memory.

```
MessageListNotifier (AsyncNotifier)
  ├── State: List<Message> window (~100 messages in memory)
  ├── initialLoad()     → GET /messages?limit=50          (newest 50, store prev_cursor)
  ├── loadOlder()       → GET /messages?before=<cursor>   (triggered: scroll to top)
  ├── loadNewer()       → GET /messages?after=<cursor>    (triggered: scroll to bottom)
  ├── onNewWsMessage()  → prepend to list, evict tail if window > 200
  └── jumpToMessage(id) → GET /messages?around=<id>       (deep-link / search result)

MessageListView widget:
  ScrollablePositionedList (scrollable_positioned_list package)
    ├── itemCount: state.messages.length + 2  (sentinel items at top/bottom)
    ├── onTop reached    → notifier.loadOlder()  → items prepended → scroll position preserved
    ├── onBottom reached → notifier.loadNewer()
    └── each item: content_type == 'blocks'
                    ? BlockRenderer(blocks: msg.blocks)
                    : MarkdownMessage(content: msg.content)
```

**Drift local cache strategy:**
```
messages table (Drift) — stores last 500 messages per channel
  ├── On app open: render from Drift immediately (zero loading state)
  ├── Fetch fresh from API in background, upsert diff
  └── On eviction: delete rows where channel_id = ? ORDER BY id ASC LIMIT evict_count
```

### Client WebSocket State Machine
```
DISCONNECTED → CONNECTING → AUTHENTICATING → CONNECTED
      ▲                                          │
      └────────── RECONNECT (exp. backoff) ──────┘
                  (messages queued locally in Drift
                   and replayed on reconnect)
```

### Platform-Specific Adaptations
| Feature            | Mobile (iOS/Android)           | Desktop (Win/Mac/Linux)       | Web                        |
|--------------------|-------------------------------|-------------------------------|----------------------------|
| Window layout      | Bottom nav + drawer           | 3-pane (sidebar+list+main)    | 3-pane                     |
| Notifications      | FCM / APNs native             | Local notification plugin     | Browser API                |
| File upload        | Camera + file picker          | Native file dialog            | `<input type=file>`        |
| Keyboard shortcuts | N/A                           | Full shortcut map             | Partial                    |
| Deep links         | App links / Universal         | Custom URI scheme             | URL routing                |
| Microphone access  | `permission_handler` + Info.plist | OS permission dialog     | `getUserMedia()` prompt    |
| Camera access      | `permission_handler` + Info.plist | OS permission dialog     | `getUserMedia()` prompt    |
| Screen share       | ReplayKit (iOS) / MediaProjection (Android) | `getDisplayMedia` via flutter_webrtc | Browser `getDisplayMedia` |
| Background audio   | AVAudioSession (iOS) / ForegroundService (Android) | OS-managed   | Not supported (tab must be active) |

---

## 8. Security Design

| Concern                  | Approach                                          |
|--------------------------|---------------------------------------------------|
| Transport                | TLS 1.3 everywhere, HSTS                         |
| Auth tokens              | Short-lived JWT + refresh token rotation          |
| WebSocket auth           | Token in first WS message (not URL param)         |
| Channel authorization    | Middleware check on every WS event + REST call    |
| File access              | Presigned URLs (15 min expiry), scoped by member  |
| Secrets management       | Env vars → Vault / Kubernetes secrets             |
| Dependency scanning      | Renovate bot + Trivy in CI                        |
| OWASP                    | CSP headers, parameterized queries, input sanitize|
| Audit log                | Append-only `audit_events` table per workspace    |

---

## 9. Repository Structure (Monorepo)

```
openslack/
├── apps/
│   └── flutter/           # Single Flutter app (all 6 platforms)
│       ├── lib/
│       ├── android/
│       ├── ios/
│       ├── web/
│       ├── windows/
│       ├── macos/
│       └── linux/
├── services/
│   ├── auth/              # Go
│   ├── api/               # Go
│   ├── messaging/         # Go
│   ├── notification/      # Go
│   ├── search/            # Go (consumer) + Meilisearch
│   ├── file/              # Go
│   ├── integration/       # Go
│   ├── media/             # Go — room lifecycle, token issuing, Livekit + Coturn integration
│   └── federation/        # Go — S2S API, discovery, signing, domain policy, invite flows
├── packages/
│   ├── proto/             # Protobuf definitions (shared)
│   ├── sdk-dart/          # Dart client SDK (used by Flutter app)
│   └── sdk-go/            # Go client SDK (for bots/integrations)
├── infra/
│   ├── docker/            # Docker Compose files
│   ├── k8s/               # Kubernetes manifests / Helm chart
│   └── terraform/         # Optional cloud infra
├── docs/
│   ├── architecture/      # ADRs (Architecture Decision Records)
│   ├── api/               # OpenAPI spec
│   └── contributing.md
└── .github/
    ├── workflows/         # CI: test, lint, build, push image
    └── CODEOWNERS
```

---

## 13. Cross-Org Federation Layer

Allows users on **separately hosted OpenSlack instances** to share channels and
exchange DMs without either server needing access to the other's database.
Modelled on email federation (DNS discovery + signed server-to-server HTTP)
rather than Matrix (too complex) or ActivityPub (designed for social posts, not chat).

---

### 13.1 Server Identity & DNS Discovery

Every server has a permanent Ed25519 keypair stored in `server_identity`.
The public key and federation endpoint are published at a well-known URL.

```
GET https://chat.acme.com/.well-known/openslack
→ 200 OK
{
  "domain":               "chat.acme.com",
  "version":              "1",
  "display_name":         "Acme Corp Chat",
  "federation_endpoint":  "https://chat.acme.com/federation/v1",
  "public_key": {
    "id":             "https://chat.acme.com/.well-known/openslack#main-key",
    "type":           "Ed25519",
    "public_key_pem": "-----BEGIN PUBLIC KEY-----\n..."
  },
  "icon_url": "https://chat.acme.com/icon.png"
}
```

**Discovery flow (server-A resolves server-B):**
```
1. Parse handle: "bob@chat.partner.com"  →  domain = "chat.partner.com"
2. Fetch GET https://chat.partner.com/.well-known/openslack
3. Verify TLS certificate (standard CA chain)
4. Pin public_key on first contact (TOFU — Trust On First Use)
5. INSERT INTO federation_servers (domain, public_key, status='pending')
6. If require_admin_approval = TRUE → notify workspace admins for approval
   else → auto-approve and set status='allowed'
```

---

### 13.2 Federation Service (New Go Service)

```
services/federation/
├── server.go               # HTTP server for inbound S2S API
├── discovery/
│   ├── resolver.go         # fetch + cache .well-known for remote servers
│   └── wellknown.go        # serve /.well-known/openslack for ourselves
├── signing/
│   ├── signer.go           # sign outbound requests with Ed25519 private key
│   └── verifier.go         # verify inbound request signatures
├── relay/
│   ├── outbound.go         # consume NATS federation.outbound → HTTP POST to remotes
│   └── inbound.go          # receive inbound events → validate → publish to NATS
├── trust/
│   ├── registry.go         # federation_servers table CRUD
│   └── policy.go           # domain classification lookups (workspace_domain_policies)
└── invite/
    ├── channel_invite.go   # cross-org channel invite flows
    └── dm_invite.go        # cross-org DM invite flows
```

| Concern              | Detail                                                                      |
|----------------------|-----------------------------------------------------------------------------|
| Language             | Go                                                                          |
| Inbound API          | `POST /federation/v1/messages` `POST /federation/v1/invites` etc.           |
| Outbound delivery    | NATS consumer on `federation.outbound.*` → HTTP POST to remote endpoints    |
| Retry               | Exponential backoff (1 s → 2 s → 4 s … max 1 h); dead-letter after 24 h   |
| Rate limiting        | Per remote-server sliding window in Redis (prevent flood from rogue servers)|
| Blocklist check      | Every inbound request checked against `federation_servers.status` first     |

---

### 13.3 HTTP Request Signing (Server-to-Server Auth)

All S2S HTTP requests are signed using the sending server's Ed25519 private key.
The receiving server verifies using the sender's public key (pinned on first contact).

```
POST https://chat.partner.com/federation/v1/messages
Host:   chat.partner.com
Date:   Mon, 01 Mar 2026 10:00:00 GMT
Digest: SHA-256=<base64(sha256(request_body))>
Signature: keyId="https://chat.acme.com/.well-known/openslack#main-key",
           algorithm="ed25519",
           headers="(request-target) host date digest",
           signature="<base64(ed25519_sign(signing_string, private_key))>"
```

**Signing string construction:**
```
(request-target): post /federation/v1/messages
host: chat.partner.com
date: Mon, 01 Mar 2026 10:00:00 GMT
digest: SHA-256=abc123...
```

**Verification steps (receiving server):**
```
1. Parse keyId from Signature header → extract sender domain
2. Lookup sender in federation_servers table
3. If status = blocked → 403 Forbidden
4. If public_key not yet pinned → fetch .well-known, pin it, set status=pending
5. Reconstruct signing string from request headers
6. Verify Ed25519 signature against pinned public key
7. Check Date header within ±5 min (replay attack prevention)
8. Check Digest matches request body (body integrity)
9. If all pass → process event
```

---

### 13.4 Domain Policy & User Classification

Per-workspace rules classify every user as `internal`, `external`, or `blocked`
based on their email domain. Evaluated in `sort_order` order; `*` is the catch-all.

```
Evaluation (at join time and on every API call):

user.email_domain → walk workspace_domain_policies ORDER BY sort_order ASC
  match 'acme.com'    → internal      ✓ full access
  match 'partner.com' → external      ✓ limited access (see matrix below)
  match '*'           → external      (default catch-all)
  match 'spammer.io'  → blocked       ✗ reject immediately
```

**Classification → Permission matrix:**

| Permission                       | internal | external | blocked |
|----------------------------------|----------|----------|---------|
| Post in shared channels          | ✓        | ✓        | ✗       |
| See member list of shared channel| ✓        | shared channel members only | ✗ |
| DM internal users                | ✓        | if `external_can_dm_internal = TRUE` | ✗ |
| Create channels                  | ✓        | if `external_can_create_channels = TRUE` | ✗ |
| Install apps                     | ✓        | ✗        | ✗       |
| See other workspaces/channels    | ✓        | ✗ (scoped to joined channels) | ✗ |
| Shown with badge in UI           | none     | "External" amber badge | — |

**External badge rule:**
A user is shown as External if their classification in the *viewing* workspace is `external`,
regardless of their role on their home server.

---

### 13.5 Global Message & User Identity

Local Snowflake IDs are not unique across servers. Federated entities use a
stable global ID string for routing and deduplication.

```
User global handle:   alice@chat.acme.com
                      └─ username ─┘ └─── home server ───┘

Message global ID:    chat.acme.com/7891234567890123456
                      └─ home server ─┘ └─── snowflake ───┘

Channel federation ID: chat.acme.com/1234567890
                       └─ home server ─┘ └── channel snowflake ──┘
```

Remote users are stored as `federated_users` rows with a local surrogate
`id` (Snowflake) for FK use inside our DB. The `global_handle` is the
stable cross-server identity shown in the UI.

---

### 13.6 Federated Channel Model

A federated channel has exactly **one home server** (the creator).
All other participating servers hold a **local mirror** that receives
events from the home server and relays local member posts back.

```
chat.acme.com  (home server — owns channel)
     │
     │  S2S: signed HTTP POST
     ├──────────────────────────────► chat.partner.com (mirror)
     │                                  └── local members see the channel
     │                                  └── their posts relay back to acme
     └──────────────────────────────► chat.startup.io  (mirror)
                                        └── same
```

**Channel mirror creation (when a remote server joins):**
```sql
-- On the remote (mirror) server:
INSERT INTO channels          (id, workspace_id, name, type)  -- local copy
INSERT INTO federated_channels(channel_id, home_server,
                               remote_channel_id, federation_id)
INSERT INTO channel_federation_servers(channel_id, server_domain)
```

---

### 13.7 S2S Message Delivery Flow

```
Alice posts in federated channel (chat.acme.com is home)

Alice (Flutter)   Messaging Svc   NATS                Federation Svc     Remote Servers
     │                 │            │                       │                   │
     │─ WS: send_msg ─►│            │                       │                   │
     │                 │─ INSERT ──►│                       │                   │
     │◄─ WS: ack ──────│            │                       │                   │
     │                 │─ PUBLISH ──►  messages.*            │                   │
     │                 │            │  federation.outbound ──►│                  │
     │                 │            │                       │                   │
     │                 │            │               for each remote server:     │
     │                 │            │                       │── POST /federation/v1/messages
     │                 │            │                       │   (signed, body = message JSON)
     │                 │            │                       │──────────────────►│
     │                 │            │                       │◄── 200 OK ────────│
     │                 │            │                       │                   │
     │                 │            │          Remote server on receipt:        │
     │                 │            │                       │   verify signature│
     │                 │            │                       │   INSERT message  │
     │                 │            │                       │   PUBLISH to NATS │
     │                 │            │                       │   fan-out WS ────►│ (Bob's client)

Failure handling:
  4xx (blocked/auth fail) → mark server as error, alert admin, stop retrying
  5xx / timeout           → retry with exponential backoff up to 24 h
  24 h no delivery        → dead-letter, notify channel admins on both sides
```

**S2S message payload:**
```json
{
  "federation_id":  "chat.acme.com/7891234567890",
  "channel_fid":    "chat.acme.com/1234567890",
  "sender":         "alice@chat.acme.com",
  "content":        "Hey team!",
  "blocks":         null,
  "content_type":   "markdown",
  "parent_fid":     null,
  "created_at":     "2026-03-01T10:00:00Z",
  "server_version": "1"
}
```

---

### 13.8 Cross-Org Invite Flows

#### Flow A — Federated Channel Invite (admin invites a remote server)
```
Admin (acme)         API Svc          Federation Svc      Remote Fed Svc   Remote Admin
     │                  │                   │                   │               │
     │─ POST /federation/channels/:id/invite │                   │               │
     │  { server: "chat.partner.com" }       │                   │               │
     │                  │─ resolve .well-known ──────────────────►│               │
     │                  │◄─ public_key, endpoint ────────────────│               │
     │                  │                   │                   │               │
     │                  │── publish ────────►│                   │               │
     │                  │   fed.outbound     │── POST /federation/v1/invites ───►│
     │                  │                   │   { channel_fid, channel_name,    │
     │                  │                   │     invited_by, home_server }     │
     │                  │                   │                   │               │
     │                  │                   │         (if require_admin_approval)│
     │                  │                   │                   │──WS push ─────►
     │                  │                   │                   │  "Acme Corp   │
     │                  │                   │                   │  invited your │
     │                  │                   │                   │  server"      │
     │                  │                   │                   │               │
     │                  │                   │◄── POST /federation/v1/invites/accept
     │                  │                   │    { channel_fid, server: "chat.partner.com" }
     │                  │                   │                   │               │
     │                  │── NATS: channel.federation_joined ────►│               │
     │◄─ WS: partner.com joined shared channel ────────────────               │
```

#### Flow B — Individual Cross-Org DM
```
Alice (acme)       API Svc       Federation Svc     Remote Fed Svc    Bob (partner)
     │                │               │                   │                │
     │ types: @bob@chat.partner.com   │                   │                │
     │─ POST /federation/dm/initiate  │                   │                │
     │  { handle: "bob@chat.partner.com" }                │                │
     │                │─ resolve ─────►│                   │                │
     │                │                │── POST /federation/v1/dm/invite ──►│
     │                │                │   { from: "alice@chat.acme.com",  │
     │                │                │     message: "Hi Bob!" }           │
     │                │                │                   │── WS push ────►│
     │                │                │                   │   incoming DM  │
     │                │                │                   │   from external│
     │                │                │◄── /dm/accept ────│◄── accepts ────│
     │                │                │                   │                │
     │◄─ local DM room created with federated_user shadow record            │
     │   subsequent messages flow via S2S                                   │
```

#### Flow C — Individual User Invite (join via email link)
```
Admin sends invite: "Join our OpenSlack at chat.acme.com"
                     → email with signed invite token
Bob (external)       clicks link → registers on chat.acme.com
                     → assigned role=guest in workspace
                     → email domain checked against domain policies
                     → classified as internal/external automatically
```

---

### 13.9 Flutter UI — Federation-Aware Changes

**Federated channel indicator:**
```
Sidebar channel list:
  # general                 ← local channel (no badge)
  # 🔗 partner-collab       ← federated channel (link icon + server count tooltip)
  # 🔗 startup-sync  [2]    ← 2 remote servers participating
```

**External user badge:**
```
Message from Bob (bob@chat.partner.com):
  ┌─────────────────────────────────────────┐
  │ 🟠 Bob Smith  [External]                │   ← amber badge, hover shows home server
  │    Hey team, looks good!                │
  └─────────────────────────────────────────┘
```

**DM composer — global handle support:**
```
New DM search:
  [ @bob@chat.partner.com ________________ ]
    └── Federation Service resolves handle
        → creates federated_user shadow record if new
        → opens DM with External badge visible
```

**Admin settings — Federation Management UI:**
```
Settings → Federation
  ┌─────────────────────────────────────────────────────┐
  │ Federation          [ Enabled ✓ ]                   │
  │ Require admin approval for new servers  [ On  ✓ ]  │
  │ External users can DM internal          [ Off ]     │
  │ External users can create channels      [ Off ]     │
  │                                                     │
  │ Domain Policies                         [+ Add]     │
  │  acme.com       → Internal                          │
  │  partner.com    → External                          │
  │  *              → External (default)                │
  │                                                     │
  │ Connected Servers                                   │
  │  chat.partner.com   Allowed   ✓  [Block] [Details] │
  │  chat.startup.io    Pending   ⏳ [Approve] [Reject] │
  │  spammer.io         Blocked   🚫 [Unblock]          │
  └─────────────────────────────────────────────────────┘
```

**New Flutter packages needed:**
| Package | Purpose |
|---------|---------|
| No new packages required | All federation is server-side; client just renders `external` badge and `🔗` indicator based on API response fields |

**New API response fields the Flutter app must handle:**
```dart
// Message model additions
class Message {
  // existing fields ...
  final String?  senderGlobalHandle;  // "alice@chat.acme.com" — null for local users
  final bool     isFederated;         // true if message came via S2S
}

// User model additions
class User {
  // existing fields ...
  final String?          homeServer;       // null for local users
  final UserClassification classification; // internal | external | blocked
}

// Channel model additions
class Channel {
  // existing fields ...
  final bool         isFederated;
  final List<String> federatedServers;  // list of remote server domains
}
```

---

### 13.10 Deployment Additions

**Docker Compose additions:**
```yaml
services:
  federation-service:
    image: openslack/federation:latest
    environment:
      - FEDERATION_ENABLED=true
      - SERVER_DOMAIN=chat.acme.com
      - LIVEKIT_PRIVATE_KEY_PATH=/secrets/ed25519.pem
    ports:
      - "443"   # served via Nginx — /federation/v1/* and /.well-known/openslack
```

**Nginx routing additions:**
```nginx
location /.well-known/openslack {
    proxy_pass http://federation-service:8080/wellknown;
}
location /federation/v1/ {
    proxy_pass http://federation-service:8080/;
    # Note: do NOT strip Authorization headers — needed for signature verification
}
```

**Required for self-hosters:**
- A valid TLS certificate on the public domain (Let's Encrypt is fine)
- The domain must be publicly reachable for S2S traffic (not localhost)
- UDP is not required — federation is pure HTTPS

---

### 13.11 Security Model

| Threat                          | Mitigation                                                              |
|---------------------------------|-------------------------------------------------------------------------|
| Message spoofing                | Ed25519 signature on every S2S request; invalid sig → 403              |
| Replay attacks                  | Date header must be within ±5 min; Digest header binds sig to body     |
| Rogue server impersonation      | Public key pinned on first contact (TOFU); key rotation requires admin re-approval |
| Spam from federated users       | Domain policy `blocked` → 403; rate limiting per remote server in Redis |
| Data exfiltration by remote server | Remote server only receives messages in channels it was explicitly invited to |
| Admin approval bypass           | `require_admin_approval = TRUE` by default; auto-approve opt-in only   |
| Key compromise                  | Private key encrypted at rest (Vault/KMS); rotation invalidates all active S2S sessions |
| External user privilege escalation | Classification re-evaluated on every API call; roles cannot exceed `external` cap |
| SSRF via .well-known fetch      | DNS allowlist + private IP range block on outbound resolution           |

---

## 10. Key ADRs (Architecture Decision Records)

| #  | Decision | Rationale |
|----|----------|-----------|
| 1  | Go for all backend services | Low memory per goroutine ideal for 10k+ WS connections |
| 2  | NATS JetStream over Kafka | Simpler ops, built-in at-least-once, lower resource floor |
| 3  | Snowflake IDs over UUID | Time-ordered → efficient B-tree index, natural cursor pagination |
| 4  | Meilisearch over Elasticsearch | 10x simpler to operate; sufficient for <100M messages |
| 5  | Presigned S3 URLs for uploads | Offloads bandwidth from app servers entirely |
| 6  | Soft-delete messages | Legal compliance, audit, and "edited" message history |
| 7  | Per-workspace Postgres schema | Simpler tenant isolation than row-level; avoids RLS overhead |
| 8  | Flutter for all clients | Single Dart codebase targets iOS, Android, Web, Windows, macOS, Linux |
| 9  | Riverpod over BLoC | Less boilerplate, compile-safe providers, trivial to test AsyncNotifiers |
| 10 | Drift (SQLite) for local cache | Offline-first message cache; type-safe queries; works on all Flutter platforms |
| 11 | **[CRITICAL] blocks JSONB + plain text fallback** | Enables full Block Kit rendering; `content` always required so notifications and search indexing never break on interactive messages |
| 12 | **[CRITICAL] member_role ENUM on workspace_members** | Role lives at workspace scope matching Slack's model; guest channel restriction in a separate join table avoids nullable flag columns |
| 13 | **[CRITICAL] Snowflake-ID cursor pagination (before/after)** | Stable under concurrent inserts (no OFFSET drift); maps directly to Drift cache eviction boundaries; enables bi-directional infinite scroll |
| 14 | **[CRITICAL] block_action via NATS + Integration Service routing** | Decouples action fan-out from HTTP delivery; retries survive app downtime; Socket Mode reuses same pipeline with a WS delivery leg |
| 15 | **Livekit as SFU over Jitsi / mediasoup** | Open-source, self-hostable, first-class Flutter/Dart SDK, Go server SDK for Media Service integration, active community; Jitsi is Java-heavy; mediasoup requires Node.js |
| 16 | **SFU over MCU** | SFU (Selective Forwarding Unit) forwards encoded streams without decoding — O(n) CPU; MCU mixes streams server-side — O(n²) CPU. SFU is the right model for self-hosted at community scale |
| 17 | **Coturn bundled in Docker Compose / Helm** | Self-hosters are the primary audience; most are behind NAT. Shipping Coturn as a first-party service removes the #1 setup failure for WebRTC deployments |
| 18 | **Livekit webhook → Media Service for DB sync** | Avoids polling Livekit API; webhook delivery is reliable for room lifecycle events; Media Service stays authoritative for participant state |
| 19 | **Separate control plane (Media Service) from media plane (Livekit)** | Keeps business logic (permissions, token issuing, call history) in our codebase; Livekit can be swapped or upgraded without changing client code |
| 20 | **Custom S2S federation over Matrix / ActivityPub** | Matrix requires full DAG sync and complex state resolution; ActivityPub is designed for social posts; custom S2S maps directly to the Slack Connect mental model and is far simpler to implement and operate |
| 21 | **Ed25519 over RSA for request signing** | Smaller keys (32 bytes vs 512 bytes), faster verification, no padding oracle attacks, widely supported in Go's `crypto/ed25519` stdlib |
| 22 | **TOFU (Trust On First Use) public key pinning** | Matches the mental model of SSH; avoids a PKI CA dependency; admin approval step gives humans a chance to verify before trust is fully granted |
| 23 | **Home server owns federated channel** | Single source of truth for message ordering and member management; avoids distributed consensus problems; mirrors receive events, not joint ownership |
| 24 | **Domain-based classification (internal/external) over per-user flags** | Domain policy is set once by admin and applies automatically to all future invites; per-user flags don't scale; matches how corporate email policies work |
| 25 | **Shadow `federated_users` records** | Remote users need local FKs for messages/reactions; shadow records enable standard SQL joins without cross-server DB queries on every read |

---

## 11. Scalability Targets

| Metric                         | Single-Node              | Kubernetes (scaled)         |
|--------------------------------|--------------------------|-----------------------------|
| Concurrent WebSocket users     | ~5,000                   | ~500,000                    |
| Messages/sec (sustained)       | ~2,000                   | ~200,000                    |
| Workspaces                     | 100s                     | 100,000s                    |
| Message history                | Unlimited*               | Unlimited*                  |
| File storage                   | Disk-bound               | Object storage (∞)          |
| Concurrent Livekit rooms       | ~100 (2 CPU / 4 GB node) | ~10,000 (Livekit cluster)   |
| Participants per room          | Up to 100 (SFU model)    | Up to 100 per room          |
| TURN relay throughput          | ~500 Mbps (Coturn)       | Multiple Coturn instances   |

*Subject to disk/DB capacity

**Livekit sizing rule of thumb:** 1 vCPU per ~30 simultaneous video publishers at 720p.

---

## 12. Phased Rollout

### Phase 1 — Core (MVP)
- [ ] Auth (email + password, JWT)
- [ ] Workspaces, channels (public/private), DMs
- [ ] **[CRITICAL] Member roles (owner/admin/member/guest) + permission middleware**
- [ ] **[CRITICAL] Guest channel access restrictions**
- [ ] Real-time messaging + threads
- [ ] **[CRITICAL] Cursor-paginated message API (before/after Snowflake ID)**
- [ ] **[CRITICAL] Virtual scroll message list in Flutter (ScrollablePositionedList)**
- [ ] **[CRITICAL] Block Kit renderer in Flutter (SectionBlock, ActionsBlock, ImageBlock, DividerBlock)**
- [ ] **[CRITICAL] blocks JSONB column + block schema validator in Integration Service**
- [ ] File upload (images, documents)
- [ ] Flutter app — web + Android + iOS targets
- [ ] Docker Compose deployment

### Phase 2 — Feature Parity
- [ ] **[CRITICAL] Interactive action dispatch pipeline (block_action → NATS → app endpoint)**
- [ ] **[CRITICAL] Socket Mode for apps without public endpoint**
- [ ] Emoji reactions, rich-text (Markdown), full block type coverage (InputBlock, ContextBlock)
- [ ] Message search (Meilisearch)
- [ ] Push notifications (FCM/APNS)
- [ ] Incoming webhooks + slash commands
- [ ] OAuth login (GitHub, Google)
- [ ] App manifest + OAuth app installation flow with scopes
- [ ] **Huddles** — Media Service + Livekit + Coturn deployed; channel audio rooms; Flutter huddle bar overlay
- [ ] **Video calls** — DM video calls; call invitation flow; accept / reject / timeout
- [ ] **Screen sharing** — all platforms; presenter indicator in participant strip

### Phase 3 — Enterprise & Ecosystem
- [ ] SAML SSO / OIDC + SCIM provisioning
- [ ] App Home tab + Workflow Builder (no-code automation)
- [ ] App directory / marketplace
- [ ] Kubernetes Helm chart (including Livekit cluster + Coturn DaemonSet)
- [ ] Admin dashboard (usage, audit log, call duration metrics)
- [ ] Flutter desktop builds (Windows, macOS, Linux) + store publishing
- [ ] **Call recording** — Livekit Egress API → MP4 stored in MinIO; shareable link in channel
- [ ] **Noise cancellation** — Livekit built-in Krisp noise suppression (opt-in)
- [ ] **Federation Phase 1** — Server identity + `.well-known` endpoint; Ed25519 keypair generation on boot; Federation Service scaffolding; domain policy engine (internal/external/blocked classification)
- [ ] **Federation Phase 2** — Server-to-server message delivery for federated channels; channel invite flow; admin approval UI; External user badge in Flutter
- [ ] **Federation Phase 3** — Cross-org DMs; blocklist enforcement; key rotation; federation audit log export; S2S retry dead-letter alerts

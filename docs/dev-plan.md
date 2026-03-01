# OpenSlack — Phased Development Plan

**Reading this document:**
- Each phase has a set of parallel **tracks** that can be worked simultaneously
- Tracks within a phase are independent unless a dependency is noted
- Phase N cannot begin until Phase N-1 milestones are met (exceptions noted)
- Engineer types: **BE** = Go backend, **FE** = Flutter/Dart, **Infra** = DevOps/Infrastructure
- Duration is in weeks assuming a team of **6–8 engineers**

---

## Phase 0 — Foundation
**Duration:** 2 weeks
**Goal:** Every engineer has a working local environment; CI is green; empty services start up.
**Milestone:** `docker compose up` starts all infrastructure; all services boot and return a health check.

| Track | Type | Work Items |
|-------|------|-----------|
| **0-A** Monorepo bootstrap | BE | Init repo structure (`apps/`, `services/`, `packages/`, `infra/`, `docs/`); Go workspace (`go.work`); Mage or `make` task runner; `.golangci.yml` linter config |
| **0-B** Infrastructure | Infra | Docker Compose: PostgreSQL 16, Redis 7, NATS JetStream, MinIO, Meilisearch; health-check endpoints; `.env.example` |
| **0-C** CI/CD pipeline | Infra | GitHub Actions: lint (golangci-lint + dart analyze), unit test, build Docker images, push to GHCR on merge to `main`; Renovate bot for dependency scanning |
| **0-D** Database migrations | BE | `golang-migrate` setup; initial schema: `users`, `workspaces`, `channels`, `messages` (bare columns); seed script for local dev |
| **0-E** Shared packages | BE | `packages/proto/` — Protobuf definitions for inter-service gRPC; `packages/sdk-go/` skeleton; `packages/sdk-dart/` skeleton with Dio client and Drift schema stub |
| **0-F** Flutter skeleton | FE | `apps/flutter/` — `go_router` routes (splash → login → workspace → channel); Material 3 theme (light + dark); Drift DB schema v1; `riverpod` provider tree; build flavors (dev/staging/prod) |

**All six tracks are fully parallel.**

---

## Phase 1 — Auth & Workspace Core
**Duration:** 3 weeks
**Prerequisite:** Phase 0 milestone green.
**Goal:** A user can register, log in, create a workspace, create channels, and invite members with role enforcement.
**Milestone:** End-to-end: register → login → create workspace → create channel → invite member as guest → permission check rejects guest from posting in announcement channel.

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **1-A** Auth Service | BE | Register (Argon2id), login, JWT (15 min) + refresh token rotation, Redis token allowlist, TOTP MFA (RFC 6238), OAuth stub (GitHub / Google — full impl in Phase 3), `/health` | 0-A, 0-D |
| **1-B** API Service — workspace & channel | BE | Workspace CRUD, channel CRUD (public/private), member invite, `PermissionGuard` middleware (resolves caller role, checks permission matrix, 403 on failure), cursor pagination contract (`before`/`after` Snowflake ID) | 0-A, 0-D |
| **1-C** Member roles & permissions schema | BE | Migration: `member_role ENUM`, `workspace_members`, `guest_channel_access`, `channel_members` (with `last_read_id`, `mention_count`); `workspace_domain_policies` stub | 0-D |
| **1-D** Flutter — auth screens | FE | Login screen, register screen, OAuth button stubs, TOTP MFA screen, token storage (`flutter_secure_storage`), Dio interceptor for JWT refresh, auth state `AsyncNotifier` | 0-F |
| **1-E** Flutter — workspace shell | FE | Workspace switcher, 3-pane layout (desktop/web) + bottom-nav (mobile), channel list sidebar, member list panel, empty channel state, `go_router` deep links | 0-F |

**1-C** should start day 1 (other tracks depend on it for FK integrity). **1-A** and **1-B** are parallel. **1-D** and **1-E** are parallel Flutter tracks that can mock the API until **1-A**/**1-B** are ready.

---

## Phase 2 — Real-Time Messaging
**Duration:** 5 weeks
**Prerequisite:** Phase 1 milestone green (auth tokens work, channels exist).
**Goal:** Users can send and receive messages in real time, upload files, receive push notifications, and search messages.
**Milestone:** Two browser tabs open on different accounts in the same channel — message sent in one appears instantly in the other, including file attachments. Search returns results.

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **2-A** Messaging Service | BE | WebSocket gateway (gorilla/websocket), connection registry (Redis per-pod map), NATS fan-out per channel, presence (heartbeat + Redis TTL), typing indicators, read receipts, `send_message` → INSERT → PUBLISH flow, WS reconnect with exponential backoff | 1-A, 1-B |
| **2-B** File Service | BE | MinIO bucket setup, presigned URL generation (upload + download, 15 min expiry), ClamAV async scan post-upload, libvips thumbnail pipeline for images, per-workspace quota tracking | 0-B, 1-B |
| **2-C** Notification Service | BE | NATS consumer; FCM (Android) + APNS (iOS) push dispatch; SMTP transactional email (Postfix/SES); per-channel notification level (all / mentions / nothing); DND schedule check before dispatch | 1-A, 2-A |
| **2-D** Search Service | BE | Meilisearch index setup (messages, files, users, channels); NATS consumer indexing new messages asynchronously; workspace-scoped index namespaces; basic `/search?q=` API endpoint | 0-B, 2-A |
| **2-E** Flutter — message list | FE | `MessageListNotifier` (cursor-paginated AsyncNotifier); `MessageListView` with `ScrollablePositionedList` (virtual scroll); `loadOlder()` / `loadNewer()` / `jumpToMessage()`; Drift local cache (last 500 msgs/channel, eviction); optimistic insert + rollback on failure; Markdown renderer (`flutter_markdown`) | 1-E |
| **2-F** Flutter — composer & files | FE | Message composer (text input, emoji picker, attachment button); file picker (`file_picker`); direct S3 presigned upload with progress bar; image/file preview (`cached_network_image`, `photo_view`); send-on-reconnect draft persistence in Drift | 1-E |

All six tracks run in parallel. **2-E** can mock the WebSocket until **2-A** is testable (use a local echo server). **2-F** can mock presigned URLs until **2-B** is ready.

---

## Phase 3 — Block Kit & Integration Foundation
**Duration:** 4 weeks
**Prerequisite:** Phase 2 milestone green (real-time messaging works).
**Goal:** Apps can send rich interactive messages; users can search with modifiers; OAuth social login works; reactions and threads are polished.
**Milestone:** A bot posts a Block Kit message with buttons; user clicks a button; the bot's endpoint receives the action payload (stub — full dispatch in Phase 4). Search returns filtered results using `from:alice` syntax.

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **3-A** Block Kit — server | BE | `blocks JSONB` + `app_id` migration; Integration Service: block schema registry, JSON Schema validator (depth limit, element count, `action_id` uniqueness); `apps` table + app registration API; inbound webhook validator | 2-A |
| **3-B** Block Kit — Flutter renderer | FE | `core/blocks/block_renderer.dart`; widgets: `SectionBlock`, `ActionsBlock`, `ImageBlock`, `DividerBlock`, `ContextBlock`, `HeaderBlock`; element widgets: `ButtonElement`, `StaticSelect`, `DatePicker`; `BlockActionNotifier` dispatches `block_action` WS event on tap | 2-E |
| **3-C** Integration Service — webhooks & slash cmds | BE | Outbound webhooks (HTTP POST + HMAC-SHA256 signature); slash command routing (POST to registered endpoint); bot API (REST + WS, same as client API); app manifest JSON schema | 2-A |
| **3-D** Auth Service — OAuth login | BE | GitHub OAuth 2.0 (authorization code flow); Google OAuth 2.0; token exchange + user upsert; `flutter_web_auth_2` callback handling | 1-A |
| **3-E** Messaging — reactions & thread polish | BE | Emoji reactions (add/remove/list API + WS event); `reply_count` + `participant_ids` denormalization on thread parent (updated on each reply INSERT); `@here` / `@channel` / `@user` mention expansion + WS push to mentioned users | 2-A |
| **3-F** Search — modifiers & Flutter UI | FE+BE | BE: modifier parser (`from:`, `in:`, `before:`, `after:`, `has:link`, `-term`) → Meilisearch filter/sort params; FE: global search overlay (⌘K), modifier chip UI, result type tabs (messages / files / users / channels) | 2-D |

All tracks parallel. **3-B** depends on **3-A** schema being merged (can start with a local block type stub). **3-F** backend part depends on **2-D** Meilisearch being live.

---

## Phase 4 — Interactive Apps
**Duration:** 4 weeks
**Prerequisite:** Phase 3 milestone green (Block Kit renders, Integration Service exists).
**Goal:** Apps can respond to user interactions; work without a public endpoint (Socket Mode); users can install OAuth apps with scoped permissions; client queues messages when offline.
**Milestone:** Install a demo app; it posts a Block Kit message with a button; click the button; the app updates the message in under 3 seconds. Disconnect from network; compose a message; reconnect; message delivers.

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **4-A** Action dispatch pipeline | BE | `block_action` WS event → NATS `block_actions` topic → Integration Service consumer → HTTP POST to `app.action_endpoint` (HMAC-signed) → response → `message_updated` NATS publish; `action_deliveries` audit table; 3-second timeout + client `action_ack`; retry on 5xx | 3-A, 3-C |
| **4-B** Socket Mode | BE | Integration Service: dedicated WebSocket endpoint for apps; app registers with `socket_mode=TRUE`; action/event payloads pushed over WS instead of HTTP POST; ack/nack protocol; reconnect handling | 4-A |
| **4-C** OAuth app install flow | BE | App manifest validation; authorization code flow with workspace-scoped consent screen; scope registry + enforcement middleware (e.g. `channels:read`, `chat:write`); token storage per app + workspace; app uninstall + token revocation | 3-D, 3-C |
| **4-D** Flutter — offline queue & push deep-links | FE | Drift outbox table for unsent messages; `OutboxWorker` replays on WS reconnect; optimistic message bubble (pending state → confirmed / failed); push notification deep link → open correct workspace + channel; notification preference settings UI | 2-E, 2-F |
| **4-E** Flutter — OAuth app UI | FE | App directory listing; app install consent screen (scope list); app management in workspace settings; installed apps sidebar section | 4-C |

**4-A** must ship before **4-B** (Socket Mode is the same pipeline with a WS delivery leg). **4-C**, **4-D**, **4-E** are parallel with **4-A**/**4-B**.

---

## Phase 5 — Voice & Video
**Duration:** 5 weeks
**Prerequisite:** Phase 4 milestone green. Livekit and Coturn infrastructure must be deployed before Flutter tracks begin E2E testing.
**Goal:** Users can start audio huddles in channels, make video calls in DMs, and share their screen on all platforms.
**Milestone:** Alice starts a huddle in a channel; Bob joins from his phone; both hear each other; Carol sees the active huddle indicator and joins; Alice shares her screen; Bob and Carol see it.

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **5-A** Livekit + Coturn deployment | Infra | Docker Compose: `livekit` (SFU) + `coturn` (TURN/STUN); Helm chart entries; env var documentation (`LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`, `COTURN_STATIC_SECRET`); port exposure (UDP 40000–50000, TCP/UDP 3478, TCP 5349); smoke test script | 0-B |
| **5-B** Media Service | BE | `rooms` + `room_participants` + `turn_credentials` migrations; room lifecycle API (`POST /rooms`, `POST /rooms/:id/join`, `DELETE /rooms/:id`); Livekit room create/end via Livekit Go SDK; Ed25519 JWT token issuing (participant grants); ephemeral TURN credential generation (HMAC-SHA1); Livekit webhook handler (room_started, participant_joined, participant_left, track_published, room_finished) → DB sync + NATS publish | 1-B, 5-A |
| **5-C** NATS room events → WS fan-out | BE | Messaging Service: consume `room.*` NATS events; fan-out `huddle_started`, `huddle_ended`, `participant_joined`, `participant_left`, `screen_share_started` WS events to channel members; active huddle state in Redis per channel | 2-A, 5-B |
| **5-D** Flutter — Huddle | FE | `HuddleNotifier` (join/leave Livekit room, `livekit_client` SDK); `HuddleBarWidget` (floating participant strip at bottom of channel view, mute button, leave button); active huddle indicator in channel sidebar; microphone permission flow (`permission_handler`) | 2-E, 5-A |
| **5-E** Flutter — Video Call | FE | `CallNotifier` state machine (idle → ringing → connecting → active → ended); `CallScreen` (video grid, mute, camera toggle, hang-up); `IncomingCallOverlay` (accept / reject, 30 s auto-decline); camera permission flow; DM call initiation button | 2-E, 5-A |
| **5-F** Flutter — Screen Share | FE | `livekit_client` `setScreenShareEnabled()` per platform; presenter badge in participant strip; platform-specific capture: ReplayKit (iOS), MediaProjection (Android), `getDisplayMedia` (desktop/web); permission request per platform | 5-D, 5-E |

**5-A** runs first (week 1 of this phase). **5-B** and **5-C** parallel (week 1–3). **5-D**, **5-E**, **5-F** parallel Flutter tracks (week 2–5, can dev against local Livekit from day 1 of **5-A**).

---

## Phase 6 — Enterprise
**Duration:** 4 weeks
**Prerequisite:** Phase 5 milestone green. Can be partially parallelised with Phase 5 if team size allows.
**Goal:** Enterprise deployments can enforce SSO, provision users via SCIM, set retention policies, export data, and deploy on Kubernetes with full Helm chart.
**Milestone:** Admin configures Okta SAML SSO; new employee signs in via Okta; account auto-provisioned; message retention policy deletes messages older than 90 days; full workspace export ZIP downloaded.

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **6-A** SAML 2.0 + OIDC | BE | Auth Service: SAML 2.0 SP (crewjam/saml library); OIDC authorization code flow; IdP metadata import; SP-initiated + IdP-initiated SSO; session mapping; `sso_providers` table | 1-A |
| **6-B** SCIM 2.0 provisioning | BE | Auth Service + API: SCIM 2.0 `/Users` + `/Groups` endpoints (POST create, PATCH update, DELETE deprovision); attribute mapping (email → workspace_members); group sync to user_groups; bearer token auth for SCIM client | 6-A |
| **6-C** Retention + data export | BE | API Service: `retention_policies` table (per-workspace, per-channel TTL); scheduled hard-delete job (pg_cron or separate Go cron); data export API (`POST /workspaces/:id/export` → async job → ZIP of JSON + files → MinIO presigned download link) | 1-B |
| **6-D** Kubernetes Helm chart | Infra | Helm chart for all services (auth, api, messaging, notification, search, file, integration, media, federation); HPA configs (messaging: WS count, api: RPS, notification: queue depth); StatefulSets for postgres, nats, meilisearch, minio, livekit, coturn; Secrets / ConfigMap templates; `values.yaml` with sensible defaults | 5-A |
| **6-E** Admin dashboard — Flutter | FE | Admin-only settings routes (guarded by `role == owner || admin`); workspace analytics (DAU/WAU, message volume, storage used); member management (role change, deactivate, remove); audit log viewer (filterable, paginated); retention policy configuration UI | 1-E, 1-C |
| **6-F** Analytics aggregation | BE | Scheduled Go jobs: daily rollup of `message_count`, `active_users`, `storage_bytes` per workspace into `workspace_analytics` table; API endpoints consumed by admin dashboard | 1-B |

All six tracks run in parallel. **6-B** depends on **6-A** (SCIM users must map to SSO identities). **6-E** depends on admin route guard being in place from Phase 1 (**1-C**).

---

## Phase 7 — Cross-Org Federation
**Duration:** 6 weeks (split into two sub-phases)
**Prerequisite:** Phase 6 milestone green (domain policy schema exists; workspace settings are extensible).
**Goal:** An admin on Server-A can invite Server-B to a shared channel; messages flow between servers; external users are classified by email domain and shown with an External badge.
**Milestone:** Admin on `chat.acme.com` invites `chat.partner.com` to `#shared-project`; admin on partner approves; Bob on partner sees the channel; Alice on acme posts; Bob sees it in under 2 seconds with an External badge on Alice's name.

### Phase 7a — Federation Foundation (Weeks 1–2)

These must complete before 7b tracks begin.

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **7a-A** Server identity & signing | BE | `server_identity` migration; Ed25519 keypair generation on first boot (private key encrypted via env secret); `/.well-known/openslack` HTTP endpoint; `signing/signer.go` + `signing/verifier.go`; Nginx routing for `.well-known` + `/federation/v1/` | 0-D |
| **7a-B** Federation Service skeleton | BE | Go service scaffold; TOFU public key fetch + pin on first contact; `federation_servers` table CRUD; inbound request verification middleware (signature, Date ±5 min, Digest); rate limiting per remote server (Redis); blocklist check (status = blocked → 403) | 7a-A |

### Phase 7b — Federation Features (Weeks 3–6, all parallel)

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **7b-A** Domain policy engine | BE | `workspace_domain_policies` CRUD API; classification logic (walk policies by sort_order, apply catch-all `*`); `federated_users` shadow records (insert on first contact, sync on profile change); classify remote user at join time + re-check on every API call; `external_can_dm_internal` + `external_can_create_channels` enforcement | 7a-B |
| **7b-B** S2S message delivery | BE | NATS consumer on `federation.outbound.*`; HTTP POST to remote `/federation/v1/messages` (signed); inbound `/federation/v1/messages` endpoint (verify → INSERT federated message → PUBLISH to NATS → WS fan-out); exponential backoff retry (max 24 h); dead-letter alert; `federation_audit` table | 7a-B, 2-A |
| **7b-C** Federated channel model + invite flow | BE | `federated_channels` + `channel_federation_servers` migrations; channel invite API (`POST /federation/channels/:id/invite`); outbound invite HTTP POST to remote; inbound `/federation/v1/invites` endpoint; admin approval flow (WS push + approve/reject API); channel mirror creation on remote on accept | 7a-B, 1-B |
| **7b-D** Cross-org DM invite | BE | DM initiation with global handle (`bob@chat.partner.com`); handle resolution (parse domain → `.well-known` fetch); `/federation/v1/dm/invite` outbound; inbound DM invite → WS push to callee; accept → create local DM conversation with shadow `federated_users` record | 7a-B, 7b-A |
| **7b-E** Flutter — federation UI | FE | `🔗` indicator on federated channels in sidebar (server count tooltip); amber `[External]` badge on messages from remote users; `User` model: `homeServer`, `classification` fields; `Channel` model: `isFederated`, `federatedServers` fields; DM composer: `@user@server.com` global handle input + resolution; Admin Settings → Federation panel (enable/disable, domain policies CRUD, server allow/block list, pending approvals) | 7b-A, 7b-B, 7b-C |

**7b-A**, **7b-B**, **7b-C**, **7b-D** are parallel BE tracks. **7b-E** can begin UI scaffolding (mock data) from week 3 and wire to real APIs as BE tracks stabilise.

---

## Phase 8 — Hardening & Scale
**Duration:** 4 weeks
**Prerequisite:** Phase 7 milestone green. This phase can partially overlap with Phase 7 for Infra tracks.
**Goal:** The platform handles production load; desktop apps are published; call recording works; the message table is partitioned.
**Milestone:** Load test: 10,000 concurrent WS users; p99 message delivery < 200 ms. macOS app submitted to App Store. Call recording link appears in channel after call ends.

| Track | Type | Work Items | Depends On |
|-------|------|-----------|------------|
| **8-A** PostgreSQL partitioning | BE | Range-partition `messages` table by `created_at` (monthly partitions); automated partition creation job; rewrite message LIST queries to use partition pruning; verify cursor pagination still works across partition boundaries | 2-A |
| **8-B** CDN for file delivery | Infra | CloudFront / Cloudflare in front of MinIO (or S3) for presigned URL delivery; cache headers for thumbnails; URL migration: existing presigned URLs remain valid during cutover | 2-B |
| **8-C** Livekit Redis clustering | Infra | Multi-node Livekit deployment (Redis pub/sub for cross-node participant events); Helm chart update (Livekit as Deployment with Redis sidecar or external Redis cluster); UDP load balancing (AWS NLB / IPVS); Coturn DaemonSet for multi-zone | 5-A |
| **8-D** Flutter desktop publishing | FE | Windows: MSIX packaging + Microsoft Store submission; macOS: Notarized DMG + Mac App Store submission; Linux: AppImage + Flatpak; CI pipeline additions for desktop builds; update `pubspec.yaml` version + `CHANGELOG.md` | 0-F |
| **8-E** Call recording | BE | Livekit Egress API integration in Media Service; `POST /rooms/:id/recording/start`; Egress job → MP4 in MinIO; recording ready → NATS event → Messaging Service posts a system message in channel with presigned download link; `room_recordings` table | 5-B |
| **8-F** Performance & graceful shutdown | BE | SIGTERM handler in Messaging Service (drain WS connections, stop accepting new ones, wait up to 30 s, exit); connection migration on K8s rolling deploy; load test (k6 script: 10k WS users, sustained 2k msg/s); profiling (pprof) and fix top bottlenecks | 2-A |

All tracks fully parallel. **8-A** requires a maintenance window for the partition migration. **8-C** requires coordinating Livekit downtime (or rolling update) with active calls.

---

## Dependency Graph Summary

```
Phase 0 ──────────────────────────────────────────────────────────────────────┐
   (Foundation)                                                                │
         │                                                                     │
Phase 1 ◄─────────────────────────────────────────────────────────────────────┘
   (Auth & Workspace Core)
         │
Phase 2 ◄──────────────────────────────────────────────────────────────────────
   (Real-time Messaging)
         │
Phase 3 ◄──────────────────────────────────────────────────────────────────────
   (Block Kit & Integration Foundation)
         │
Phase 4 ◄──────────────────────────────────────────────────────────────────────
   (Interactive Apps)
         │
         ├──────► Phase 5 (Voice & Video) ──────────────────────────────────┐
         │                                                                   │
         └──────► Phase 6 (Enterprise)    ◄──────────────────────────────────┘
                        │
                  Phase 7a (Federation Foundation)
                        │
                  Phase 7b (Federation Features — all parallel)
                        │
                  Phase 8 (Hardening & Scale)
```

> Phase 5 and Phase 6 can run in **parallel** if the team is large enough (8+ engineers).
> Phase 7a is a short 2-week gate; Phase 7b opens 4 parallel tracks.

---

## Team Allocation by Phase

| Phase | BE Engineers | FE Engineers | Infra | Total |
|-------|-------------|-------------|-------|-------|
| 0     | 2           | 1           | 1     | 4     |
| 1     | 2           | 2           | 0     | 4     |
| 2     | 4           | 2           | 0     | 6     |
| 3     | 3 + 1 FE    | 1 + 1 FE+BE | 0     | 6     |
| 4     | 3           | 2           | 0     | 5     |
| 5     | 2           | 3           | 1     | 6     |
| 6     | 4           | 1           | 1     | 6     |
| 7a    | 2           | 0           | 0     | 2     |
| 7b    | 4           | 1           | 0     | 5     |
| 8     | 2           | 1           | 2     | 5     |

---

## Testing Strategy Per Phase

| Phase | Unit Tests | Integration Tests | E2E Tests |
|-------|-----------|------------------|-----------|
| 0     | Go: service boot; Flutter: widget smoke | Infra: all containers healthy | — |
| 1     | Auth: token issue/verify; Permission: role matrix | Register → login → create workspace flow | — |
| 2     | WS: message fan-out; Drift: cache eviction | Send message → appears on second client | File upload → download |
| 3     | Block validator: invalid block rejected; Search: modifier parsing | Bot posts Block Kit → Flutter renders it | Search `from:alice` returns correct messages |
| 4     | Action dispatch: timeout, retry; Scope: unauthorized scope blocked | Button click → app endpoint receives POST | Offline: compose → reconnect → delivered |
| 5     | Room lifecycle: create/join/leave/end | Huddle: two clients hear each other | Screen share: all platforms |
| 6     | SAML: assertion parsing; Retention: expired messages deleted | SCIM provision → user appears in workspace | Export ZIP: all messages + files present |
| 7a    | Signature: valid/invalid/replay/tampered | .well-known served correctly | — |
| 7b    | Domain policy: classification ordering; Shadow user: sync | Cross-server message delivery | Channel invite → both servers see shared channel |
| 8     | Partition: cursor pagination crosses boundary | Load test: 10k WS, p99 < 200 ms | Desktop: all 3 platforms install and run |

---

## Definition of Done (per track)

A track is **done** when:
1. All unit tests pass (`go test ./...` or `flutter test`)
2. Integration tests for the track's feature pass
3. PR reviewed and approved by at least one other engineer
4. Docker image builds and passes health check in CI
5. API changes documented in OpenAPI spec (`docs/api/openapi.yaml`)
6. Dart SDK updated if the track exposed a new API endpoint
7. Any new database migration is idempotent and includes a rollback

---

## Milestone Summary

| Phase | Weeks | Key Deliverable |
|-------|-------|----------------|
| 0 | 1–2 | Monorepo boots; all infra up; CI green |
| 1 | 3–5 | Register → login → workspace → channel with roles |
| 2 | 6–10 | Real-time messaging, file upload, search, push notifications |
| 3 | 11–14 | Block Kit rendering, OAuth login, reactions, search modifiers |
| 4 | 15–18 | Interactive app actions, Socket Mode, offline queue |
| 5 | 19–23 | Huddles, video calls, screen sharing |
| 6 | 19–22* | SSO, SCIM, retention, Helm chart, admin dashboard |
| 7a | 24–25 | Server identity, signing, .well-known endpoint |
| 7b | 26–29 | Federated channels, domain policy, cross-org DMs, External badge |
| 8 | 30–33 | Partitioned DB, CDN, desktop apps, call recording, load-tested |

*Phase 5 and 6 overlap if team ≥ 8 engineers.

**Total: ~33 weeks** (≈ 8 months) for a team of 6–8 engineers to reach full feature parity including federation.

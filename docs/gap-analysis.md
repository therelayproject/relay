# Gap Analysis: OpenSlack Architecture vs. Slack

**Severity key**
| Level    | Meaning                                              |
|----------|------------------------------------------------------|
| CRITICAL | Blocks core usability — users will notice immediately|
| HIGH     | Major feature parity gap — frequent user complaint   |
| MEDIUM   | Meaningful quality-of-life / ecosystem gap           |
| LOW      | Enterprise / edge case — addressable later           |

---

## 1. Messaging & Communication

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Block Kit / Rich message components** | CRITICAL | Structured interactive messages: buttons, dropdowns, date pickers, modals, overflow menus, images with alt text — rendered natively by client | Only plain Markdown. No interactive component model defined. No message payload schema beyond `content: TEXT`. |
| **Message scheduling** | HIGH | Users can schedule a message to send at a future time | Not mentioned anywhere in schema or services. |
| **Message reminders** | HIGH | "Remind me about this" on any message (Slackbot DM at set time) | No reminder model in schema. |
| **Pinned messages** | HIGH | Per-channel pinned messages list (max 100) | No `pins` table or API endpoint. |
| **Saved items / Bookmarks** | HIGH | Users can bookmark any message or file; accessible globally | No `saved_items` table. |
| **@here / @channel / @everyone** | HIGH | Workspace-wide broadcast mentions with permission controls and warnings | Mention system not modeled. No distinction between mention scopes. |
| **User groups (@team mentions)** | HIGH | Named groups of users (`@design`, `@oncall`) that can be mentioned and managed | No `user_groups` or `group_members` tables. |
| **Announcement-only channels** | MEDIUM | Channel mode where only admins can post | No `channel_type` or `posting_permissions` field in schema. |
| **Channel sections (sidebar grouping)** | MEDIUM | Users organize channels into named collapsible sections in sidebar | Pure client-side UX feature; no backend model defined. |
| **Link unfurling** | HIGH | Pasting a URL auto-fetches OG metadata and renders a rich preview card | No unfurl service or OG metadata fetch defined. Integration service does not include unfurling. |
| **Message forwarding / sharing to channel** | MEDIUM | A message can be shared into another channel as a new message with attribution | Not modeled. |
| **Shared channels (Slack Connect)** | HIGH | A channel can include members from two different workspaces | Architecture marks federation as "future". The data model has no cross-workspace channel concept. |
| **Draft messages** | MEDIUM | Unsent drafts persist per channel, even across devices | No `drafts` table or sync mechanism defined. |
| **Custom status + expiry** | MEDIUM | User can set an emoji + text status with optional auto-clear time | Presence service only tracks online/offline heartbeat; no status text/emoji model. |
| **Message reactions — detail gaps** | MEDIUM | Full reactor list, animated emoji, custom workspace emoji | `reactions` table exists but no custom emoji model or animator defined. |
| **Threaded reply count + participants** | MEDIUM | Thread metadata shows participant avatars and unread count inline in channel | Thread model has `parent_id` but no `reply_count` or `participant_ids` denormalization. |

---

## 2. Rich Media & Files

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Clips (async video/audio messages)** | HIGH | Record a short video or audio clip directly in the composer; stored and played inline | File service handles uploads but no clip recording flow, duration limits, or playback metadata. |
| **Canvas (collaborative docs)** | HIGH | Rich text + media documents embedded in channels and DMs; real-time collaborative editing | Completely absent. Would require an OT/CRDT engine (e.g., Yjs), a Canvas service, and new DB tables. |
| **File preview rendering** | HIGH | In-app preview for PDF, Word, Excel, PowerPoint, code files, images, and video | File service handles upload/download via presigned URL but has no preview/thumbnail pipeline for documents. Only images mentioned. |
| **Google Drive / OneDrive / Dropbox integration** | MEDIUM | First-class file browsing and sharing from cloud storage providers | Integration service is webhook/slash-command focused; no cloud file provider connector model. |
| **Video/image inline player** | MEDIUM | Video files play inline in the message stream; images are lightboxed | Flutter client uses `cached_network_image`; no inline video player or document renderer mentioned. |
| **File starring** | LOW | Users can star individual files | Not modeled. |
| **Storage quota enforcement (per workspace)** | MEDIUM | Per-plan file storage cap with visible usage meter | File service mentions quotas but no enforcement logic, usage tracking column, or alert trigger is defined. |

---

## 3. Search

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Search modifiers / operators** | HIGH | `from:user`, `in:channel`, `before:YYYY-MM-DD`, `after:`, `has:link`, `has:reaction:emoji`, `is:starred`, `-term` (exclusion) | Meilisearch supports filters but no query DSL or modifier parser is defined. Users would get basic full-text only. |
| **Search within files (content)** | HIGH | Searches inside document content (PDF text, docx body) | Only file metadata is indexed. No document text extraction pipeline (e.g., Apache Tika). |
| **Search within threads** | MEDIUM | Thread replies are independently searchable with context of parent | Thread indexing strategy not defined. |
| **Relevance tuning / ML ranking** | LOW | Slack uses behavioral signals (recency, sender relationship, channel activity) to rank results | Meilisearch provides BM25 ranking only. No personalization signals. Acceptable for open-source. |
| **Recent searches & saved searches** | LOW | Search history stored per user | Not modeled. |

---

## 4. Notifications & Alerts

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Keyword / highlight word notifications** | HIGH | Users define words (e.g., "incident", their name) that trigger notification regardless of direct mention | No `notification_keywords` table or matching logic in notification service. |
| **Per-channel notification overrides** | HIGH | Each channel can have its own notification level independent of workspace default | Notification service has DND preferences but no per-channel override model. |
| **Notification schedule (quiet hours per day/timezone)** | HIGH | Define exact hours and days when notifications are muted per workspace | Mentioned as "DND schedules" but the model and timezone handling are not designed. |
| **Custom notification sounds** | LOW | Per-device custom sound for different notification types | No sound preference model. |
| **Notification batching / digest** | MEDIUM | Mobile notifications grouped when many arrive in a burst | No batching logic in notification service. Every message triggers individual push. |
| **Email digest (daily/weekly)** | MEDIUM | Sends a digest of missed messages for inactive users | SMTP is set up for transactional email only. No digest generation job. |

---

## 5. Administration & Workspace Management

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Member roles (guest / member / admin / owner)** | CRITICAL | Four distinct roles with granular permission sets. Single-channel guests and multi-channel guests have restricted access. | `members` table exists but no `role` or `guest_channels` constraint. No permission matrix defined. |
| **SCIM provisioning** | HIGH | Automated user/group provisioning and deprovisioning via identity providers (Okta, Azure AD) | Auth service handles SSO but SCIM 2.0 endpoints (POST /Users, PATCH /Users, DELETE /Users) are absent. |
| **Message retention policies** | HIGH | Per-workspace and per-channel configurable retention (e.g., delete messages older than 90 days). Retention runner job. | `deleted_at` soft-delete exists but no retention policy table, scheduler, or hard-delete job. |
| **Data export (eDiscovery / compliance)** | HIGH | Full workspace data export (all channels, DMs, files) in JSON/ZIP. Enterprise: DLP integration. | Not defined. Audit log table exists but bulk export API is absent. |
| **Admin analytics dashboard** | MEDIUM | Message activity, DAU/WAU, most active channels, file usage, member growth | No analytics service, aggregation jobs, or reporting DB layer defined. |
| **Domain management (auto-join)** | MEDIUM | Users with email matching a verified domain auto-join the workspace | No domain verification table or auto-join logic. |
| **Enterprise Grid (multi-org)** | LOW | Org-level layer above workspaces for large enterprises; shared channels between workspaces | Architecture has single workspace layer. No `organizations` table. |
| **Workspace discovery** | LOW | Users can find and request to join public workspaces on the same domain | No discovery index or join-request workflow. |

---

## 6. Apps & Integration Platform

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Block Kit (interactive components for apps)** | CRITICAL | Apps send structured payloads (Block Kit) with buttons, select menus, overflow menus, date pickers, plain/rich text, images, and modals. The client renders them natively. | Integration service handles webhooks and slash commands but there is no component model. Apps can only send plain text. This eliminates almost all real-world Slack apps. |
| **App Home tab** | HIGH | Each installed app gets a dedicated Home tab in the sidebar — a persistent canvas the app controls | Not modeled. No `app_home` view concept. |
| **Workflow Builder (no-code automation)** | HIGH | Visual drag-and-drop automation: triggers (message posted, reaction added, schedule) → steps (send message, create channel, call webhook) | Completely absent. Would require a workflow engine service, step registry, and UI builder. |
| **Interactive message payloads (action callbacks)** | HIGH | When a user clicks a button in a Block Kit message, Slack sends an `action` POST to the app's endpoint | Integration service has no action-dispatch mechanism. Slash commands are one-way only. |
| **Event subscriptions API** | HIGH | Apps subscribe to specific event types (message.channels, reaction_added, member_joined_channel, etc.) and receive HTTP POST | Webhooks are outbound only. No typed event subscription registry with per-app scopes. |
| **Socket Mode** | MEDIUM | Apps behind firewalls can receive events over a WebSocket instead of exposing an HTTP endpoint | Not mentioned. Important for self-hosted integrations. |
| **Slash command autocomplete** | MEDIUM | Slash commands surface a hint/description in the composer UI | No command registry or manifest-level description field for client rendering. |
| **App shortcuts (global + message)** | MEDIUM | ⚡ shortcuts available globally in composer and on individual messages | Not modeled. |
| **Scheduled messages (app-triggered)** | MEDIUM | Apps can schedule messages via API for future delivery | No scheduled message queue or `send_at` field in message schema. |
| **App marketplace** | LOW | Discoverable app directory with install flow, permissions review | No marketplace service or app discovery layer. |

---

## 7. Real-Time & Voice/Video

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Huddles (lightweight persistent audio)** | HIGH | One-click "always on" audio rooms per channel — low latency, no scheduling required; screen sharing, emoji reactions live in huddle | WebRTC via Livekit is mentioned only in Phase 3 and is not architecturally designed. No huddle concept (room lifecycle, mute state, participant list) defined. |
| **Screen sharing in calls** | HIGH | Full screen sharing during video calls and huddles | No signaling protocol or screen-capture flow defined for Flutter (requires platform-specific plugins). |
| **Clips recording in-client** | MEDIUM | Record video/audio asynchronously without scheduling a call | Not modeled (related to Clips gap in §2). |
| **Call recording** | LOW | Enterprise: record and transcribe calls | No recording pipeline. |
| **WebRTC TURN/STUN server** | HIGH | Slack manages global TURN infrastructure for NAT traversal | Architecture delegates to Livekit but no TURN/STUN deployment is defined for self-hosters. |

---

## 8. Security & Compliance

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Data residency** | HIGH | Enterprise customers choose data region (US, EU, JP) — data never leaves region | No data residency zones, multi-region DB routing, or tenant-to-region mapping defined. |
| **Message encryption at rest (E2EE option)** | MEDIUM | Enterprise Key Management (EKM): customer holds encryption keys; Slack cannot read messages | Messages stored as plaintext TEXT in PostgreSQL. No field-level encryption or key management layer. |
| **Data Loss Prevention (DLP)** | MEDIUM | Integration with DLP tools (Nightfall, etc.); messages scanned before delivery | No content inspection hook in message delivery pipeline. |
| **SOC 2 / ISO 27001 / HIPAA controls** | MEDIUM | Slack holds certifications; requires specific audit logging, access controls, and data handling | Audit log table defined but no structured event taxonomy, tamper-evidence (immutability), or export format for auditors. |
| **Session management (device list)** | MEDIUM | Users can see and revoke active sessions per device | No `sessions` or `devices` table; only Redis token store. No revocation UI or API. |
| **IP allow-listing (workspace restriction)** | LOW | Enterprise: restrict access to specific IP ranges | No IP restriction middleware or admin config defined. |
| **Token scopes for apps** | HIGH | Fine-grained OAuth scopes (e.g., `channels:read`, `chat:write`, `files:write`) enforced server-side | Auth service has OAuth 2.0 but no scope definition, enforcement middleware, or scope registry for third-party apps. |

---

## 9. Flutter Client — Platform Gaps

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **Virtual / infinite scroll for message history** | CRITICAL | Message list renders only a window of ~50 messages; older messages load on scroll (bi-directional pagination) | Drift caches messages locally but no bi-directional cursor pagination strategy or virtual list widget is defined. Rendering 10k+ messages without this will crash Flutter. |
| **Optimistic UI updates** | HIGH | Messages appear instantly in the UI before server ACK; rollback on failure | WebSocket ACK flow is defined but no optimistic insert → confirm/rollback pattern in Riverpod layer. |
| **Offline message composition** | HIGH | Users can compose messages while offline; queued and sent on reconnect | Drift is noted for caching but no outbound message queue model or send-on-reconnect logic is designed. |
| **Keyboard shortcuts (desktop)** | MEDIUM | Comprehensive shortcut map: `⌘K` quick switcher, `⌘/` search, `Alt+↑↓` channel navigation, `⌘⇧M` mentions | Mentioned in platform table but no shortcut registry or `Intent`/`Shortcut` widget tree defined. |
| **Quick switcher (⌘K)** | HIGH | Fuzzy-search across all channels, DMs, and users with keyboard navigation | No quick-switcher component or client-side index defined. |
| **Unread / mention badges + jump-to-unread** | HIGH | Sidebar badges per channel, global unread count, "Jump to oldest unread" banner | No `unread_count` or `mention_count` tracking model on the server or client. |
| **Accessibility (a11y)** | MEDIUM | WCAG 2.1 AA — screen reader support, keyboard navigation, color contrast | Flutter has `Semantics` widget support but no a11y strategy or audit defined. |
| **Theming (custom workspace colors)** | LOW | Workspace owners set a custom sidebar color scheme | No workspace-level theme fields in schema. Material 3 theming is static. |
| **Native spell check / autocorrect** | LOW | Platform spell-check integrated into composer | Flutter `TextField` supports this natively on most platforms but behavior is untested/unspecified. |

---

## 10. Infrastructure & Operations Gaps

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **CDN for file delivery** | HIGH | Files served through Cloudflare/Fastly CDN — low latency globally | Presigned S3 URLs serve files directly from MinIO/S3. No CDN layer defined. |
| **Message fanout at scale (10M+ users)** | HIGH | Slack uses a layered pub/sub: per-channel topics + user-level delivery with connection affinity | NATS fan-out is correct at moderate scale. At very large scale (100k+ members per channel) NATS fan-out becomes a bottleneck. No sparse subscription or gateway-level filtering is designed. |
| **Presence at scale** | HIGH | Presence is aggregated and throttled — not every connect/disconnect broadcasts to all subscribers | Redis TTL heartbeat works at small scale. At tens of thousands of users per workspace, presence events flood subscribers. No presence aggregation or batch-broadcast is designed. |
| **Database sharding / partitioning** | MEDIUM | Slack shards message storage by workspace/channel | `messages` table will grow unboundedly. No partitioning strategy (e.g., PostgreSQL range partitioning by `created_at`) defined. |
| **Message archive (cold storage)** | MEDIUM | Messages older than retention threshold moved to cold storage (S3 Glacier / BigQuery) | No tiered storage or archive job defined. All messages stay in hot PostgreSQL forever. |
| **Backup & disaster recovery** | MEDIUM | Point-in-time recovery, cross-region replication, tested DR runbooks | Patroni mentioned for HA but no PITR schedule, backup retention policy, or DR process defined. |
| **Rate limiting per app/bot** | MEDIUM | Slack enforces Tier 1–4 API rate limits per app token | Rate limiting is Redis sliding window per user/IP. No per-app token rate tier system. |
| **WebSocket connection limits per pod** | MEDIUM | Slack uses connection draining on deploy, gradual pod shutdown | No graceful shutdown, SIGTERM handler, or connection migration strategy defined in messaging service. |
| **Observability (SLOs / alerting)** | MEDIUM | Slack has defined SLOs (p99 latency, error rate) with PagerDuty-level alerting | Grafana + Loki + OpenTelemetry listed in infra table but no SLO definitions, alert rules, or runbooks. |

---

## 11. Developer Experience Gaps

| Gap | Severity | Slack Has | Our Architecture |
|-----|----------|-----------|-----------------|
| **OpenAPI / Swagger docs** | HIGH | Full API reference docs with try-it-now | OpenAPI spec listed in `docs/api/` but not defined. No code-gen pipeline from spec. |
| **SDK completeness** | MEDIUM | Official SDKs in Python, JavaScript, Java, Go, .NET with Bolt framework | Only Dart and Go SDKs defined. No Bolt-equivalent event framework for app developers. |
| **Local development tunnel (like ngrok)** | MEDIUM | Apps need public HTTPS endpoint to receive webhooks during development | No dev tunnel helper or Socket Mode alternative for local testing. |
| **Postman / Bruno collection** | LOW | Slack publishes Postman collections for all APIs | Not mentioned. |

---

## Summary — Gap Count by Severity

| Severity | Count | Recommendation |
|----------|-------|----------------|
| CRITICAL | 4     | Must be resolved before any public launch |
| HIGH     | 28    | Required for meaningful Slack parity (Phase 1–2) |
| MEDIUM   | 22    | Quality and ecosystem gaps (Phase 2–3) |
| LOW      | 8     | Enterprise polish (Phase 3+) |
| **Total**| **62**| |

---

## Top 5 Architecture Decisions Needed Now

### 1. Define a Block Kit equivalent (CRITICAL)
The entire integration ecosystem depends on it. Design a `blocks` JSON column in the `messages` table and a Flutter widget tree that renders block types (`section`, `actions`, `input`, `image`, `divider`, `context`). Without this, no real app can be built on top of OpenSlack.

### 2. Model member roles and permissions properly (CRITICAL)
Add `role ENUM('owner','admin','member','guest')` to `workspace_members`. Define a `guest_channel_access` table. Add a permission matrix service/middleware. This affects nearly every API endpoint.

### 3. Design virtual scrolling + cursor pagination (CRITICAL)
The Flutter message list must use bi-directional cursor pagination (before/after Snowflake ID) and a `SliverList` or `ListView.builder` pattern. Without this, channels with long history are unusable.

### 4. Add a notification keyword model (HIGH)
Add `notification_keywords (user_id, workspace_id, keyword, match_whole_word)` table. Add a keyword scanner step in the messaging fan-out pipeline before notification dispatch.

### 5. Design the Huddle / WebRTC service properly (HIGH)
Define a `rooms` table, room lifecycle (create, join, leave, end), participant state, and Livekit integration in detail. Include TURN/STUN deployment in the Docker Compose and Helm chart. This cannot stay as a one-liner in Phase 3.

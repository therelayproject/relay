# Relay.gg — Pre-Launch Plan
## What to do before the Reddit post

**The goal:** When someone reads the Reddit post and clicks the GitHub link,
they can understand the project in 60 seconds, run it locally in under 5 minutes,
and find a task that matches their skill level immediately.

Everything below is ordered. Do not post on Reddit until all steps are done.

---

## Step 1 — GitHub Repository & Legal
*Do this first. Everything else builds on it.*

- [ ] Create GitHub organisation: `relay-gg`
- [ ] Create public repo: `therelayproject/relay`
- [ ] Add `LICENSE` — **MIT** (maximises adoption; contributors won't negotiate Apache vs GPL)
- [ ] Add `CODE_OF_CONDUCT.md` — use the standard [Contributor Covenant v2.1](https://www.contributor-covenant.org/)
- [ ] Set branch protection on `main`:
  - Require PR with at least 1 approval
  - Require CI to pass before merge
  - No direct push (including from maintainers)
- [ ] Add `CODEOWNERS` file (you own `*` initially)

**Output:** A repo that looks professionally maintained before a single line of product code is written.

---

## Step 2 — README.md (the most important file)
*This is what every Redditor sees first. Get it right.*

Structure:
```
1. One-line pitch          "Open-source, self-hosted Slack — with federation."
2. Why it exists           3 bullet points (no vendor lock-in, self-host, cross-org)
3. Architecture diagram    ASCII or image export of the high-level diagram from architecture.md
4. Feature status table    What's built ✓, in progress 🚧, planned 📋
5. Quickstart              docker compose up and you're chatting in < 5 min
6. Tech stack              Go · Flutter · PostgreSQL · NATS · Livekit
7. Contributing            One paragraph + link to CONTRIBUTING.md
8. Community               Discord link + GitHub Discussions link
9. Roadmap                 Link to dev-plan.md
10. License                MIT badge
```

Rules for the README:
- No walls of text — every section fits on one screen
- The quickstart must actually work before you post
- Include a screenshot or demo GIF (even a simple terminal showing services starting)
- Feature status table manages expectations — don't claim things that aren't built

---

## Step 3 — CONTRIBUTING.md
*Converts curious visitors into actual contributors.*

Sections:
```
1. Ways to contribute      (code, docs, testing, design, triaging issues)
2. Project structure       (monorepo map — link to architecture.md)
3. Local dev setup         (prerequisites, clone, docker compose up, run tests)
4. Finding something to work on
                           - Filter issues by label
                           - good first issue  → < 4 hours of work
                           - help wanted       → medium complexity
                           - phase/0, phase/1  → current sprint
5. PR process              (fork, branch naming, commit style, PR template)
6. Code style              (Go: gofmt + golangci-lint; Dart: dart format + flutter analyze)
7. Commit message format   (Conventional Commits: feat:, fix:, chore:, docs:)
8. Getting help            (Discord #contributors channel, GitHub Discussions)
```

---

## Step 4 — Phase 0 Scaffolding (actual code)
*Give contributors something real to land on.*

These must all be committed and working before the post.
Each sub-item maps directly to a Phase 0 track from the dev plan.

### 4a — Monorepo structure
```
relay/
├── apps/flutter/          # flutter create .
├── services/
│   ├── auth/              # go mod init + main.go + /health endpoint
│   ├── api/               # go mod init + main.go + /health endpoint
│   ├── messaging/         # go mod init + main.go + /health endpoint
│   ├── notification/      # go mod init + main.go + /health endpoint
│   ├── search/            # go mod init + main.go + /health endpoint
│   ├── file/              # go mod init + main.go + /health endpoint
│   ├── integration/       # go mod init + main.go + /health endpoint
│   ├── media/             # go mod init + main.go + /health endpoint
│   └── federation/        # go mod init + main.go + /health endpoint
├── packages/
│   ├── proto/             # .proto files skeleton
│   └── sdk-dart/          # dart create
├── infra/
│   ├── docker/            # docker-compose.yml (see 4b)
│   └── k8s/               # empty, placeholder README
├── docs/
│   ├── architecture.md    # copy from our architecture.md
│   ├── dev-plan.md        # copy from our dev-plan.md
│   └── adr/               # empty dir, placeholder README
├── go.work                # Go workspace linking all services
├── Makefile               # dev shortcuts (see below)
└── README.md
```

### 4b — Docker Compose (must work end-to-end)
Services in `infra/docker/docker-compose.yml`:
```yaml
postgres:16-alpine    (port 5432, volume, healthcheck)
redis:7-alpine        (port 6379, healthcheck)
nats:latest           (port 4222/8222, JetStream enabled)
minio/minio           (port 9000/9001, default creds in .env.example)
getmeili/meilisearch  (port 7700)
```
All services have healthchecks. `docker compose up` exits cleanly with all green.

### 4c — Go service skeleton (same pattern for all 9 services)
Each service has:
```
services/auth/
├── cmd/
│   └── main.go          # starts HTTP server, reads config from env
├── internal/
│   └── handler/
│       └── health.go    # GET /health → { "status": "ok", "service": "auth" }
├── Dockerfile           # multi-stage, final image < 20 MB
├── go.mod
└── README.md            # "Auth Service — what it does, how to run it"
```

### 4d — Database migrations
```
services/api/migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_workspaces.up.sql
... etc
```
Use `golang-migrate`. `make migrate-up` runs all migrations against local Postgres.

### 4e — Flutter app skeleton
```
flutter create --org gg.relay --project-name relay apps/flutter
```
Then add:
- `go_router` — routing skeleton (splash, login, workspace, channel routes)
- `riverpod` — provider tree wired up
- `drift` — DB schema stub (users, messages tables)
- Material 3 theme (light + dark mode toggle)
- Builds and runs on web: `flutter run -d chrome`

### 4f — Makefile shortcuts
```makefile
make dev          # docker compose up -d
make stop         # docker compose down
make migrate-up   # run all migrations
make migrate-down # rollback last migration
make test         # go test ./... across all services
make lint         # golangci-lint + dart analyze
make build        # docker build all service images
make flutter-web  # flutter build web
```

### 4g — GitHub Actions CI
```
.github/workflows/ci.yml
  on: [push, pull_request]
  jobs:
    lint-go:    golangci-lint on all services
    test-go:    go test ./... with postgres + redis from service containers
    lint-dart:  dart analyze + dart format --check
    test-dart:  flutter test
    build:      docker build for each service (verify Dockerfiles are valid)
```
CI must be green on `main` before the Reddit post.

---

## Step 5 — GitHub Issues & Project Board
*The contribution backlog — what every visitor looks for after the README.*

### Label system
```
# Type
good first issue     green    (< 4 hours, no deep context needed)
help wanted          blue     (medium complexity, guidance available)
bug                  red
enhancement          purple
documentation        yellow

# Area
area/backend         dark blue
area/flutter         teal
area/infra           orange
area/federation      pink

# Phase
phase/0              grey
phase/1              grey
```

### Issues to create (Phase 0 + Phase 1 — enough to keep 10 contributors busy)

**Phase 0 — good first issues (self-contained, no dependency on other issues)**
- [ ] `[Infra]` Add healthcheck to docker-compose.yml for all services
- [ ] `[BE]` Add structured logging (zerolog) to auth service
- [ ] `[BE]` Add structured logging (zerolog) to api service
- [ ] `[BE]` Write Dockerfile for messaging service
- [ ] `[BE]` Write Dockerfile for notification service
- [ ] `[BE]` Write Dockerfile for search service
- [ ] `[BE]` Write Dockerfile for file service
- [ ] `[FE]` Add dark mode toggle to Flutter app settings screen
- [ ] `[FE]` Set up Drift database schema (users + workspaces tables)
- [ ] `[Infra]` Add `make lint` target to Makefile

**Phase 1 — help wanted**
- [ ] `[BE]` Implement user registration endpoint (POST /auth/register)
- [ ] `[BE]` Implement JWT access + refresh token issuing
- [ ] `[BE]` Implement workspace CRUD (POST/GET/PATCH/DELETE /workspaces)
- [ ] `[BE]` Implement channel CRUD with member role enforcement
- [ ] `[BE]` Write migration: member_role ENUM + workspace_members table
- [ ] `[BE]` Write migration: guest_channel_access table
- [ ] `[FE]` Build login screen (email + password, form validation)
- [ ] `[FE]` Build register screen
- [ ] `[FE]` Build workspace sidebar (channel list + member list)
- [ ] `[FE]` Implement JWT refresh interceptor in Dio client

### GitHub Project board
Create a public Project (beta) board with columns:
```
Backlog | Ready | In Progress | In Review | Done
```
Add all Phase 0 and Phase 1 issues to Backlog.
Move Phase 0 issues to **Ready** (these are what contributors should pick up first).

---

## Step 6 — Community Infrastructure
*Contributors need somewhere to talk. Set this up before the post.*

### Discord server
Create a server named **Relay.gg** with channels:
```
# INFORMATION
  📌 announcements     (read-only, maintainer posts only)
  📋 roadmap           (read-only, links to dev-plan.md)

# COMMUNITY
  💬 general
  👋 introductions
  🙏 support           (self-hosting questions)

# DEVELOPMENT
  🔧 backend           (Go services discussion)
  🎨 frontend          (Flutter discussion)
  🏗️ infra             (Docker / K8s / CI)
  🔗 federation        (federation design discussion)
  🐛 bugs
  💡 ideas

# VOICE
  🔊 dev standup       (optional async)
```
Pin the GitHub repo link and CONTRIBUTING.md in #announcements.
Add Discord invite link to README.

### GitHub Discussions
Enable GitHub Discussions as a lower-friction alternative for async questions.
Categories:
- 💡 Ideas
- 🙏 Q&A
- 📣 Show and Tell
- 🗳️ Polls (for design decisions)

---

## Step 7 — Pre-Post Checklist
*Run through this the day before posting. Every item must be ✓.*

### Repository
- [ ] LICENSE file is MIT
- [ ] README renders correctly on GitHub (check mobile view)
- [ ] `docker compose up` starts all infra cleanly with no errors
- [ ] All 9 services start and return 200 on `/health`
- [ ] `make test` passes
- [ ] CI is green on `main`
- [ ] All Phase 0 issues are created and labelled
- [ ] At least 10 `good first issue` labels are set
- [ ] GitHub Project board is public

### Community
- [ ] Discord server is live with invite link in README
- [ ] GitHub Discussions are enabled

### Content
- [ ] README has a working quickstart section (tested on a clean machine)
- [ ] Architecture diagram is in the README (even a simple version)
- [ ] Feature status table is accurate and honest
- [ ] CONTRIBUTING.md explains setup in < 10 steps
- [ ] docs/architecture.md is committed

---

## Step 8 — Draft the Reddit Post

Write the post now (before you're ready) so you know what you're building toward.
Revise it the night before posting.

**Target subreddits (post in this order, space by a day each):**
1. r/selfhosted — post here first; this is the core audience
2. r/golang — backend contributors
3. r/FlutterDev — frontend contributors
4. r/opensource — general visibility
5. r/SideProject — founder story angle

**Post template (r/selfhosted):**

---
**Title:** I'm building an open-source, self-hosted Slack alternative with cross-org federation — relay.gg

**Body:**

I got tired of paying for Slack and being locked into their ecosystem, so I'm building
an open-source alternative that you can run on your own server.

**What makes Relay different from Mattermost / Rocket.Chat:**
- Built in Flutter — one codebase for web, iOS, Android, Windows, macOS, Linux
- Cross-org federation — invite people from other self-hosted instances
  (like email, but for chat)
- Voice & video huddles (WebRTC via Livekit — fully self-hosted, no Twilio)
- Block Kit-style interactive messages so bots actually work properly

**Stack:**
- Backend: Go microservices
- Frontend: Flutter (all platforms)
- Real-time: WebSocket + NATS JetStream
- Voice/Video: Livekit + Coturn
- DB: PostgreSQL + Redis

**Current status:** Phase 0 scaffolding is done. Looking for contributors.

The architecture is fully designed (federation, Block Kit, voice/video — everything).
The dev plan is broken into parallel tracks so multiple people can work simultaneously
without blocking each other.

**GitHub:** github.com/therelayproject/relay
**Discord:** [invite link]
**Contributing:** github.com/therelayproject/relay/blob/main/CONTRIBUTING.md

If you've ever wanted to build something that people actually use every day —
this is it. Happy to answer any questions.

---

**Estimated time to complete all steps before posting: 1–2 weeks**

| Step | Effort | Who |
|------|--------|-----|
| 1 — GitHub repo + legal | 1 hour | You |
| 2 — README | 3–4 hours | You |
| 3 — CONTRIBUTING.md | 2 hours | You |
| 4 — Phase 0 scaffolding | 3–5 days | You (or first contributor) |
| 5 — Issues + Project board | 2–3 hours | You |
| 6 — Discord + Discussions | 1–2 hours | You |
| 7 — Pre-post checklist | 1 hour | You |
| 8 — Reddit post | 30 min | You |

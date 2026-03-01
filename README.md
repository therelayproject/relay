# Relay.gg

**Open-source, self-hosted team chat — with cross-org federation.**

> The Slack alternative you actually own.

[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![CI](https://github.com/therelayproject/relay/actions/workflows/ci.yml/badge.svg)](https://github.com/therelayproject/relay/actions)
[![Discord](https://img.shields.io/discord/000000000?label=Discord&logo=discord)](https://discord.gg/relay-gg)

---

## Why Relay?

- **No vendor lock-in.** You own your data, your server, your history.
- **Cross-org federation.** Invite people from other self-hosted Relay instances — like email, but for chat.
- **One app, every platform.** Flutter gives us iOS, Android, Web, Windows, macOS, and Linux from a single codebase.
- **Voice & video included.** Huddles and video calls via self-hosted Livekit — no Twilio required.
- **Built for contributors.** Fully documented architecture, parallel dev tracks, good first issues ready to claim.

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│         Flutter Client  (web · iOS · Android         │
│                  Windows · macOS · Linux)             │
└────────────────────┬─────────────────────────────────┘
                     │  REST + WebSocket
          ┌──────────▼──────────┐
          │   API Gateway       │  Nginx / Caddy
          │   (TLS, rate limit) │
          └──┬─────┬────────┬───┘
             │     │        │
        ┌────▼─┐ ┌─▼──────┐ ┌▼────────────┐
        │ Auth │ │Messaging│ │  API Service │
        │      │ │(WS + WS)│ │  (REST CRUD) │
        └──────┘ └────┬────┘ └─────────────┘
                      │ NATS JetStream
          ┌───────────┼──────────────┐
     ┌────▼───┐  ┌────▼──┐  ┌───────▼────┐
     │Notif.  │  │Search │  │Integration │
     │FCM/APNS│  │Meili  │  │Block Kit   │
     └────────┘  └───────┘  └────────────┘
          │ Livekit SFU + Coturn TURN
     ┌────▼──────────┐   ┌─────────────────┐
     │ Media Service │   │Federation Service│
     │ (rooms/tokens)│   │ (S2S + Ed25519) │
     └───────────────┘   └─────────────────┘
          PostgreSQL · Redis · MinIO · Meilisearch
```

---

## Feature Status

| Feature | Status |
|---------|--------|
| Real-time messaging (channels, DMs, threads) | 🚧 In progress |
| File uploads & previews | 🚧 In progress |
| Member roles (owner / admin / member / guest) | 🚧 In progress |
| Block Kit — rich interactive messages | 📋 Planned |
| Full-text search with modifiers | 📋 Planned |
| Push notifications (FCM / APNS) | 📋 Planned |
| Voice huddles & video calls (Livekit) | 📋 Planned |
| Screen sharing | 📋 Planned |
| OAuth login (GitHub / Google) | 📋 Planned |
| SAML SSO / SCIM provisioning | 📋 Planned |
| Cross-org federation | 📋 Planned |
| Flutter Web / iOS / Android | 🚧 In progress |
| Flutter Desktop (Win / Mac / Linux) | 📋 Planned |

---

## Quickstart

**Prerequisites:** Docker, Docker Compose v2, Go 1.22+, Flutter 3.19+

```bash
# 1. Clone
git clone https://github.com/therelayproject/relay.git
cd relay

# 2. Configure
cp .env.example .env

# 3. Start infrastructure
make dev

# 4. Run migrations
make migrate-up

# 5. Start a service (example: auth)
cd services/auth && go run ./cmd
```

All infrastructure services will be available at:

| Service | URL |
|---------|-----|
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |
| NATS | `localhost:4222` |
| MinIO console | `http://localhost:9001` |
| Meilisearch | `http://localhost:7700` |

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22 (microservices) |
| Frontend | Flutter 3.19 (all platforms) |
| Real-time | WebSocket + NATS JetStream |
| Database | PostgreSQL 16 + Redis 7 |
| Search | Meilisearch |
| Object storage | MinIO (S3-compatible) |
| Voice / Video | Livekit SFU + Coturn |
| Migrations | golang-migrate |
| CI | GitHub Actions |

---

## Project Structure

```
relay/
├── apps/flutter/      # Flutter app (all 6 platform targets)
├── services/          # Go microservices
│   ├── auth/          # Authentication, JWT, OAuth, SSO
│   ├── api/           # REST API (workspaces, channels, users)
│   ├── messaging/     # WebSocket gateway, real-time fan-out
│   ├── notification/  # Push (FCM/APNS), email, desktop
│   ├── search/        # Meilisearch indexing & query
│   ├── file/          # S3 uploads, thumbnails, virus scan
│   ├── integration/   # Block Kit, webhooks, slash commands
│   ├── media/         # Voice/video rooms (Livekit)
│   └── federation/    # Cross-org S2S messaging
├── packages/
│   ├── proto/         # Shared Protobuf definitions
│   ├── sdk-dart/      # Dart client SDK
│   └── sdk-go/        # Go client SDK
└── infra/
    ├── docker/        # Docker Compose
    └── k8s/           # Kubernetes Helm chart
```

---

## Contributing

We'd love your help. Relay is built in the open and designed so multiple
contributors can work in parallel without blocking each other.

**Start here:** [CONTRIBUTING.md](CONTRIBUTING.md)

**Find a task:** [GitHub Issues](https://github.com/therelayproject/relay/issues?q=label%3A%22good+first+issue%22)

**Architecture deep-dive:** [docs/architecture.md](docs/architecture.md)

**Development plan:** [docs/dev-plan.md](docs/dev-plan.md)

---

## Community

- **Discord:** [discord.gg/relay-gg](https://discord.gg/relay-gg)
- **GitHub Discussions:** [github.com/therelayproject/relay/discussions](https://github.com/therelayproject/relay/discussions)
- **Issues:** [github.com/therelayproject/relay/issues](https://github.com/therelayproject/relay/issues)

---

## License

MIT — see [LICENSE](LICENSE).

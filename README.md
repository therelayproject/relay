# Relay

An open-source, self-hostable Slack alternative built on Go microservices and a Flutter client.

---

## Architecture Overview

Relay is a distributed messaging platform composed of **10 Go microservices** that communicate via gRPC and NATS JetStream, backed by a **Flutter** cross-platform client.

```
┌─────────────────────────────────────────────────────────────────┐
│                        Flutter Client                           │
│                   (iOS · Android · Web · Desktop)               │
└────────────────────────────┬────────────────────────────────────┘
                             │ REST / WebSocket
          ┌──────────────────▼──────────────────┐
          │            ws-gateway :8006          │
          │    (WebSocket ↔ NATS JetStream)      │
          └──────┬───────┬────────┬─────┬───────┘
                 │       │        │     │
   ┌─────────────▼─┐ ┌───▼──┐ ┌──▼──┐ ┌▼──────────────┐
   │ auth-service  │ │ user │ │ ch. │ │ message-svc   │
   │   :8001/:9001 │ │:8002 │ │:8004│ │   :8005       │
   └───────────────┘ └──────┘ └─────┘ └───────────────┘
   ┌──────────────┐ ┌────────────┐ ┌──────────┐ ┌──────────────┐
   │ workspace-svc│ │ presence   │ │ notif.   │ │ search-svc   │
   │    :8003     │ │   :8007    │ │  :8008   │ │   :8009      │
   └──────────────┘ └────────────┘ └──────────┘ └──────────────┘
                                                 ┌──────────────┐
                                                 │ file-service │
                                                 │   :8010      │
                                                 └──────────────┘
           PostgreSQL · Redis · NATS · Elasticsearch · MinIO
```

---

## Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Backend services | Go | 1.22+ |
| Client | Flutter / Dart | 3.22+ |
| Relational DB | PostgreSQL | 16 |
| Cache / sessions | Redis | 7 |
| Message bus | NATS + JetStream | 2.10 |
| Full-text search | Elasticsearch | 8.13 |
| Object storage | MinIO (S3-compatible) | latest |
| Service contracts | Protocol Buffers + gRPC | buf 1.x |
| Container runtime | Docker + Docker Compose | 24+ / v2 |
| Orchestration | Kubernetes | manifests in `k8s/` |

---

## Services

| Service | HTTP Port | gRPC Port | Purpose |
|---------|-----------|-----------|---------|
| `auth-service` | 8001 | 9001 | Registration, login, JWT/refresh tokens, OAuth |
| `user-service` | 8002 | 9002 | User profiles, avatars, preferences |
| `workspace-service` | 8003 | — | Workspace (team) creation and membership |
| `channel-service` | 8004 | — | Channels, DMs, topics, permissions |
| `message-service` | 8005 | — | Send/edit/delete messages, reactions, threads |
| `ws-gateway` | 8006 | — | WebSocket gateway — bridges clients to NATS |
| `presence-service` | 8007 | — | Online/away/offline status, typing indicators |
| `notification-service` | 8008 | — | Push, email, and in-app notifications |
| `search-service` | 8009 | — | Full-text search over messages and users |
| `file-service` | 8010 | — | File uploads/downloads via MinIO (S3) |

---

## Quickstart

### Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.22+ |
| Flutter / Dart | 3.22+ |
| Docker + Docker Compose | 24+ / v2 |
| buf | 1.x |
| golangci-lint | 1.58+ |
| make | any modern |

### 1. Clone the repository

```bash
git clone https://github.com/relay-im/relay.git
cd relay
```

### 2. Start infrastructure

```bash
make docker-up
# PostgreSQL    → localhost:5432
# Redis         → localhost:6379
# NATS          → localhost:4222 (monitoring: localhost:8222)
# Elasticsearch → localhost:9200
# MinIO S3 API  → localhost:9000 (console: localhost:9001)
```

### 3. Build all services

```bash
make build
# Binaries land in ./bin/
```

### 4. Run tests

```bash
make test
```

### 5. Run the Flutter client

```bash
cd ../relay-app
flutter pub get
flutter run   # connects to the local backend by default
```

### Single-service development

```bash
# Build one service
make build-auth-service

# Run one service
go run ./services/auth-service/cmd/service/...

# Test one service
make test-auth-service
```

---

## Repository Layout

```
relay/
├── proto/                  # Protobuf service definitions
├── shared/                 # Shared Go module (proto gen output, common libs)
├── services/
│   ├── auth-service/
│   ├── channel-service/
│   ├── file-service/
│   ├── message-service/
│   ├── notification-service/
│   ├── presence-service/
│   ├── search-service/
│   ├── user-service/
│   ├── workspace-service/
│   └── ws-gateway/
├── k8s/                    # Kubernetes manifests
├── scripts/                # Dev/CI helper scripts
├── docker-compose.yml
├── buf.yaml / buf.gen.yaml # Protobuf toolchain config
├── go.work                 # Go workspace spanning all modules
└── Makefile

relay-app/                  # Flutter client (sibling directory)
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for coding standards, branch naming, commit conventions, and the PR process.

## License

Relay is released under the terms of the [LICENSE](LICENSE) file in this repository.

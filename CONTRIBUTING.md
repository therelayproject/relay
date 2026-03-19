# Contributing to Relay

Thank you for your interest in contributing to Relay, the open-source Slack alternative!
Please read this guide before opening issues or pull requests.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Prerequisites](#prerequisites)
- [Local Development Setup](#local-development-setup)
- [Repository Layout](#repository-layout)
- [Coding Standards](#coding-standards)
- [Branch Naming](#branch-naming)
- [Commit Message Conventions](#commit-message-conventions)
- [Pull Request Process](#pull-request-process)
- [Running Tests](#running-tests)
- [Adding a New Service](#adding-a-new-service)

---

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).
By participating you agree to uphold it. Please report unacceptable behaviour to
the maintainers listed in that document.

---

## Prerequisites

| Tool | Minimum version | Install |
|------|----------------|---------|
| Go | 1.22 | <https://go.dev/dl/> |
| Flutter / Dart | 3.22 | <https://docs.flutter.dev/get-started/install> |
| Docker + Docker Compose | 24+ / v2 plugin | <https://docs.docker.com/get-docker/> |
| buf | 1.x | `brew install bufbuild/buf/buf` or <https://buf.build/docs/installation> |
| golangci-lint | 1.58+ | `brew install golangci-lint` or <https://golangci-lint.run/usage/install/> |
| make | any modern | pre-installed on macOS/Linux |

---

## Local Development Setup

### 1. Fork and clone

```bash
git clone https://github.com/<your-fork>/relay.git
cd relay
```

### 2. Start infrastructure

All backing services (PostgreSQL, Redis, NATS, Elasticsearch, MinIO) are provided
via Docker Compose:

```bash
make docker-up
# PostgreSQL  → localhost:5432
# Redis       → localhost:6379
# NATS        → localhost:4222
# Elasticsearch → localhost:9200
# MinIO       → localhost:9000
```

### 3. Tidy Go modules

```bash
make tidy
```

### 4. Generate protobuf code

```bash
make proto
# Generated files land in shared/proto/gen/
```

### 5. Build all services

```bash
make build
# Binaries land in ./bin/
```

### 6. Run a single service (example)

```bash
go run ./services/auth-service/cmd/service/...
```

Each service reads its configuration from environment variables. Copy
`.env.example` (if present in the service directory) to `.env` and adjust as
needed.

### 7. Flutter app setup

```bash
cd relay-app
flutter pub get
flutter run   # connects to local backend by default
```

---

## Repository Layout

```
relay/
├── proto/                  # Protobuf definitions
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
├── buf.yaml / buf.gen.yaml
├── go.work                 # Go workspace spanning all modules
└── Makefile

relay-app/                  # Flutter client (sibling directory)
```

---

## Coding Standards

### Go

- **Formatter**: `gofmt` + `goimports` (run `make fmt`).
- **Linter**: `golangci-lint` with the project config at `.golangci.yml`.
  Active linters: `errcheck`, `gosimple`, `govet`, `ineffassign`, `staticcheck`,
  `unused`, `gofmt`, `goimports`, `misspell`, `revive`, `bodyclose`, `noctx`,
  `sqlclosecheck`.
- Always check errors; never use `_` to discard them outside of tests.
- Use `context.Context` as the first parameter for all I/O-bound functions.
- Close HTTP response bodies (`bodyclose` will catch it).
- Import grouping order (enforced by `goimports`):
  1. Standard library
  2. Third-party
  3. Internal (`github.com/relay-im/relay/...`)
- Exported symbols must have doc comments (`revive` warns on missing ones).

Run before every commit:

```bash
make lint
make vet
```

### Dart / Flutter

- Analysis options are defined in `relay-app/analysis_options.yaml`.
- Lints extend `flutter_lints`; notable additions:
  - `always_declare_return_types`
  - `prefer_single_quotes`
  - `avoid_dynamic_calls`
  - `unawaited_futures`
- Run the analyzer:

```bash
cd relay-app
flutter analyze
```

- Format Dart code with `dart format .` before committing.

---

## Branch Naming

```
<type>/<short-description>
```

| Type | When to use |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `chore` | Maintenance, deps, tooling |
| `docs` | Documentation only |
| `refactor` | Code restructure without behaviour change |
| `test` | Test additions or fixes |
| `ci` | CI/CD pipeline changes |

Examples:

```
feat/message-reactions
fix/auth-token-expiry
docs/contributing-guide
```

---

## Commit Message Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

[optional body]

[optional footer(s)]
```

- **type**: same set as branch types above (`feat`, `fix`, `chore`, etc.)
- **scope**: service or area affected (e.g. `auth`, `ws-gateway`, `flutter`, `proto`)
- **subject**: imperative, lowercase, no trailing period, ≤72 chars
- **body**: explain *why*, not *what*; wrap at 72 chars
- **footer**: reference issues (`Closes #42`, `Fixes #99`)

Examples:

```
feat(message-service): add reaction endpoints

Implements PUT/DELETE /messages/:id/reactions backed by a new
reactions table. Includes gRPC notification fanout.

Closes #87
```

```
fix(auth-service): refresh token not invalidated on logout

The revocation check queried the wrong Redis key prefix, causing
valid refresh tokens to persist after sign-out.
```

---

## Pull Request Process

1. **Open a draft PR** early if you want feedback before the work is complete.
2. Ensure `make lint test` passes locally before marking it ready.
3. For Flutter changes, ensure `flutter analyze` and `flutter test` pass.
4. Fill in the PR template (if provided) with a clear description and
   test evidence (screenshots, log snippets).
5. Request a review from at least one maintainer.
6. Address all review comments; resolve conversations once fixed.
7. Squash-merge or rebase-merge; do not merge with a merge commit.
8. Delete the branch after merge.

### CI checks

All PRs must pass:

- `go vet ./...`
- `golangci-lint run ./...`
- `go test ./... -race`
- `flutter analyze`
- `flutter test`

---

## Running Tests

### Go (backend services)

```bash
# All tests across the workspace
make test

# Single service
make test-auth-service

# With coverage report
make test-cover
# Open coverage.html in your browser
```

### Flutter (client)

```bash
cd relay-app

# Unit + widget tests
flutter test

# Integration tests (requires a running emulator/device)
flutter test integration_test/
```

---

## Adding a New Service

Follow the structure of an existing service (e.g. `auth-service`):

```
services/<new-service>/
├── Dockerfile
├── go.mod                  # module: github.com/relay-im/relay/<new-service>
├── go.sum
├── cmd/
│   └── service/
│       └── main.go         # wires up deps and starts the server
└── internal/
    ├── domain/             # business entities and interfaces
    ├── repository/         # DB access (sqlc / pgx)
    ├── service/            # application logic
    ├── grpcserver/         # gRPC server implementation
    └── handler/            # HTTP handlers (if applicable)
```

**Step-by-step:**

1. Create the directory tree above.
2. Init the Go module:
   ```bash
   cd services/<new-service>
   go mod init github.com/relay-im/relay/<new-service>
   ```
3. Add the module to the workspace:
   ```bash
   # in repo root
   go work use ./services/<new-service>
   ```
4. Add the service name to the `SERVICES` list in `Makefile`.
5. Define the service's API in `proto/<new-service>/v1/<new-service>.proto`
   and run `make proto` to generate Go stubs.
6. Add a `Dockerfile` following the multi-stage pattern used in other services.
7. Add the service to `docker-compose.yml` if it needs infrastructure at dev time.
8. Wire up any new database migrations under `services/<new-service>/db/migrations/`.
9. Write unit tests alongside the code (`*_test.go`).
10. Open a PR — CI will validate linting and tests automatically.

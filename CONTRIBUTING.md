# Contributing to Relay.gg

Thank you for considering a contribution to Relay. This guide gets you from
zero to a merged PR as quickly as possible.

---

## Ways to contribute

- **Code** — pick up a GitHub issue and implement it
- **Documentation** — improve READMEs, architecture docs, or the OpenAPI spec
- **Testing** — write unit or integration tests for existing code
- **Bug reports** — open an issue with reproduction steps
- **Design** — UI mockups, UX feedback, icon contributions
- **Triage** — help label and reproduce open issues

---

## Before you start

1. Read the [architecture overview](docs/architecture.md) — understand the
   system before touching it.
2. Check the [development plan](docs/dev-plan.md) — understand which phase
   is active and what's being built next.
3. Find an issue to work on (see below).
4. Comment on the issue to claim it — this prevents two people building the
   same thing.

---

## Local development setup

**Prerequisites**

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.22+ | https://go.dev/dl |
| Flutter | 3.19+ | https://docs.flutter.dev/get-started/install |
| Docker + Compose v2 | latest | https://docs.docker.com/get-docker |
| golang-migrate | latest | `go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
| golangci-lint | latest | https://golangci-lint.run/usage/install |

**Setup**

```bash
# Clone
git clone https://github.com/therelayproject/relay.git
cd relay

# Configure environment
cp .env.example .env
# Edit .env if needed — defaults work for local dev

# Start all infrastructure (postgres, redis, nats, minio, meilisearch)
make dev

# Run database migrations
make migrate-up

# Verify everything is up
make health
```

**Run a service**

```bash
cd services/auth
go run ./cmd
# → {"level":"info","port":"8080","service":"auth","message":"starting"}
```

**Run the Flutter app (web)**

```bash
make flutter-web
# Opens http://localhost:8080 in browser
```

**Run all tests**

```bash
make test        # Go: all services
make test-dart   # Flutter unit + widget tests
```

**Run linters**

```bash
make lint        # golangci-lint + dart analyze
```

---

## Finding something to work on

Filter issues at https://github.com/therelayproject/relay/issues:

| Label | Meaning |
|-------|---------|
| `good first issue` | Self-contained, < 4 hours, no deep prior context needed |
| `help wanted` | Medium complexity, guidance available in the issue |
| `phase/0` | Current sprint — Phase 0 Foundation work |
| `area/backend` | Go microservices |
| `area/flutter` | Flutter / Dart client |
| `area/infra` | Docker, Kubernetes, CI |

If you have an idea not tracked in an issue, open one first and discuss the
approach before writing code.

---

## PR process

**1. Fork and branch**

```bash
git checkout -b feat/your-feature-name
# or
git checkout -b fix/issue-123-short-description
```

Branch naming convention:
- `feat/` — new feature
- `fix/` — bug fix
- `chore/` — tooling, CI, dependencies
- `docs/` — documentation only

**2. Write your code**

- Match the existing code style in the file you're editing
- Go: `gofmt` + `golangci-lint` must pass
- Dart: `dart format` + `flutter analyze` must pass
- Add tests for non-trivial logic

**3. Commit messages**

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(auth): add TOTP MFA endpoint
fix(messaging): handle WebSocket close on client disconnect
chore(ci): add dart analyze to CI pipeline
docs(api): document cursor pagination contract
```

**4. Open a PR**

- Fill in the PR template
- Link the issue: `Closes #123`
- Keep PRs focused — one feature or fix per PR

**5. Review**

- A maintainer will review within 48 hours
- Address review comments in new commits (don't force-push during review)
- Once approved, a maintainer will merge

---

## Coding principles

These are binding decisions, not suggestions. PRs that diverge from them will be asked to align before merge.

---

### Go

#### Error handling

Wrap errors with `fmt.Errorf` using a short operation description. Use sentinel errors for cases callers need to distinguish.

```go
// Wrapping — always include context
return fmt.Errorf("store.GetChannel: %w", err)

// Sentinel errors — declared at package level
var ErrNotFound  = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

// Caller checks with errors.Is — never string-match
if errors.Is(err, store.ErrNotFound) {
    writeError(w, http.StatusNotFound, "channel not found")
    return
}
```

Never `panic` in request handlers. Never use `github.com/pkg/errors` — stdlib `fmt.Errorf("%w")` covers it.

#### Logging (zerolog)

Always structured, always with relevant fields. Never log inside `internal/store/` or `internal/service/` — return the error and let the handler log it with full context.

```go
// Good — structured, at handler level
log.Error().
    Str("user_id", claims.UserID).
    Str("channel_id", channelID).
    Err(err).
    Msg("failed to create message")

// Bad — no context, wrong layer
fmt.Println("error:", err)          // never
log.Printf("error: %v", err)        // never (stdlib log)
log.Info().Msg(err.Error())         // no Err() field
```

Never log JWT values, passwords, tokens, or any secret material — even at debug level.

#### Interfaces

Define interfaces at the **consumer** (the thing that calls), not the implementer. Keep them small.

```go
// Good — defined in the handler package that needs it
type ChannelStore interface {
    GetChannel(ctx context.Context, id int64) (*model.Channel, error)
    CreateChannel(ctx context.Context, input CreateChannelInput) (*model.Channel, error)
}

// Bad — fat interface defined next to the implementation
type Store interface {
    GetChannel(...)
    CreateChannel(...)
    ListChannels(...)
    DeleteChannel(...)
    GetUser(...)
    // ... 20 more methods
}
```

Only create an interface when you have ≥2 concrete implementations **or** need to mock in tests. Name single-action interfaces with `-er`: `MessageSender`, `TokenValidator`.

#### Testing

Use `testify` for assertions. Table-driven tests for all deterministic functions. No real network, DB, or NATS calls in unit tests.

```go
func TestPermissionGuard(t *testing.T) {
    tests := []struct {
        name    string
        role    string
        wantErr bool
    }{
        {"owner can post", "owner", false},
        {"guest cannot post in announcement", "guest", true},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            err := checkPermission(tc.role, "announcements")
            if tc.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

Use `testify/mock` and an interface to mock stores, NATS, and external HTTP clients — never make real calls in unit tests.

#### HTTP handlers

All handlers use the stdlib shape. Never return an error from a handler — write it directly.

```go
// Handler shape — consistent across all 9 services
func (h *Handler) CreateMessage(w http.ResponseWriter, r *http.Request) {
    claims := middleware.ClaimsFrom(r.Context())

    var input CreateMessageInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    msg, err := h.store.InsertMessage(r.Context(), input)
    if err != nil {
        log.Error().Err(err).Msg("insert message")
        writeError(w, http.StatusInternalServerError, "failed to create message")
        return
    }

    writeJSON(w, http.StatusCreated, msg)
}

// Shared helpers — add these to internal/handler/response.go in each service
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, map[string]string{"error": msg})
}
```

All endpoints are under `/api/v1/`. Never use a bare `/api/` prefix. A versioned endpoint is never broken — add `/api/v2/` instead.

#### Database

Driver: `pgx/v5` with `pgxpool`. No ORM. No SQL outside `internal/store/`.

```go
// store/messages.go — all SQL lives here
func (s *Store) InsertMessage(ctx context.Context, input InsertMessageInput) (*model.Message, error) {
    row := s.pool.QueryRow(ctx, `
        INSERT INTO messages (channel_id, user_id, content, blocks)
        VALUES ($1, $2, $3, $4)
        RETURNING id, channel_id, user_id, content, created_at
    `, input.ChannelID, input.UserID, input.Content, input.Blocks)

    var m model.Message
    if err := row.Scan(&m.ID, &m.ChannelID, &m.UserID, &m.Content, &m.CreatedAt); err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, store.ErrNotFound
        }
        return nil, fmt.Errorf("store.InsertMessage: %w", err)
    }
    return &m, nil
}

// Transactions
tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
if err != nil { return fmt.Errorf("begin tx: %w", err) }
defer tx.Rollback(ctx) // no-op after Commit
// ... do work ...
return tx.Commit(ctx)
```

#### Constants

No magic numbers. Name every timeout, limit, port, and threshold.

```go
// Good
const (
    maxMessageLength   = 40_000
    wsReadTimeout      = 60 * time.Second
    natsPublishTimeout = 5 * time.Second
)

// Bad
time.Sleep(60 * time.Second)
if len(content) > 40000 { ... }
```

---

### Dart / Flutter

#### State management (Riverpod)

`AsyncNotifier` for network/DB. `Notifier` for local UI state. Code-generate all providers.

```dart
// Good — AsyncNotifier for anything async
@riverpod
class MessageListNotifier extends _$MessageListNotifier {
  @override
  Future<List<Message>> build(String channelId) async {
    return ref.read(messageRepositoryProvider).getMessages(channelId);
  }

  Future<void> send(String content) async {
    await ref.read(messageRepositoryProvider).send(channelId, content);
    ref.invalidateSelf();
  }
}

// Bad — setState in feature code
class MessageListWidget extends StatefulWidget { ... }
class _MessageListWidgetState extends State<MessageListWidget> {
  List<Message> _messages = [];
  void _load() async {
    final msgs = await api.getMessages(channelId); // never
    setState(() => _messages = msgs);
  }
}
```

Colocate providers with their feature: `lib/features/messaging/provider.dart`. Never put providers in `lib/providers/` as a global dumping ground.

#### Widget decomposition

Extract to a named class when `build` exceeds ~50 lines. One public widget class per file.

```dart
// Good — named extraction
class MessageBubble extends ConsumerWidget {
  const MessageBubble({super.key, required this.message});
  final Message message;

  @override
  Widget build(BuildContext context, WidgetRef ref) { ... }
}

// Bad — anonymous builder buried in parent
Column(children: [
  Builder(builder: (ctx) => Builder(builder: (ctx2) => Container(
    child: GestureDetector(onTap: () { ... },
      child: Text(message.content)), // 60 more lines follow
  ))),
])
```

Use `ConsumerWidget` by default. Only reach for `ConsumerStatefulWidget` when you genuinely need `initState`, `dispose`, or an `AnimationController`.

#### Models

```dart
// API response model — fromJson factory
class Message {
  final String id;      // String, not int — Snowflake precision
  final String content;
  final DateTime createdAt;

  factory Message.fromJson(Map<String, dynamic> json) => Message(
    id: json['id'] as String,
    content: json['content'] as String,
    createdAt: DateTime.parse(json['created_at'] as String),
  );
}

// Domain model — use freezed for immutability + copyWith + equality
@freezed
class MessageViewModel with _$MessageViewModel {
  const factory MessageViewModel({
    required String id,
    required String content,
    required bool isPending,
    required UserClassification authorClassification,
  }) = _MessageViewModel;
}

// Drift DataClass → domain model mapping (in a repository, not a widget)
MessageViewModel _fromDrift(MessagesTableData row) => MessageViewModel(
  id: row.id,
  content: row.content,
  isPending: row.localStatus == 'pending',
  authorClassification: UserClassification.internal,
);
```

Never pass a Drift `DataClass` directly into a widget. Always map it first.

#### Async

```dart
// Good — explicit all three states
ref.watch(messageListProvider(channelId)).when(
  data: (messages) => MessageListView(messages: messages),
  loading: () => const MessageListSkeleton(),
  error: (err, _) => ErrorBanner(message: err.toString()),
);

// Bad — await in build, silent failure
@override
Widget build(BuildContext context) {
  final messages = await api.getMessages(channelId); // never
  return ListView(...);
}
```

---

### Cross-cutting

**Security**
- All user-supplied strings (channel names, message content, search queries) are untrusted until validated
- JWT secret must be ≥32 bytes — assert at startup and fail fast: `if len(secret) < 32 { log.Fatal()... }`
- Never log JWT values, raw tokens, passwords, or TOTP secrets at any log level

**API versioning**
- All REST endpoints: `/api/v1/<resource>`
- Breaking changes get a new version prefix; old versions are deprecated with a sunset header before removal

**NATS events**
- Event structs are append-only: add new optional fields, never remove or rename existing ones
- Subscribers must tolerate unknown fields gracefully (use `json.Unmarshal` which ignores unknown fields by default)

---

## Service conventions (Go)

Every service follows this structure:

```
services/{name}/
├── cmd/
│   └── main.go          # Entry point — wire up HTTP server, env config, graceful shutdown
├── internal/
│   ├── handler/         # HTTP handlers (one file per resource group)
│   ├── service/         # Business logic (no HTTP concerns)
│   ├── repository/      # DB access (PostgreSQL via pgx)
│   └── model/           # Domain types
├── Dockerfile
├── go.mod
└── README.md
```

Every service must have:
- `GET /health` returning `{"status":"ok","service":"{name}"}`
- Structured zerolog logging
- Graceful 30-second SIGTERM shutdown
- `PORT` env var for the listen port

---

## Database migrations

Migrations live in `services/api/migrations/` and use `golang-migrate`.

```bash
# Create a new migration
make migration name=add_reactions_table

# Apply migrations
make migrate-up

# Rollback one
make migrate-down
```

Rules:
- Every `.up.sql` must have a corresponding `.down.sql`
- Migrations must be idempotent (`IF NOT EXISTS`, `IF EXISTS`)
- Never modify a migration that has been merged to `main` — add a new one

---

## Getting help

- **Discord `#contributors`** — fastest response, general questions
- **Discord `#backend` / `#frontend` / `#infra`** — area-specific questions
- **GitHub Discussions** — design discussions, RFCs
- **Issue comments** — questions about a specific task

We're friendly. Ask anything.

# Search Service

Full-text search powered by Meilisearch. Consumes messages, files, users, and channels from NATS JetStream and indexes them in workspace-scoped Meilisearch indexes. Supports search modifiers (from:, in:, before:, after:, has:link).

**Port:** 8084 (default) | **Health:** `GET /health`

## Running locally

```bash
cd services/search && go run ./cmd
```

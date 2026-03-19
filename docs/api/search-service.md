# Search Service API

**Port:** 8009 (HTTP)

The search service provides full-text search across messages within a workspace, backed by Elasticsearch. It consumes NATS events to index messages in near-real time.

## Authentication

All endpoints require a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)`.

## REST Endpoints

#### `GET /api/v1/workspaces/{workspace_id}/search`

Search messages within a workspace.

- **Auth:** JWT required
- **Path params:** `workspace_id`
- **Query params:**

  | Param | Type | Required | Description |
  |-------|------|----------|-------------|
  | `q` | string | Yes | Full-text search query |
  | `channel_id` | string | No | Restrict results to a specific channel |
  | `author_id` | string | No | Restrict results to a specific author |
  | `after` | string (RFC3339) | No | Return messages created after this timestamp |
  | `before` | string (RFC3339) | No | Return messages created before this timestamp |
  | `from` | integer | No | Pagination offset (default: 0) |
  | `size` | integer | No | Number of results to return (default: 20, max: 100) |

- **Response `200`:**
  ```json
  {
    "hits": [
      {
        "id": "string",
        "channel_id": "string",
        "workspace_id": "string",
        "author_id": "string",
        "body": "string",
        "created_at": "2024-01-01T00:00:00Z",
        "highlight": {
          "body": ["...matched <em>term</em>..."]
        }
      }
    ],
    "total": 42,
    "from": 0,
    "size": 20
  }
  ```

## Indexing

The service consumes NATS subjects to keep the Elasticsearch index current:

| Subject | Action |
|---------|--------|
| `message.created` | Index new message |
| `message.updated` | Update existing indexed message |
| `message.deleted` | Remove message from index |

## Elasticsearch Index

Messages are indexed under a workspace-scoped index. Indexed fields include: `id`, `workspace_id`, `channel_id`, `author_id`, `body`, `created_at`, `deleted`.

The `body` field uses a text analyzer for full-text search with highlighting support.

## Error Responses

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `INVALID_ARGUMENT` (400 — missing `q` param), `INTERNAL` (500).

# File Service API

**Port:** 8010 (HTTP)

The file service handles file uploads, metadata retrieval, and presigned download/thumbnail URL generation. Files are stored in MinIO (S3-compatible object storage).

## Authentication

All endpoints require a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)`.

## REST Endpoints

#### `POST /api/v1/workspaces/{workspace_id}/files`

Upload a file to a workspace, optionally associating it with a channel.

- **Auth:** JWT required
- **Path params:** `workspace_id`
- **Request body:** `multipart/form-data`
  - `file` (required) — file content, max size: **32 MiB**
  - `channel_id` (optional) — associate the file with a channel
- **Response `201`:**
  ```json
  {
    "id": "string",
    "workspace_id": "string",
    "channel_id": "string",
    "uploader_id": "string",
    "filename": "string",
    "content_type": "image/png",
    "size_bytes": 102400,
    "is_image": true,
    "has_thumbnail": true,
    "created_at": "2024-01-01T00:00:00Z"
  }
  ```

#### `GET /api/v1/files/{file_id}`

Get file metadata.

- **Auth:** JWT required
- **Path params:** `file_id`
- **Response `200`:**
  ```json
  {
    "id": "string",
    "workspace_id": "string",
    "channel_id": "string",
    "uploader_id": "string",
    "filename": "string",
    "content_type": "string",
    "size_bytes": 102400,
    "is_image": false,
    "has_thumbnail": false,
    "created_at": "2024-01-01T00:00:00Z"
  }
  ```

#### `GET /api/v1/files/{file_id}/download`

Get a presigned URL to download the file directly from object storage.

- **Auth:** JWT required
- **Path params:** `file_id`
- **Response `200`:**
  ```json
  { "url": "https://..." }
  ```
  The URL is time-limited. Redirect the client to this URL to stream the file.

#### `GET /api/v1/files/{file_id}/thumbnail`

Get a presigned URL for the file's thumbnail image (only valid when `has_thumbnail` is `true`).

- **Auth:** JWT required
- **Path params:** `file_id`
- **Response `200`:**
  ```json
  { "url": "https://..." }
  ```

## Storage

Files are stored in MinIO (S3-compatible). The service generates presigned URLs so clients download directly from object storage without proxying through the service.

Thumbnails are generated automatically for uploaded images.

## Error Responses

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND` (404), `PAYLOAD_TOO_LARGE` (413 — exceeds 32 MiB), `INVALID_ARGUMENT` (400), `INTERNAL` (500).

# File Service

Manages file uploads and downloads via S3-compatible storage (MinIO). Generates presigned URLs for direct client uploads, runs async ClamAV virus scans, generates image thumbnails via libvips, and enforces per-workspace storage quotas.

**Port:** 8085 (default) | **Health:** `GET /health`

## Running locally

```bash
cd services/file && go run ./cmd
```

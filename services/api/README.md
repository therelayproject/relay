# API Service
REST API for workspaces, channels, users, members, files, and search. Enforces role-based permissions via PermissionGuard middleware. All list endpoints use Snowflake-ID cursor pagination.

**Port:** 8081 (default)
**Health:** `GET /health`

## Running locally
```bash
cd services/api
go run ./cmd
```

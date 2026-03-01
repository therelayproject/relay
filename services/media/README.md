# Media Service

Control plane for voice and video calls. Manages room lifecycle (create/join/leave/end), issues short-lived Livekit JWT access tokens, generates ephemeral TURN credentials (HMAC-SHA1 via Coturn), and handles Livekit webhooks to keep room_participants DB in sync.

**Port:** 8087 (default) | **Health:** `GET /health`

## Running locally

```bash
cd services/media && go run ./cmd
```

**Required env vars:** LIVEKIT_API_KEY, LIVEKIT_API_SECRET, LIVEKIT_URL, COTURN_SECRET

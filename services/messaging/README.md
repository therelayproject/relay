# Messaging Service
WebSocket gateway for real-time messaging. Manages connections via Redis, fans out messages via NATS JetStream, handles presence, typing indicators, and read receipts.

**Port:** 8082 (default)
**Health:** `GET /health`

## Running locally
```bash
cd services/messaging
go run ./cmd
```

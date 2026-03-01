# Notification Service

Dispatches push notifications (FCM/APNS), transactional email (SMTP), and WebSocket desktop notifications. Consumes events from NATS JetStream. Respects per-user DND schedules and per-channel notification preferences.

**Port:** 8083 (default) | **Health:** `GET /health`

## Running locally

```bash
cd services/notification && go run ./cmd
```

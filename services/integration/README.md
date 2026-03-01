# Integration Service

Manages third-party app integrations: outbound webhooks (HMAC-SHA256 signed), slash commands, bot API, Block Kit message validation, interactive action dispatch (block_action → NATS → app endpoint), and Socket Mode for apps without a public endpoint.

**Port:** 8086 (default) | **Health:** `GET /health`

## Running locally

```bash
cd services/integration && go run ./cmd
```

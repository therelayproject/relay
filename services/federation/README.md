# Federation Service

Enables cross-organisation messaging between separately hosted Relay instances. Handles server identity (Ed25519 keypair), serves /.well-known/openslack discovery endpoint, signs and verifies all server-to-server HTTP requests, manages the trusted server registry, enforces domain classification policies (internal/external/blocked), and routes federated channel invites and DMs.

**Port:** 8088 (default) | **Health:** `GET /health`

## Running locally

```bash
cd services/federation && go run ./cmd
```

**Required env vars:** SERVER_DOMAIN, FEDERATION_PRIVATE_KEY_PATH

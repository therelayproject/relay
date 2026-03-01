# proto

Shared Protobuf definitions for internal gRPC communication between Relay services.

## Structure

```
proto/
  buf.yaml              — buf.build configuration
  auth/v1/auth.proto    — token validation + revocation RPC
  messaging/v1/         — inbound message publication RPC (used by Federation)
  gen/                  — generated Go stubs (committed, do not edit manually)
```

## Generating stubs

Install buf: https://buf.build/docs/installation

```bash
cd packages/proto
buf generate
```

The generated Go code is committed to `gen/` so services can import it without
requiring contributors to have buf installed.

## Status

Stubs are defined. Generated Go code will be added in Phase 1 when inter-service
gRPC calls are first implemented (Auth ↔ Messaging WebSocket upgrade path).

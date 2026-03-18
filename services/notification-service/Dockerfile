# syntax=docker/dockerfile:1

# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder
WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work.sum* ./
COPY shared/ ./shared/

ARG SERVICE
COPY services/${SERVICE}/ ./services/${SERVICE}/

WORKDIR /workspace/services/${SERVICE}
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /service ./cmd/service/...

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/service"]

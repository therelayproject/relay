FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /service ./cmd/

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /service /service
EXPOSE 8083
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s \
    CMD wget -qO- http://localhost:8083/health || exit 1
CMD ["/service"]

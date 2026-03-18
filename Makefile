# Relay Monorepo Makefile
# Usage: make <target>

.PHONY: all build test lint fmt proto docker-up docker-down tidy help

SERVICES := auth-service user-service workspace-service channel-service \
            message-service ws-gateway presence-service notification-service \
            search-service file-service

GO      := go
GOFLAGS := -trimpath
LINTER  := golangci-lint

# ── Default target ────────────────────────────────────────────────────────────
all: lint test build

# ── Build ─────────────────────────────────────────────────────────────────────
build: ## Build all services
	@echo "==> Building all services..."
	@for svc in $(SERVICES); do \
		echo "    building $$svc..."; \
		$(GO) build $(GOFLAGS) -o bin/$$svc ./services/$$svc/cmd/service/... || exit 1; \
	done
	@echo "==> Done. Binaries in ./bin/"

build-%: ## Build a single service (e.g. make build-auth-service)
	@echo "==> Building $*..."
	$(GO) build $(GOFLAGS) -o bin/$* ./services/$*/cmd/service/...

# ── Test ──────────────────────────────────────────────────────────────────────
test: ## Run all tests
	@echo "==> Running tests..."
	$(GO) test ./... -count=1 -race -timeout 120s

test-cover: ## Run tests with coverage report
	$(GO) test ./... -count=1 -race -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "==> Coverage report: coverage.html"

test-%: ## Test a single service (e.g. make test-auth-service)
	$(GO) test ./services/$*/... -count=1 -race

# ── Lint & Format ─────────────────────────────────────────────────────────────
lint: ## Run golangci-lint
	@echo "==> Linting..."
	$(LINTER) run ./...

fmt: ## Format all Go files
	@echo "==> Formatting..."
	$(GO) fmt ./...
	goimports -w .

vet: ## Run go vet
	$(GO) vet ./...

# ── Proto ─────────────────────────────────────────────────────────────────────
proto: ## Generate protobuf code via buf
	@echo "==> Generating proto..."
	buf generate
	@echo "==> Proto generated in shared/proto/gen/"

proto-lint: ## Lint proto definitions
	buf lint

proto-breaking: ## Check proto for breaking changes
	buf breaking --against '.git#branch=main'

# ── Modules ───────────────────────────────────────────────────────────────────
tidy: ## Tidy all Go modules
	@echo "==> Tidying modules..."
	$(GO) work sync
	@for mod in shared $(addprefix services/,$(SERVICES)); do \
		echo "    tidying $$mod"; \
		(cd $$mod && $(GO) mod tidy) || exit 1; \
	done

# ── Docker ───────────────────────────────────────────────────────────────────
docker-up: ## Start all infrastructure services
	docker compose up -d
	@echo "==> Infrastructure up. PostgreSQL=5432 Redis=6379 NATS=4222 ES=9200 MinIO=9000"

docker-down: ## Stop and remove all infrastructure containers
	docker compose down

docker-reset: ## Stop, remove volumes, and restart (WARNING: destroys data)
	docker compose down -v
	docker compose up -d

docker-logs: ## Tail logs from all infrastructure services
	docker compose logs -f

# ── Misc ──────────────────────────────────────────────────────────────────────
help: ## Show this help
	@grep -E '^[a-zA-Z_%-]+:.*## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'

.PHONY: dev stop restart logs health \
        migrate-up seed migrate-down migration \
        test test-dart lint lint-dart \
        build flutter-web flutter-build \
        tidy clean

COMPOSE := docker compose -f infra/docker/docker-compose.yml
MIGRATE  := migrate -path services/api/migrations -database "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable"

# ── Infrastructure ─────────────────────────────────────────────────────────────

## Start all infrastructure services (postgres, redis, nats, minio, meilisearch)
dev:
	@echo "Starting infrastructure..."
	$(COMPOSE) up -d
	@echo "All services up. Run 'make health' to verify."

## Stop all infrastructure services
stop:
	$(COMPOSE) down

## Restart all infrastructure services
restart:
	$(COMPOSE) restart

## Tail logs from all infrastructure services
logs:
	$(COMPOSE) logs -f

## Check health of all infrastructure services
health:
	@echo "Checking infrastructure health..."
	@$(COMPOSE) ps
	@echo ""
	@for port in 8080 8081 8082 8083 8084 8085 8086 8087 8088; do \
		response=$$(curl -sf http://localhost:$$port/health 2>/dev/null) && \
		echo "  ✓ :$$port → $$response" || \
		echo "  ✗ :$$port → not running"; \
	done

# ── Database ───────────────────────────────────────────────────────────────────

## Run all pending migrations
migrate-up:
	@echo "Running migrations..."
	$(MIGRATE) up
	@echo "Migrations complete."

## Seed the database with local development fixtures (idempotent)
seed:
	@echo "Seeding database..."
	$(COMPOSE) exec -T postgres psql -U relay -d relay -f /dev/stdin < services/api/migrations/seed.sql
	@echo "Seed complete."

## Roll back the last migration
migrate-down:
	@echo "Rolling back last migration..."
	$(MIGRATE) down 1

## Create a new migration pair: make migration name=your_migration_name
migration:
	@test -n "$(name)" || (echo "Usage: make migration name=your_migration_name" && exit 1)
	@count=$$(ls services/api/migrations/*.up.sql 2>/dev/null | wc -l); \
	seq=$$(printf "%06d" $$((count + 1))); \
	touch services/api/migrations/$${seq}_$(name).up.sql; \
	touch services/api/migrations/$${seq}_$(name).down.sql; \
	echo "Created: services/api/migrations/$${seq}_$(name).{up,down}.sql"

# ── Testing ────────────────────────────────────────────────────────────────────

## Run all Go tests across all services
test:
	@echo "Running Go tests..."
	go test ./services/... ./packages/...

## Run Flutter unit and widget tests
test-dart:
	@echo "Running Flutter tests..."
	cd apps/flutter && flutter test

# ── Linting ────────────────────────────────────────────────────────────────────

## Run golangci-lint on all Go code
lint:
	@echo "Linting Go..."
	golangci-lint run ./services/... ./packages/...

## Run dart analyze and format check on Flutter code
lint-dart:
	@echo "Linting Dart..."
	cd apps/flutter && dart format --output=none --set-exit-if-changed . && flutter analyze

# ── Build ──────────────────────────────────────────────────────────────────────

## Build Docker images for all services
build:
	@for svc in auth api messaging notification search file integration media federation; do \
		echo "Building $$svc..."; \
		docker build -t relay-gg/$$svc:dev services/$$svc; \
	done

## Run Flutter web in development mode
flutter-web:
	cd apps/flutter && flutter run -d chrome

## Build Flutter web for production
flutter-build:
	cd apps/flutter && flutter build web --release

# ── Maintenance ────────────────────────────────────────────────────────────────

## Run go mod tidy across all services
tidy:
	@for svc in auth api messaging notification search file integration media federation; do \
		echo "Tidying services/$$svc..."; \
		cd services/$$svc && go mod tidy && cd ../..; \
	done
	cd packages/sdk-go && go mod tidy

## Remove build artefacts
clean:
	rm -rf apps/flutter/build
	find . -name "*.out" -delete
	find . -name "coverage.html" -delete

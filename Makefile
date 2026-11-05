# Load .env (if present) and export its variables to every recipe.
ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE_PROJECT ?= tutorhub
COMPOSE := docker compose -p $(COMPOSE_PROJECT) -f deploy/docker-compose.yml $(if $(wildcard .env),--env-file .env)

POSTGRES_USER ?= tutorhub
POSTGRES_PASSWORD ?= tutorhub
POSTGRES_PORT ?= 5432
TEST_DB ?= tutorhub_test
TEST_DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(TEST_DB)?sslmode=disable
ADMIN ?= 1

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# ---- Local development -----------------------------------------------------

.PHONY: dev
dev: db-up migrate-up ## Run Postgres (docker), then API and Vite on the host
	@$(MAKE) -j2 --no-print-directory dev-api dev-web

.PHONY: dev-api
dev-api:
	cd backend && go run ./cmd/api

.PHONY: dev-web
dev-web: frontend/node_modules
	cd frontend && npm run dev

frontend/node_modules: frontend/package-lock.json
	cd frontend && npm ci
	@touch $@

.PHONY: db-up
db-up: ## Start only Postgres and wait until it is healthy
	$(COMPOSE) up -d --wait postgres

.PHONY: seed
seed: ## Load dev seed data (demo teachers in Almaty); ADMIN=<telegram_id> becomes admin (default 1)
	$(COMPOSE) exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -v admin_id=$(ADMIN) -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < backend/seed/dev.sql

.PHONY: db-reset
db-reset: ## DELETE the local Postgres volume and start fresh (dev data is lost)
	$(COMPOSE) rm -sf postgres
	docker volume rm -f $(COMPOSE_PROJECT)_pgdata
	$(MAKE) --no-print-directory db-up migrate-up

# ---- Migrations ------------------------------------------------------------

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	cd backend && go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the most recent migration
	cd backend && go run ./cmd/migrate down

.PHONY: migrate-status
migrate-status: ## Show migration status
	cd backend && go run ./cmd/migrate status

# ---- Build / test / lint ---------------------------------------------------

.PHONY: build
build: build-api build-web ## Build backend binaries and the frontend bundle

.PHONY: build-api
build-api:
	cd backend && CGO_ENABLED=0 go build -o bin/api ./cmd/api && CGO_ENABLED=0 go build -o bin/migrate ./cmd/migrate

.PHONY: build-web
build-web: frontend/node_modules
	cd frontend && npm run build

.PHONY: test
test: ## Run backend unit tests (DB tests are skipped)
	cd backend && env -u TEST_DATABASE_URL go test -race ./...

.PHONY: test-db
test-db: db-up ## Run all backend tests, including DB integration tests, against $(TEST_DB)
	$(COMPOSE) exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '"'"'$(TEST_DB)'"'"'" | grep -q 1 || createdb -U "$$POSTGRES_USER" $(TEST_DB)'
	cd backend && TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -race -count=1 -p 1 ./...

.PHONY: lint-api
lint-api: ## Lint docs/api.yaml with redocly
	npx --yes @redocly/cli@latest lint docs/api.yaml

.PHONY: lint
lint: ## gofmt check + go vet (+ golangci-lint if installed)
	@cd backend && out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	cd backend && go vet ./...
	@if command -v golangci-lint >/dev/null; then cd backend && golangci-lint run; else echo "golangci-lint not installed, skipping"; fi

# ---- Production (run these on the server, in /opt/tutorhub) ---------------

PROD_PROJECT ?= tutorhub
PROD_COMPOSE := docker compose -p $(PROD_PROJECT) --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml

.PHONY: prod-up
prod-up: ## [server] Rebuild and restart the current checkout, then health-check
	APP_DIR=$(CURDIR) COMPOSE_PROJECT=$(PROD_PROJECT) bash deploy/deploy.sh --no-pull

.PHONY: prod-logs
prod-logs: ## [server] Follow logs of all services (Ctrl+C to stop)
	$(PROD_COMPOSE) logs -f --tail 200

.PHONY: prod-ps
prod-ps: ## [server] Show container status
	$(PROD_COMPOSE) ps -a

.PHONY: prod-seed
prod-seed: ## [server] Load the 8 demo teachers (optional, for a demo): make prod-seed ADMIN=<your telegram id>
	@if [ "$(origin ADMIN)" != "command line" ]; then echo "usage: make prod-seed ADMIN=<your telegram id>"; exit 1; fi
	$(PROD_COMPOSE) exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -v admin_id=$(ADMIN) -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < backend/seed/dev.sql

.PHONY: prod-unseed
prod-unseed: ## [server] Hide the demo teachers (users 1001-1008) from the catalogue
	$(PROD_COMPOSE) exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -c "UPDATE specialists SET status = '"'"'hidden'"'"' WHERE user_id BETWEEN 1001 AND 1008"'

.PHONY: edge-logs
edge-logs: ## [server] Follow the shared edge proxy logs
	docker compose -p edge -f /opt/edge/docker-compose.yml logs -f --tail 200 edge

.PHONY: prod-psql
prod-psql: ## [server] Open psql on the production database
	$(PROD_COMPOSE) exec postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

.PHONY: prod-backup
prod-backup: ## [server] Dump the database to backups/ now (keeps the newest 7)
	APP_DIR=$(CURDIR) COMPOSE_PROJECT=$(PROD_PROJECT) bash deploy/backup.sh

.PHONY: prod-restore
prod-restore: ## [server] REPLACE the database with a backup: make prod-restore FILE=backups/<file>.sql.gz
	@if [ -z "$(FILE)" ] || [ ! -f "$(FILE)" ]; then echo "usage: make prod-restore FILE=backups/tutorhub-YYYYMMDD-HHMMSS.sql.gz"; echo "available:"; ls -1 backups/*.sql.gz 2>/dev/null; exit 1; fi
	$(PROD_COMPOSE) stop api
	set -o pipefail; gunzip -c "$(FILE)" | $(PROD_COMPOSE) exec -T postgres sh -c 'psql -q -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'
	$(PROD_COMPOSE) start api
	@echo "Restored $(FILE)."

# ---- Full stack in docker --------------------------------------------------

.PHONY: up
up: build-web ## Build the frontend and start postgres, migrate, api, caddy
	$(COMPOSE) up -d --build

.PHONY: down
down: ## Stop the docker stack (keeps data)
	$(COMPOSE) down

.PHONY: logs
logs: ## Tail docker stack logs
	$(COMPOSE) logs -f

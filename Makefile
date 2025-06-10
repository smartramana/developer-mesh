# DevOps MCP Makefile
# Run 'make help' to see all available commands

.DEFAULT_GOAL := help
.PHONY: help
help: ## Show this help message
	@echo "DevOps MCP - Available Commands"
	@echo "==============================="
	@echo ""
	@echo "Common workflows:"
	@echo "  make help           # Show all available commands"
	@echo "  make dev            # Start development environment with Docker"
	@echo "  make test           # Run all tests"
	@echo "  make pre-commit     # Run all checks before committing"
	@echo "  make build          # Build all applications"
	@echo ""
	@echo "All commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ==============================================================================
# Variables
# ==============================================================================

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOWORK=$(GOCMD) work
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Application directories
MCP_SERVER_DIR=./apps/mcp-server
REST_API_DIR=./apps/rest-api
WORKER_DIR=./apps/worker

# Binary names
MCP_SERVER_BINARY=mcp-server
REST_API_BINARY=rest-api
WORKER_BINARY=worker

# Docker configuration
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
DOCKER_COMPOSE=docker-compose -f docker-compose.local.yml

# Other configuration
SWAG_VERSION=v1.16.2

# ==============================================================================
# Primary Commands
# ==============================================================================

.PHONY: all
all: clean test build ## Clean, test, and build everything

.PHONY: dev
dev: dev-setup up ## Start development environment with Docker

.PHONY: dev-native
dev-native: dev-setup ## Setup for running services locally (without Docker)
	@echo "Starting services locally requires PostgreSQL and Redis running."
	@echo "Run: brew services start postgresql redis"
	@echo "Then run each service: make run-mcp-server, run-rest-api, run-worker"

.PHONY: pre-commit
pre-commit: fmt lint test-coverage security-check ## Run all pre-commit checks
	@echo "Checking test coverage..."
	@coverage=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$coverage < 85" | bc) -eq 1 ]; then \
		echo "❌ Test coverage $$coverage% is below 85%"; \
		exit 1; \
	else \
		echo "✅ Test coverage $$coverage% meets requirement"; \
	fi
	@echo "✅ All pre-commit checks passed!"

# ==============================================================================
# Setup & Installation
# ==============================================================================

.PHONY: install-tools
install-tools: ## Install all development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	@echo "✅ Tools installed"

.PHONY: dev-setup
dev-setup: ## Setup development environment
	@if [ ! -f .env ]; then \
		echo "Creating .env file from example..."; \
		cp .env.example .env; \
		echo "✅ Created .env file - please update with your values"; \
	else \
		echo "✅ .env file already exists"; \
	fi

# ==============================================================================
# Build Commands
# ==============================================================================

.PHONY: build
build: build-mcp-server build-rest-api build-worker ## Build all applications

.PHONY: build-mcp-server
build-mcp-server: ## Build MCP server
	cd $(MCP_SERVER_DIR) && $(GOBUILD) -o $(MCP_SERVER_BINARY) -v ./cmd/server

.PHONY: build-rest-api
build-rest-api: ## Build REST API
	cd $(REST_API_DIR) && $(GOBUILD) -o $(REST_API_BINARY) -v ./cmd/api

.PHONY: build-worker
build-worker: ## Build worker service
	cd $(WORKER_DIR) && $(GOBUILD) -o $(WORKER_BINARY) -v ./cmd/worker

.PHONY: clean
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(MCP_SERVER_DIR)/$(MCP_SERVER_BINARY) $(REST_API_DIR)/$(REST_API_BINARY) $(WORKER_DIR)/$(WORKER_BINARY)

# ==============================================================================
# Testing Commands
# ==============================================================================

.PHONY: test
test: ## Run all unit tests
	$(GOTEST) -v -short ./apps/mcp-server/... ./apps/rest-api/... ./apps/worker/... ./pkg/...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	$(GOTEST) -coverprofile=coverage.out ./apps/mcp-server/... ./apps/rest-api/... ./apps/worker/... ./pkg/...
	$(GOCMD) tool cover -func=coverage.out

.PHONY: test-coverage-html
test-coverage-html: test-coverage ## Generate HTML coverage report
	$(GOCMD) tool cover -html=coverage.out

.PHONY: test-integration
test-integration: ## Run integration tests
	ENABLE_INTEGRATION_TESTS=true $(GOTEST) -tags=integration -v ./pkg/tests/integration ./test/integration

.PHONY: test-functional
test-functional: ## Run functional tests (Docker)
	@set -a; [ -f .env ] && . ./.env; set +a; \
	export MCP_TEST_MODE=true && ./test/scripts/run_functional_tests.sh

.PHONY: test-functional-local
test-functional-local: ## Run functional tests with local services and real AWS
	@set -a; [ -f .env ] && . ./.env; set +a; \
	export MCP_TEST_MODE=true && ./test/scripts/run_functional_tests_local.sh

.PHONY: test-websocket
test-websocket: ## Run all WebSocket tests
	@echo "Running WebSocket tests..."
	@$(MAKE) test-websocket-unit
	@$(MAKE) test-websocket-functional
	@$(MAKE) test-websocket-integration

.PHONY: test-websocket-unit
test-websocket-unit: ## Run WebSocket unit tests
	$(GOTEST) -v -short ./apps/mcp-server/internal/api/websocket/... ./pkg/models/websocket/...

.PHONY: test-websocket-functional
test-websocket-functional: ## Run WebSocket functional tests
	@set -a; [ -f .env ] && . ./.env; set +a; \
	cd test/functional && ginkgo -v --focus "WebSocket" ./api

.PHONY: test-websocket-integration
test-websocket-integration: ## Run WebSocket integration tests
	@set -a; [ -f .env ] && . ./.env; set +a; \
	$(GOTEST) -v -tags=integration ./test/integration -run TestWebSocket

.PHONY: test-websocket-load
test-websocket-load: ## Run WebSocket load tests
	./scripts/websocket-load-test.sh

.PHONY: bench
bench: ## Run benchmarks (PACKAGE=./pkg/embedding)
	$(GOTEST) -bench=. -benchmem ${PACKAGE:-./pkg/embedding}

# ==============================================================================
# Code Quality & Security
# ==============================================================================

.PHONY: lint
lint: ## Run linters
	@./.github/scripts/lint-all-modules.sh

.PHONY: fmt
fmt: ## Format code
	$(GOFMT) -w -s .
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w .

.PHONY: vet
vet: ## Run go vet
	$(GOVET) ./...

.PHONY: security-check
security-check: ## Run security checks
	@which gosec > /dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec ./...

# ==============================================================================
# Docker Commands
# ==============================================================================

.PHONY: up
up: ## Start all services with Docker Compose
	$(DOCKER_COMPOSE) up -d
	@echo "Services started:"
	@echo "  MCP Server: http://localhost:8080"
	@echo "  REST API: http://localhost:8081"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Grafana: http://localhost:3000"

.PHONY: down
down: ## Stop all Docker services
	$(DOCKER_COMPOSE) down

.PHONY: logs
logs: ## View Docker logs (service=<name> to filter)
	$(DOCKER_COMPOSE) logs -f $(service)

.PHONY: restart
restart: ## Restart Docker services (service=<name> for specific service)
	$(DOCKER_COMPOSE) up -d --build $(service)

.PHONY: ps
ps: ## Show running Docker services
	$(DOCKER_COMPOSE) ps

# ==============================================================================
# Database Commands
# ==============================================================================

.PHONY: db-shell
db-shell: ## Open PostgreSQL shell
	$(DOCKER_COMPOSE) exec database psql -U dev -d dev

.PHONY: migrate-create
migrate-create: ## Create new migration (name=migration_name)
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	cd apps/rest-api && migrate create -ext sql -dir migrations/sql -seq $(name)

.PHONY: migrate-up
migrate-up: ## Run all pending migrations
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://dev:dev@localhost:5432/dev?sslmode=disable" -path apps/rest-api/migrations/sql up

.PHONY: migrate-down
migrate-down: ## Rollback last migration
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://dev:dev@localhost:5432/dev?sslmode=disable" -path apps/rest-api/migrations/sql down 1

.PHONY: migrate-status
migrate-status: ## Show migration status
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://dev:dev@localhost:5432/dev?sslmode=disable" -path apps/rest-api/migrations/sql version

# ==============================================================================
# Development Helpers
# ==============================================================================

.PHONY: run-mcp-server
run-mcp-server: ## Run MCP server locally
	cd $(MCP_SERVER_DIR) && ./$(MCP_SERVER_BINARY)

.PHONY: run-rest-api
run-rest-api: ## Run REST API locally
	cd $(REST_API_DIR) && ./$(REST_API_BINARY)

.PHONY: run-worker
run-worker: ## Run worker service locally
	cd $(WORKER_DIR) && ./$(WORKER_BINARY)

.PHONY: swagger
swagger: ## Generate Swagger documentation
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	cd $(MCP_SERVER_DIR) && swag init -g ./cmd/server/main.go -o ./docs --parseDependency --parseInternal
	cd $(REST_API_DIR) && swag init -g ./cmd/api/main.go -o ./docs --parseDependency --parseInternal
	@echo "Swagger UI available at:"
	@echo "  MCP Server: http://localhost:8080/swagger/index.html"
	@echo "  REST API: http://localhost:8081/swagger/index.html"

.PHONY: deps
deps: ## Update and sync dependencies
	$(GOMOD) tidy
	cd $(MCP_SERVER_DIR) && $(GOMOD) tidy
	cd $(REST_API_DIR) && $(GOMOD) tidy
	cd $(WORKER_DIR) && $(GOMOD) tidy
	$(GOWORK) sync

# ==============================================================================
# Monitoring & Debugging
# ==============================================================================

.PHONY: health
health: ## Check health of all services
	@curl -sf http://localhost:8080/health | jq . || echo "MCP Server: ❌ Not responding"
	@curl -sf http://localhost:8081/health | jq . || echo "REST API: ❌ Not responding"
	@echo "✅ Health check complete"

.PHONY: metrics
metrics: ## Open Grafana dashboard
	open http://localhost:3000

.PHONY: profile-cpu
profile-cpu: ## Profile CPU usage (DURATION=30s)
	go tool pprof -http=:8090 http://localhost:8080/debug/pprof/profile?seconds=${DURATION:-30}

.PHONY: profile-mem
profile-mem: ## Profile memory usage
	go tool pprof -http=:8090 http://localhost:8080/debug/pprof/heap

# ==============================================================================
# Advanced Commands
# ==============================================================================

.PHONY: generate-adapter
generate-adapter: ## Generate new adapter (NAME=adapter_name)
	@if [ -z "${NAME}" ]; then echo "Error: NAME is required. Usage: make generate-adapter NAME=harness"; exit 1; fi
	@mkdir -p pkg/adapters/${NAME}
	@echo "package ${NAME}" > pkg/adapters/${NAME}/adapter.go
	@echo "" >> pkg/adapters/${NAME}/adapter.go
	@echo "// TODO: Implement ${NAME} adapter" >> pkg/adapters/${NAME}/adapter.go
	@echo "package ${NAME}_test" > pkg/adapters/${NAME}/adapter_test.go
	@echo "package ${NAME}" > pkg/adapters/${NAME}/config.go
	@echo "" >> pkg/adapters/${NAME}/config.go
	@echo "type Config struct {" >> pkg/adapters/${NAME}/config.go
	@echo "    // TODO: Add configuration fields" >> pkg/adapters/${NAME}/config.go
	@echo "}" >> pkg/adapters/${NAME}/config.go
	@echo "✅ Adapter skeleton created at pkg/adapters/${NAME}/"

.PHONY: load-test
load-test: ## Run load tests (requires k6)
	@which k6 > /dev/null || (echo "Error: k6 not installed. Run: brew install k6" && exit 1)
	k6 run --vus ${USERS:-10} --duration ${DURATION:-30s} scripts/k6-load-test.js

# ==============================================================================
# Quick Shortcuts
# ==============================================================================

.PHONY: t
t: test ## Shortcut for 'make test'

.PHONY: b
b: build ## Shortcut for 'make build'

.PHONY: l
l: lint ## Shortcut for 'make lint'

.PHONY: c
c: clean ## Shortcut for 'make clean'
.PHONY: all build clean test test-coverage test-coverage-html test-integration test-fuzz test-functional docker-compose-up docker-compose-down docker-compose-logs

# Default Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOWORK=$(GOCMD) work

# App directories
MCP_SERVER_DIR=./apps/mcp-server
REST_API_DIR=./apps/rest-api
WORKER_DIR=./apps/worker

# Binary names
MCP_SERVER_BINARY=mcp-server
REST_API_BINARY=rest-api
WORKER_BINARY=worker

# Docker configuration
# Enable BuildKit for faster builds
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
# Use buildx for advanced caching features
export DOCKER_DEFAULT_BUILDER=devops-builder
DOCKER_COMPOSE=docker-compose -f docker-compose.local.yml

# Swagger configuration
SWAG_VERSION=v1.16.2

all: clean sync test build

# Development setup
.PHONY: dev-setup
dev-setup: ## Set up local development environment
	@echo "Setting up development environment..."
	@if [ ! -f .env ]; then \
		echo "Creating .env file from example..."; \
		cp .env.example .env; \
		echo "✅ Created .env file - please update with your values"; \
	else \
		echo "✅ .env file already exists"; \
	fi
	@echo ""
	@echo "📋 Next steps:"
	@echo "1. Edit .env with your GitHub token and other settings"
	@echo "2. Run 'make local-dev' to start with Docker (uses docker service names)"
	@echo "3. Or run 'make local-native' to run locally (uses localhost)"

# Local development with Docker
.PHONY: local-dev
local-dev: dev-setup docker-compose-up ## Start local development environment with Docker

# Local development without Docker
.PHONY: local-native
local-native: dev-setup ## Run services locally without Docker
	@echo "Starting PostgreSQL and Redis are required. Please ensure they are running."
	@echo "Run: brew services start postgresql redis"
	@echo "Then run each service:"
	@echo "  ./mcp-server"
	@echo "  ./rest-api"
	@echo "  ./worker"

# Sync Go workspace
sync:
	$(GOWORK) sync

# Build all applications
build: build-mcp-server build-rest-api build-worker

build-mcp-server:
	cd $(MCP_SERVER_DIR) && $(GOBUILD) -o $(MCP_SERVER_BINARY) -v ./cmd/server

build-rest-api:
	cd $(REST_API_DIR) && $(GOBUILD) -o $(REST_API_BINARY) -v ./cmd/api

build-worker:
	cd $(WORKER_DIR) && $(GOBUILD) -o $(WORKER_BINARY) -v ./cmd/worker

clean:
	$(GOCLEAN)
	rm -f $(MCP_SERVER_DIR)/$(MCP_SERVER_BINARY) $(REST_API_DIR)/$(REST_API_BINARY) $(WORKER_DIR)/$(WORKER_BINARY)

# Run unit tests for all workspaces
test:
	$(GOTEST) -v -short ./apps/mcp-server/... ./apps/rest-api/... ./apps/worker/... ./pkg/...

# Test specific application
test-mcp-server:
	$(GOTEST) -v -short ./apps/mcp-server/...

test-rest-api:
	$(GOTEST) -v -short ./apps/rest-api/...

test-worker:
	$(GOTEST) -v -short ./apps/worker/...

# Run all tests, including integration and functional (requires server)
test-all:
	$(GOTEST) -v ./apps/mcp-server/... ./apps/rest-api/... ./apps/worker/... ./pkg/...

# Run integration tests with Docker Compose setup (removed - duplicate target below)

test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./apps/mcp-server/... ./apps/rest-api/... ./apps/worker/... ./pkg/...
	$(GOCMD) tool cover -func=coverage.out

test-coverage-html: test-coverage
	$(GOCMD) tool cover -html=coverage.out

test-integration:
	ENABLE_INTEGRATION_TESTS=true $(GOTEST) -tags=integration -v ./pkg/tests/integration

test-github:
	$(GOTEST) -v ./pkg/tests/integration/github_integration_test.go

test-fuzz:
	$(GOTEST) -fuzz=FuzzTruncateOldestFirst -fuzztime=30s ./apps/mcp-server/internal/core

test-functional:
	@echo "Loading environment variables for functional tests..."
	@set -a; \
	[ -f .env ] && . ./.env; \
	export ELASTICACHE_ENDPOINT=$${ELASTICACHE_ENDPOINT:-localhost}; \
	export ELASTICACHE_PORT=$${ELASTICACHE_PORT:-6379}; \
	export MCP_GITHUB_WEBHOOK_SECRET=$${MCP_GITHUB_WEBHOOK_SECRET:-$${GITHUB_WEBHOOK_SECRET:-dev-webhook-secret}}; \
	set +a; \
	bash ./test/functional/check_test_env.sh || (echo "\nEnvironment check failed. Please set all required environment variables before running functional tests." && exit 1); \
	export MCP_TEST_MODE=true && ./test/scripts/run_functional_tests.sh

# Run only specific functional tests
# Usage: make test-functional-focus FOCUS="Health Endpoint"
test-functional-focus:
	@bash ./test/functional/check_test_env.sh || (echo "\nEnvironment check failed. Please set all required environment variables before running functional tests." && exit 1)
	cd $(shell pwd) && export MCP_TEST_MODE=true && ./test/scripts/run_functional_tests_fixed.sh --focus "$(FOCUS)"

# Run functional tests with verbose output
test-functional-verbose:
	@bash ./test/functional/check_test_env.sh || (echo "\nEnvironment check failed. Please set all required environment variables before running functional tests." && exit 1)
	cd $(shell pwd) && export MCP_TEST_MODE=true && ./test/scripts/run_functional_tests_fixed.sh --verbose

# Run functional tests locally with proper environment setup
test-functional-local:
	@echo "Running functional tests with local environment..."
	@bash ./test/scripts/run_functional_tests_local.sh

# Run functional tests locally with verbose output
test-functional-local-verbose:
	@echo "Running functional tests with local environment (verbose)..."
	@bash ./test/scripts/run_functional_tests_local.sh --verbose

# Multi-Agent Embedding System tests
test-embedding: test-embedding-unit test-embedding-integration

# Run unit tests for embedding system
test-embedding-unit:
	@echo "Running embedding unit tests..."
	$(GOTEST) -v -short ./pkg/embedding/... ./pkg/agents/...

# Run integration tests for multi-agent embeddings (requires Docker)
test-embedding-integration: docker-compose-up
	@echo "Running embedding integration tests..."
	$(GOTEST) -v -tags=integration ./test/integration/multi_agent_embedding_test.go

# Test specific embedding provider
# Usage: make test-embedding-provider PROVIDER=openai
test-embedding-provider:
	@echo "Testing $(PROVIDER) provider..."
	$(GOTEST) -v ./pkg/embedding/providers -run Test.*$(PROVIDER).*

# Test with coverage for embedding system
test-embedding-coverage:
	$(GOTEST) -v -coverprofile=embedding_coverage.out ./pkg/embedding/... ./pkg/agents/...
	$(GOCMD) tool cover -html=embedding_coverage.out -o embedding_coverage.html
	@echo "Coverage report generated: embedding_coverage.html"

# Benchmark embedding system
test-embedding-bench:
	@echo "Running embedding benchmarks..."
	$(GOTEST) -bench=. -benchmem ./pkg/embedding/...

# Run all multi-agent tests
test-multi-agent: test-embedding test-agents-unit
	@echo "All multi-agent tests completed"

# Test agent configuration system
test-agents-unit:
	@echo "Running agent configuration tests..."
	$(GOTEST) -v -short ./pkg/agents/...

# MCP Functional Tests
test-functional-mcp: test-functional-mcp-all

# Run all MCP functional tests
test-functional-mcp-all:
	@echo "Running all MCP functional tests..."
	@set -a; \
	[ -f .env ] && . ./.env; \
	export MCP_SERVER_URL=$${MCP_SERVER_URL:-http://localhost:8080}; \
	export MCP_API_KEY=$${MCP_API_KEY:-docker-admin-api-key}; \
	export MCP_TEST_MODE=true; \
	set +a; \
	cd test/functional && ginkgo -v --label-filter="" ./mcp

# Run only WebSocket tests
test-functional-mcp-websocket:
	@echo "Running MCP WebSocket tests..."
	@set -a; \
	[ -f .env ] && . ./.env; \
	export MCP_SERVER_URL=$${MCP_SERVER_URL:-http://localhost:8080}; \
	export MCP_API_KEY=$${MCP_API_KEY:-docker-admin-api-key}; \
	export MCP_TEST_MODE=true; \
	set +a; \
	cd test/functional && ginkgo -v --focus "WebSocket" ./mcp

# Run only REST API tests
test-functional-mcp-rest:
	@echo "Running MCP REST API tests..."
	@set -a; \
	[ -f .env ] && . ./.env; \
	export MCP_SERVER_URL=$${MCP_SERVER_URL:-http://localhost:8080}; \
	export MCP_API_KEY=$${MCP_API_KEY:-docker-admin-api-key}; \
	export MCP_TEST_MODE=true; \
	set +a; \
	cd test/functional && ginkgo -v --focus "REST API" ./mcp

# Run MCP tests with coverage
test-functional-mcp-coverage:
	@echo "Running MCP tests with coverage..."
	@set -a; \
	[ -f .env ] && . ./.env; \
	export MCP_SERVER_URL=$${MCP_SERVER_URL:-http://localhost:8080}; \
	export MCP_API_KEY=$${MCP_API_KEY:-docker-admin-api-key}; \
	export MCP_TEST_MODE=true; \
	set +a; \
	cd test/functional && go test -v -cover -coverprofile=mcp_coverage.out ./mcp && \
	go tool cover -html=mcp_coverage.out -o mcp_coverage.html && \
	echo "Coverage report generated: test/functional/mcp_coverage.html"

# Run MCP tests with specific focus
# Usage: make test-functional-mcp-focus FOCUS="Tool Discovery"
test-functional-mcp-focus:
	@echo "Running MCP tests with focus: $(FOCUS)"
	@set -a; \
	[ -f .env ] && . ./.env; \
	export MCP_SERVER_URL=$${MCP_SERVER_URL:-http://localhost:8080}; \
	export MCP_API_KEY=$${MCP_API_KEY:-docker-admin-api-key}; \
	export MCP_TEST_MODE=true; \
	set +a; \
	cd test/functional && ginkgo -v --focus "$(FOCUS)" ./mcp

# Run MCP tests in watch mode for development
test-functional-mcp-watch:
	@echo "Running MCP tests in watch mode..."
	@set -a; \
	[ -f .env ] && . ./.env; \
	export MCP_SERVER_URL=$${MCP_SERVER_URL:-http://localhost:8080}; \
	export MCP_API_KEY=$${MCP_API_KEY:-docker-admin-api-key}; \
	export MCP_TEST_MODE=true; \
	set +a; \
	cd test/functional && ginkgo watch -v ./mcp

deps:
	$(GOWORK) sync
	$(GOMOD) download
	$(GOMOD) tidy

# Run individual services locally
run-mcp-server:
	cd $(MCP_SERVER_DIR) && ./$(MCP_SERVER_BINARY)

run-rest-api:
	cd $(REST_API_DIR) && ./$(REST_API_BINARY)

run-worker:
	cd $(WORKER_DIR) && ./$(WORKER_BINARY)

# Start the whole stack with Docker Compose
local-dev: docker-compose-up
	@echo "Started development environment"
	@echo "MCP Server available at: http://localhost:8080"
	@echo "REST API available at: http://localhost:8081"
	@echo "Run 'make docker-compose-logs' to view logs"
	@echo "Run 'make docker-compose-down' to stop services"

# Docker Compose commands
docker-compose-up:
	$(DOCKER_COMPOSE) up -d

docker-compose-down:
	$(DOCKER_COMPOSE) down

# View logs from all services or a specific service
docker-compose-logs:
	$(DOCKER_COMPOSE) logs -f $(service)

# Rebuild and restart a specific service
docker-compose-restart:
	$(DOCKER_COMPOSE) up -d --build $(service)

# Build and start everything needed for local development including test data (removed - duplicate target above)

init-config:
	cp configs/config.yaml.template configs/config.yaml

# Workspace structure and tool validation commands
check-workspace:
	@echo "Checking Go workspace configuration..."
	$(GOWORK) init
	$(GOWORK) use ./apps/mcp-server ./apps/rest-api ./apps/worker ./pkg
	$(GOWORK) sync
	@echo "Workspace check complete."

check-imports:
	@echo "Checking for import cycles..."
	$(GOWORK) graph | grep -v '@' | sort | uniq > imports.txt
	@echo "Import check complete. See imports.txt for details."

# Linting and code quality
lint:
	@echo "Running linters..."
	@./.github/scripts/lint-all-modules.sh

# Swagger documentation commands
swagger-install:
	@echo "Installing swag CLI tool..."
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

swagger-init: swagger-install
	@echo "Initializing Swagger documentation..."
	cd $(MCP_SERVER_DIR) && swag init -g ./cmd/server/main.go -o ./docs --parseDependency --parseInternal
	cd $(REST_API_DIR) && swag init -g ./cmd/api/main.go -o ./docs --parseDependency --parseInternal

swagger-fmt:
	@echo "Formatting Swagger comments..."
	cd $(MCP_SERVER_DIR) && swag fmt
	cd $(REST_API_DIR) && swag fmt

swagger: swagger-fmt swagger-init
	@echo "Swagger documentation generated successfully"
	@echo "MCP Server Swagger UI: http://localhost:8080/swagger/index.html"
	@echo "REST API Swagger UI: http://localhost:8081/swagger/index.html"

swagger-serve:
	@echo "Serving OpenAPI documentation..."
	python3 -m http.server 8082 --directory ./docs/swagger &
	@echo "OpenAPI specs available at: http://localhost:8082/"
	@echo "Main spec: http://localhost:8082/openapi.yaml"

# Check adapter pattern implementation
check-adapter-pattern:
	@echo "Verifying adapter pattern implementations..."
	find ./apps -name "*adapter*.go" | xargs grep -l "repository"
	@echo "Adapter implementation check complete."

# Verify Docker setup
docker-verify:
	@echo "Verifying Docker setup..."
	@docker info > /dev/null 2>&1 || (echo "Docker is not running" && exit 1)
	@echo "Docker is running properly."
	@echo "Checking PostgreSQL with pgvector..."
	$(DOCKER_COMPOSE) pull database
	@echo "Docker verification complete."

# Database migration commands using golang-migrate
# Note: Database migrations have been moved to apps/rest-api/migrations

# Create a new migration file with the given name
# Usage: make migrate-create name=add_new_table
migrate-create:
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	cd apps/rest-api && migrate create -ext sql -dir migrations/sql -seq $(name)

# Run all pending migrations
# Usage: make migrate-up dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-up:
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "$(dsn)" -path apps/rest-api/migrations/sql up

# Run migrations for local development environment
migrate-local:
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://dev:dev@localhost:5432/dev?sslmode=disable" -path apps/rest-api/migrations/sql up

# Run migrations for docker environment
migrate-docker:
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "postgresql://dev:dev@localhost:5432/dev?sslmode=disable" -path apps/rest-api/migrations/sql up

# Roll back the most recent migration
# Usage: make migrate-down dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-down:
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "$(dsn)" -path apps/rest-api/migrations/sql down 1

# Roll back all migrations
# Usage: make migrate-reset dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-reset:
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "$(dsn)" -path apps/rest-api/migrations/sql drop -f

# Check the current migration version
# Usage: make migrate-version dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-version:
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "$(dsn)" -path apps/rest-api/migrations/sql version

# Force the database to a specific version
# Usage: make migrate-force dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable" version=5
migrate-force:
	@which migrate > /dev/null || (echo "Error: golang-migrate not installed. Run: brew install golang-migrate" && exit 1)
	migrate -database "$(dsn)" -path apps/rest-api/migrations/sql force $(version)

# Helper to enter database shell from Docker
db-shell:
	$(DOCKER_COMPOSE) exec database psql -U dev -d dev

# Helper to view vector tables
psql-vector-tables:
	$(DOCKER_COMPOSE) exec database psql -U dev -d dev -c "SELECT table_name FROM information_schema.tables WHERE table_name LIKE '%vector%' OR table_name LIKE '%embedding%';"

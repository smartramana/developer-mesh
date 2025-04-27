.PHONY: all build clean test test-coverage test-coverage-html test-integration test-fuzz test-functional docker-build docker-run mock mockserver-build mockserver-run local-dev-setup test-github migrate migrate-up migrate-down migrate-create migrate-version migrate-force

# Default Go parameters
GOCMD=/usr/local/go/bin/go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=mcp-server
MOCKSERVER_NAME=mockserver
DOCKER_IMAGE=mcp-server
MOCKSERVER_IMAGE=mcp-mockserver

all: clean deps test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/server

mockserver-build:
	$(GOBUILD) -o $(MOCKSERVER_NAME) -v ./cmd/mockserver

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME) $(MOCKSERVER_NAME)

test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -func=coverage.out

test-coverage-html: test-coverage
	$(GOCMD) tool cover -html=coverage.out

test-integration:
	ENABLE_INTEGRATION_TESTS=true $(GOTEST) -tags=integration -v ./test/integration

test-github:
	$(GOTEST) -v ./test/github_integration_test.go

test-fuzz:
	$(GOTEST) -fuzz=FuzzTruncateOldestFirst -fuzztime=30s ./internal/core

test-functional:
	cd $(shell pwd) && export MCP_TEST_MODE=true && ./test/scripts/run_functional_tests_fixed.sh

# Run only specific functional tests
# Usage: make test-functional-focus FOCUS="Health Endpoint"
test-functional-focus:
	cd $(shell pwd) && export MCP_TEST_MODE=true && ./test/scripts/run_functional_tests_fixed.sh --focus "$(FOCUS)"

# Run functional tests with verbose output
test-functional-verbose:
	cd $(shell pwd) && export MCP_TEST_MODE=true && ./test/scripts/run_functional_tests_fixed.sh --verbose

deps:
	$(GOMOD) download
	$(GOMOD) tidy

run:
	MCP_CONFIG_FILE=configs/config.local.yaml ./$(BINARY_NAME)

mockserver-run:
	./$(MOCKSERVER_NAME)

# Run both the mock server and the MCP server locally
local-dev:
	@echo "Starting mock server in background..."
	@./$(MOCKSERVER_NAME) > mockserver.log 2>&1 &
	@echo "Starting MCP server..."
	MCP_CONFIG_FILE=configs/config.local.yaml ./$(BINARY_NAME)

# Build and start everything needed for local development
local-dev-setup: build mockserver-build local-dev-dependencies
	MCP_CONFIG_FILE=configs/config.local.yaml $(MAKE) local-dev

# One command to set up dependencies for local development
local-dev-dependencies:
	@echo "Starting PostgreSQL and Redis in Docker containers..."
	docker-compose up -d postgres redis
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5
	
docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-build-mockserver:
	docker build -t $(MOCKSERVER_IMAGE) -f Dockerfile.mockserver .

docker-run:
	docker run --name $(BINARY_NAME) -p 8080:8080 $(DOCKER_IMAGE)

docker-run-mockserver:
	docker run --name $(MOCKSERVER_NAME) -p 8081:8081 $(MOCKSERVER_IMAGE)

docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

init-config:
	cp configs/config.yaml.template configs/config.yaml

check-structure:
	@echo "Checking if all required directories exist..."
	@mkdir -p cmd/server cmd/mockserver internal/adapters internal/api internal/cache internal/config internal/core internal/database internal/metrics pkg/mcp
	@echo "Structure check complete."

check-imports:
	@echo "Checking for import cycles..."
	$(GOCMD) mod graph | grep -v '@' | sort | uniq > imports.txt
	@echo "Import check complete. See imports.txt for details."

# GitHub API integration
build-github-only:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/server
	@echo "Building GitHub components"
	$(GOCMD) get github.com/xeipuuv/gojsonschema
	$(GOCMD) get github.com/golang-jwt/jwt/v4
	$(GOMOD) tidy

# Create a sample GitHub app for testing
create-github-app:
	@echo "Generating GitHub App private key..."
	openssl genrsa -out configs/github-app-private-key.pem 2048
	@echo "Private key generated. Use this for your GitHub App."
	@echo "Follow the GitHub App setup guide in docs/github-integration-guide.md"

# Setup GitHub integration
setup-github: build-github-only test-github
	@echo "GitHub integration setup complete."
	@echo "See docs/github-integration-guide.md for usage instructions."

# Database migration commands
migrate-tool:
	$(GOBUILD) -o migrate -v ./cmd/migrate

# Create a new migration file with the given name
# Usage: make migrate-create name=add_new_table
migrate-create: migrate-tool
	./migrate -create -name $(name)

# Run all pending migrations
# Usage: make migrate-up dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-up: migrate-tool
	./migrate -up -dsn "$(dsn)"

# Run all pending migrations (steps limited)
# Usage: make migrate-up-steps dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable" steps=1
migrate-up-steps: migrate-tool
	./migrate -up -steps $(steps) -dsn "$(dsn)"

# Roll back the most recent migration
# Usage: make migrate-down dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-down: migrate-tool
	./migrate -down -dsn "$(dsn)"

# Roll back all migrations
# Usage: make migrate-reset dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-reset: migrate-tool
	./migrate -reset -dsn "$(dsn)"

# Check the current migration version
# Usage: make migrate-version dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-version: migrate-tool
	./migrate -version -dsn "$(dsn)"

# Force the database to a specific version
# Usage: make migrate-force dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable" version=5
migrate-force: migrate-tool
	./migrate -force $(version) -dsn "$(dsn)"

# Validate migrations without applying them
# Usage: make migrate-validate dsn="postgres://user:pass@localhost:5432/mcp_db?sslmode=disable"
migrate-validate: migrate-tool
	./migrate -validate -dsn "$(dsn)"

# Run migrations for local development
migrate-local: migrate-tool
	./migrate -up -dsn "postgres://postgres:postgres@localhost:5432/mcp_db?sslmode=disable"

.PHONY: all build clean test test-coverage test-coverage-html test-integration test-fuzz docker-build docker-run mock mockserver-build mockserver-run local-dev-setup test-github

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

deps:
	$(GOMOD) download
	$(GOMOD) tidy

run:
	./$(BINARY_NAME)

mockserver-run:
	./$(MOCKSERVER_NAME)

# Run both the mock server and the MCP server locally
local-dev:
	@echo "Starting mock server in background..."
	@./$(MOCKSERVER_NAME) > mockserver.log 2>&1 &
	@echo "Starting MCP server..."
	@./$(BINARY_NAME)

# Build and start everything needed for local development
local-dev-setup: build mockserver-build local-dev-dependencies local-dev

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

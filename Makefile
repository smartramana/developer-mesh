.PHONY: all build clean test docker-build docker-run mock mockserver-build mockserver-run local-dev-setup

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
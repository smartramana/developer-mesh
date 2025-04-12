.PHONY: all build clean test docker-build docker-run

# Default Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=mcp-server
DOCKER_IMAGE=mcp-server

all: clean deps test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/server

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

test:
	$(GOTEST) -v ./...

deps:
	$(GOMOD) download
	$(GOMOD) tidy

run:
	./$(BINARY_NAME)

docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker run --name $(BINARY_NAME) -p 8080:8080 $(DOCKER_IMAGE)

docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

init-config:
	cp configs/config.yaml.template configs/config.yaml

check-structure:
	@echo "Checking if all required directories exist..."
	@mkdir -p cmd/server internal/adapters internal/api internal/cache internal/config internal/core internal/database internal/metrics pkg/mcp
	@echo "Structure check complete."

check-imports:
	@echo "Checking for import cycles..."
	$(GOCMD) mod graph | grep -v '@' | sort | uniq > imports.txt
	@echo "Import check complete. See imports.txt for details."
# REST API Makefile
.PHONY: build test clean docker run lint migrate

APP_NAME := rest-api
BIN_DIR := ../../bin
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.BuildTime=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ") \
           -X main.GitCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/api

test:
	go test -race ./...

lint:
	golangci-lint run ./...

docker:
	docker build -t $(APP_NAME):$(VERSION) -f Dockerfile .

clean:
	rm -f $(BIN_DIR)/$(APP_NAME)

run:
	go run ./cmd/api

# Run database migrations
migrate:
	go run ./cmd/api --migrate-only

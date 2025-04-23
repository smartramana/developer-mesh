#!/bin/sh
# Script to verify GitHub integration fixes

set -e

echo "Verifying Go module..."
/usr/local/go/bin/go mod tidy
/usr/local/go/bin/go mod verify

echo "Checking for circular dependencies..."
/usr/local/go/bin/go list -f '{{.ImportPath}} -> {{.Error}}' ./... | grep "import cycle" || echo "No circular dependencies found!"

echo "Compiling the code..."
/usr/local/go/bin/go build -o /tmp/mcp-server ./cmd/server

echo "Running tests for GitHub integration..."
/usr/local/go/bin/go test ./internal/adapters/github/... -v

echo "All checks completed!"

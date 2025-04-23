#!/bin/bash

# fixed_github_test.sh - Script to test GitHub integration focusing specifically on that aspect
# This version directly sets the PATH environment variable to include Go

set -e

# Define paths explicitly
GO_PATH="/usr/local/go/bin/go"
GINKGO_PATH="/Users/seancorkum/go/bin/ginkgo"

# Set current directory to project root
cd "$(dirname "$0")"
PROJECT_ROOT=$(pwd)

echo "Starting GitHub integration tests from $PROJECT_ROOT"

# Set PATH to include Go binaries
export PATH="/usr/local/go/bin:$PATH"

echo "Using PATH: $PATH"
echo "Go version: $(${GO_PATH} version)"
echo "Ginkgo version: $(${GINKGO_PATH} version)"

# First run the direct GitHub integration test
echo "Running GitHub adapter tests..."
${GO_PATH} test ./internal/adapters/github/... -v

# Test the error package to verify fixes
echo "Testing GitHub error handling..."
${GO_PATH} test ./internal/adapters/errors/... -v

# Start Docker containers for a more limited test focusing only on GitHub
echo "Setting up minimal test environment (this may take a minute)..."
docker-compose -f docker-compose.test.yml down -v --remove-orphans
docker-compose -f docker-compose.test.yml up -d

# Wait for services to be healthy
echo "Waiting for services to be ready..."
sleep 10  # Give services some time to start

# Set environment variables for the tests
export MCP_SERVER_URL="http://localhost:8080"
export MCP_API_KEY="test-admin-api-key"
export MOCKSERVER_URL="http://localhost:8081"

# Use explicit path for testing
echo "Running functional tests focused on GitHub integration..."
${GO_PATH} test ./test/functional/integrations/tool_integrations_test.go -v -run TestGitHub

# Clean up
docker-compose -f docker-compose.test.yml down -v

echo "GitHub integration tests completed."

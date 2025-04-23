#!/bin/bash

# github_ginkgo_test.sh - Script to test GitHub integration using Ginkgo directly
# This version uses Ginkgo to run the tests with proper focus

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

# Start Docker containers for the test environment
echo "Setting up test environment (this may take a minute)..."
docker-compose -f docker-compose.test.yml down -v --remove-orphans
docker-compose -f docker-compose.test.yml up -d

# Wait for services to be healthy
echo "Waiting for services to be ready..."
sleep 10  # Give services some time to start

# Set environment variables for the tests
export MCP_SERVER_URL="http://localhost:8080"
export MCP_API_KEY="test-admin-api-key"
export MOCKSERVER_URL="http://localhost:8081"

# Run the tests focused on GitHub integration using Ginkgo directly
echo "Running functional tests focused on GitHub integration..."
cd test/functional
${GINKGO_PATH} --focus="GitHub" -v

# Clean up
cd $PROJECT_ROOT
docker-compose -f docker-compose.test.yml down -v

echo "GitHub integration tests completed."

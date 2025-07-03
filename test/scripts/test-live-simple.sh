#!/bin/bash

# Simple script to test live endpoints without workspace issues

set -e

echo "Testing live endpoints..."

# Set environment for live endpoints
export MCP_WEBSOCKET_URL="wss://mcp.dev-mesh.io/ws"
export REST_API_URL="https://api.dev-mesh.io"
export MCP_SERVER_URL="https://mcp.dev-mesh.io"
export MCP_API_KEY="dev-admin-key-1234567890"
export ADMIN_API_KEY="dev-admin-key-1234567890"
export SKIP_MOCK_TESTS=true
export TEST_ENV="production"

# Additional test env vars
export GITHUB_TOKEN="test-github-token"
export GITHUB_REPO="test-repo"
export GITHUB_OWNER="test-org"

# Change to functional test directory
cd test/functional

# Run a simple health check test
echo "Running health check tests..."
go test -v -run TestHealthCheck ./api -count=1

echo "Done!"
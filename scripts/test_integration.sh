#!/bin/bash

# Test Script for MCP Server Integration

set -e  # Exit on error

# Build the server
echo "Building MCP Server..."
go build -o mcp-server ./cmd/server

# Check if PostgreSQL is running using pg_isready
echo "Checking PostgreSQL connection..."
if command -v pg_isready &> /dev/null; then
    if ! pg_isready -h localhost -p 5432; then
        echo "PostgreSQL is not running. Starting using Docker..."
        docker run --name mcp-postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_USER=postgres -e POSTGRES_DB=mcp -p 5432:5432 -d postgres:14
        # Wait for PostgreSQL to start
        sleep 5
    else
        echo "PostgreSQL is already running."
    fi
else
    echo "pg_isready not found. Assuming PostgreSQL is running."
fi

# Check if Redis is running
echo "Checking Redis connection..."
if command -v redis-cli &> /dev/null; then
    if ! redis-cli ping &> /dev/null; then
        echo "Redis is not running. Starting using Docker..."
        docker run --name mcp-redis -p 6379:6379 -d redis:7
        # Wait for Redis to start
        sleep 2
    else
        echo "Redis is already running."
    fi
else
    echo "redis-cli not found. Assuming Redis is running."
fi

# Start the server in the background
echo "Starting MCP Server..."
export MCP_DATABASE_DSN="postgres://postgres:postgres@localhost:5432/mcp?sslmode=disable"
export MCP_CACHE_ADDRESS="localhost:6379"
export MCP_API_LISTEN_ADDRESS=":8080"
export MCP_AUTH_JWT_SECRET="test-jwt-secret"
export MCP_AUTH_API_KEYS_ADMIN="test-api-key"
export MCP_AGENT_WEBHOOK_SECRET="test-webhook-secret"

# Run in background
./mcp-server &
SERVER_PID=$!

# Wait for server to start
echo "Waiting for server to start..."
sleep 5

# Run integration tests
echo "Running integration tests..."
export ENABLE_INTEGRATION_TESTS="true"
export MCP_SERVER_ADDR="http://localhost:8080"
export MCP_API_KEY="test-api-key"
export MCP_WEBHOOK_SECRET="test-webhook-secret"

go test -v ./test/integration/...

# Clean up
echo "Cleaning up..."
kill $SERVER_PID

echo "Tests completed successfully!"

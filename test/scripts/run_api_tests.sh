#!/bin/bash

# This script runs a simplified version of the functional tests for the MCP server
# focusing on the API tests only to verify basic functionality

set -e  # Exit on error

# Set current directory to project root
cd "$(dirname "$0")"
PROJECT_ROOT=$(pwd)

echo "Starting API tests from $PROJECT_ROOT"

# Set specific path to Go
GO_PATH="/usr/local/go/bin/go"

# Check if Go is installed
if ! command -v $GO_PATH &> /dev/null; then
    echo "Error: Go is not installed at $GO_PATH"
    exit 1
fi

echo "Running individual API tests..."

# Run specific healthcheck test to verify the API is working
$GO_PATH test ./internal/api/... -v -run=TestHealthHandler

echo "API tests completed successfully!"

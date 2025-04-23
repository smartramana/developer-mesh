#!/bin/bash

# This script runs the fixed GitHub adapter tests

set -e  # Exit on error

# Set current directory to project root
cd "$(dirname "$0")"
PROJECT_ROOT=$(pwd)

echo "Running GitHub adapter tests from $PROJECT_ROOT"

# Set specific path to Go
GO_PATH="/usr/local/go/bin/go"

# Check if Go is installed
if ! command -v $GO_PATH &> /dev/null; then
    echo "Error: Go is not installed at $GO_PATH"
    exit 1
fi

# Run the GitHub adapter tests
echo "Running GitHub adapter tests..."

$GO_PATH test ./internal/adapters/providers/github -v

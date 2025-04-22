#!/bin/bash

# run_functional_tests.sh - Script to run functional tests for MCP server
# Usage: ./run_functional_tests.sh [options]
#   Options:
#     -v, --verbose     Show verbose output
#     -k, --keep-up     Keep containers up after tests finish
#     -f, --focus TEXT  Run only specs matching the TEXT

set -e  # Exit on error

# Parse command line arguments
VERBOSE=0
KEEP_UP=0
FOCUS=""

while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -v|--verbose)
      VERBOSE=1
      shift
      ;;
    -k|--keep-up)
      KEEP_UP=1
      shift
      ;;
    -f|--focus)
      FOCUS="$2"
      shift
      shift
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Set current directory to project root
cd "$(dirname "$0")"
PROJECT_ROOT=$(pwd)

echo "Starting functional tests from $PROJECT_ROOT"

# Set specific path to Go
GO_PATH="/usr/local/go/bin/go"

# Check if Go is installed
if ! command -v $GO_PATH &> /dev/null; then
    echo "Error: Go is not installed at $GO_PATH"
    exit 1
fi

# Check if Ginkgo is installed
if ! command -v ginkgo &> /dev/null; then
    echo "Installing Ginkgo..."
    $GO_PATH install github.com/onsi/ginkgo/v2/ginkgo@latest
    export PATH=$PATH:$HOME/go/bin
    if ! command -v ginkgo &> /dev/null; then
        echo "Error: Failed to install Ginkgo. Please install it manually:"
        echo "    go install github.com/onsi/ginkgo/v2/ginkgo@latest"
        exit 1
    fi
fi

# Function to clean up resources
cleanup() {
    if [ $KEEP_UP -eq 0 ]; then
        echo "Cleaning up test environment..."
        docker-compose -f docker-compose.test.yml down -v
    else
        echo "Keeping containers up as requested. Use the following command to stop them:"
        echo "    docker-compose -f docker-compose.test.yml down -v"
    fi
}

# Function to check if service is healthy
check_health() {
    local service=$1
    local max_attempts=$2
    local attempt=1
    
    echo "Checking health of $service..."
    while [ $attempt -le $max_attempts ]; do
        if docker-compose -f docker-compose.test.yml exec $service wget -q -O- http://localhost:8080/health > /dev/null 2>&1; then
            echo "$service is healthy!"
            return 0
        fi
        echo "Waiting for $service to be ready (attempt $attempt/$max_attempts)..."
        sleep 3
        attempt=$((attempt+1))
    done
    
    echo "Error: $service is not healthy after $max_attempts attempts"
    return 1
}

# Register cleanup function to be called on script exit
trap cleanup EXIT

# Start the test environment
echo "Starting test environment with Docker Compose..."
docker-compose -f docker-compose.test.yml down -v --remove-orphans
docker-compose -f docker-compose.test.yml build
docker-compose -f docker-compose.test.yml up -d

# Wait for all services to be healthy
echo "Waiting for all services to be healthy..."
sleep 5  # Initial wait for services to start

# Check PostgreSQL
echo "Checking PostgreSQL..."
max_attempts=20
attempt=1
while [ $attempt -le $max_attempts ]; do
    if docker-compose -f docker-compose.test.yml exec postgres pg_isready -U postgres > /dev/null 2>&1; then
        echo "PostgreSQL is ready!"
        break
    fi
    echo "Waiting for PostgreSQL to be ready (attempt $attempt/$max_attempts)..."
    sleep 3
    attempt=$((attempt+1))
done

if [ $attempt -gt $max_attempts ]; then
    echo "Error: PostgreSQL is not ready after $max_attempts attempts"
    exit 1
fi

# Check MCP Server
echo "Checking MCP Server..."
max_attempts=20
attempt=1
while [ $attempt -le $max_attempts ]; do
    if docker-compose -f docker-compose.test.yml exec -T mcp-server wget -q -O- http://localhost:8080/health > /dev/null 2>&1; then
        echo "MCP Server is ready!"
        break
    fi
    echo "Waiting for MCP Server to be ready (attempt $attempt/$max_attempts)..."
    sleep 3
    attempt=$((attempt+1))
done

if [ $attempt -gt $max_attempts ]; then
    echo "Error: MCP Server is not ready after $max_attempts attempts"
    exit 1
fi

# Set environment variables for the tests
export MCP_SERVER_URL="http://localhost:8080"
export MCP_API_KEY="test-admin-api-key"
export MOCKSERVER_URL="http://localhost:8081"

# Run the functional tests
echo "Running functional tests..."
if [ -n "$FOCUS" ]; then
    echo "Focusing on tests matching: $FOCUS"
    focus_arg="--focus=$FOCUS"
else
    focus_arg=""
fi

if [ $VERBOSE -eq 1 ]; then
    ginkgo -v $focus_arg --randomize-all ./test/functional/...
else
    ginkgo $focus_arg --randomize-all ./test/functional/...
fi

echo "Functional tests completed!"

#!/bin/bash

# run_functional_tests_fixed.sh - Fixed script to run functional tests with explicit Go path
# Usage: ./run_functional_tests_fixed.sh [options]
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

# Get the script directory
SCRIPT_DIR="$(dirname "$0")"

# Navigate to project root (two directories up from script location)
cd "$SCRIPT_DIR/../.."
PROJECT_ROOT=$(pwd)

echo "Starting functional tests from $PROJECT_ROOT"

# Explicitly set Go path
GO_PATH="/usr/local/go/bin/go"
GINKGO_PATH="/Users/seancorkum/go/bin/ginkgo"

# Verify that the path to Go exists
if [ ! -f "$GO_PATH" ]; then
    echo "Error: Go executable not found at $GO_PATH"
    exit 1
fi

# Verify that the path to Ginkgo exists
if [ ! -f "$GINKGO_PATH" ]; then
    echo "Error: Ginkgo executable not found at $GINKGO_PATH"
    exit 1
fi

echo "Using Go at: $GO_PATH"
echo "Using Ginkgo at: $GINKGO_PATH"

# Function to clean up resources
cleanup() {
    if [ $KEEP_UP -eq 0 ]; then
        echo "Cleaning up test environment..."
        docker-compose -f $PROJECT_ROOT/docker-compose.test.yml down -v
    else
        echo "Keeping containers up as requested. Use the following command to stop them:"
        echo "    docker-compose -f docker-compose.test.yml down -v"
    fi
}

# Register cleanup function to be called on script exit
trap cleanup EXIT

# Start the test environment
echo "Starting test environment with Docker Compose..."
docker-compose -f $PROJECT_ROOT/docker-compose.test.yml down -v --remove-orphans
docker-compose -f $PROJECT_ROOT/docker-compose.test.yml build
docker-compose -f $PROJECT_ROOT/docker-compose.test.yml up -d

# Wait for all services to be healthy
echo "Waiting for all services to be healthy..."
sleep 5  # Initial wait for services to start

# Check PostgreSQL
echo "Checking PostgreSQL..."
max_attempts=20
attempt=1
while [ $attempt -le $max_attempts ]; do
    if docker-compose -f $PROJECT_ROOT/docker-compose.test.yml exec postgres pg_isready -U postgres > /dev/null 2>&1; then
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
    if docker-compose -f $PROJECT_ROOT/docker-compose.test.yml exec -T mcp-server wget -q -O- http://localhost:8080/health > /dev/null 2>&1; then
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

# Run the functional tests on the host machine instead of inside Docker
echo "Running functional tests using host Go installation..."
if [ -n "$FOCUS" ]; then
    echo "Focusing on tests matching: $FOCUS"
    focus_arg="--focus=$FOCUS"
else
    focus_arg=""
fi

# First run unit tests to make sure everything is compiled
echo "Running unit tests first..."
$GO_PATH test ./internal/adapters/github/... -run=TestUnit

echo "Running Ginkgo functional tests..."
if [ $VERBOSE -eq 1 ]; then
    $GINKGO_PATH -v $focus_arg --randomize-all --race ./test/functional/...
else
    $GINKGO_PATH $focus_arg --randomize-all --race ./test/functional/...
fi

echo "Functional tests completed successfully!"

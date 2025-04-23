#!/bin/bash

# This script will run the functional tests inside the Docker container
# Set current directory to project root
cd "$(dirname "$0")"
PROJECT_ROOT=$(pwd)

echo "Starting functional tests in Docker container from $PROJECT_ROOT"

# Start the Docker compose environment
docker-compose -f docker-compose.test.yml up -d

# Wait for services to be healthy
echo "Waiting for services to be healthy..."
sleep 10

# Execute tests inside the mockserver container which has Go installed
echo "Running tests inside container..."
docker-compose -f docker-compose.test.yml exec -T mockserver sh -c "cd /app && go test -v ./test/functional/..."

# Cleanup
echo "Cleaning up test environment..."
docker-compose -f docker-compose.test.yml down -v

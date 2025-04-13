#!/bin/bash

# Set up the local development environment for MCP Server
set -e

# Show what we're doing
echo "Setting up local development environment for MCP Server..."

# Ensure configuration exists
echo "Ensuring configuration is in place..."
if [ ! -f "configs/config.yaml" ]; then
    echo "Creating config.yaml from template..."
    cp configs/config.yaml.template configs/config.yaml
fi

# Build the applications
echo "Building applications..."
go build -o mcp-server ./cmd/server
go build -o mockserver ./cmd/mockserver

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "Docker is not running or not installed. Please start Docker."
    exit 1
fi

# Start required services with Docker Compose
echo "Starting PostgreSQL and Redis with Docker Compose..."
docker-compose up -d postgres redis

# Wait for PostgreSQL to be ready
echo "Waiting for PostgreSQL to be ready..."
for i in {1..10}; do
    if docker-compose exec postgres pg_isready -U postgres > /dev/null 2>&1; then
        echo "PostgreSQL is ready!"
        break
    fi
    echo "Waiting for PostgreSQL to start... ($i/10)"
    sleep 3
    
    if [ $i -eq 10 ]; then
        echo "Failed to connect to PostgreSQL after multiple attempts."
        echo "Please check the Docker logs:"
        echo "  docker-compose logs postgres"
        exit 1
    fi
done

# Initialize the database
echo "Initializing the database..."
docker-compose exec postgres psql -U postgres -f /docker-entrypoint-initdb.d/init.sql

echo "Setup complete! You can now start the applications with:"
echo "  ./mockserver &  # Run in background"
echo "  ./mcp-server    # Run in foreground"
echo
echo "Or use the make command:"
echo "  make local-dev"
#!/bin/bash
# Setup environment for functional tests
# This script exports required environment variables if not already set

set -e

# Function to set default value if not already set
set_default() {
    local var_name=$1
    local default_value=$2
    
    if [ -z "${!var_name}" ]; then
        export "$var_name=$default_value"
        echo "Set $var_name=$default_value"
    else
        echo "$var_name already set to ${!var_name}"
    fi
}

echo "Setting up functional test environment..."

# Load .env file if it exists
if [ -f "$(dirname "$0")/.env" ]; then
    echo "Loading .env file..."
    export $(cat "$(dirname "$0")/.env" | grep -v '^#' | xargs)
fi

# MCP Server Configuration
set_default "MCP_SERVER_URL" "http://localhost:8080"
set_default "MCP_API_KEY" "dev-admin-key-1234567890"

# Mock Server Configuration
set_default "MOCKSERVER_URL" "http://localhost:8082"

# GitHub Configuration
set_default "GITHUB_TOKEN" "test-github-token"
set_default "GITHUB_REPO" "test-repo"
set_default "GITHUB_OWNER" "test-org"
set_default "MCP_GITHUB_WEBHOOK_SECRET" "docker-github-webhook-secret"

# Redis/ElastiCache Configuration
set_default "REDIS_ADDR" "localhost:6379"
set_default "ELASTICACHE_ENDPOINT" "localhost"
set_default "ELASTICACHE_PORT" "6379"

# AWS/LocalStack Configuration
set_default "AWS_REGION" "us-west-2"
set_default "AWS_ACCESS_KEY_ID" "test"
set_default "AWS_SECRET_ACCESS_KEY" "test"
set_default "AWS_ENDPOINT_URL" "http://localhost:4566"
set_default "S3_BUCKET" "mcp-contexts"
set_default "S3_ENDPOINT" "http://localhost:4566"

# Check if services are running
echo ""
echo "Checking service availability..."

# Function to check if a service is accessible
check_service() {
    local service_name=$1
    local url=$2
    
    if curl -f -s "$url" > /dev/null 2>&1; then
        echo "✓ $service_name is accessible at $url"
    else
        echo "✗ $service_name is NOT accessible at $url"
        echo "  Make sure docker-compose is running: make docker-compose-up"
    fi
}

# Check services
check_service "MCP Server" "${MCP_SERVER_URL}/health"
check_service "Mock Server" "${MOCKSERVER_URL}/health"
check_service "LocalStack" "${AWS_ENDPOINT_URL}/_localstack/health"

# Check Redis
if redis-cli -h localhost ping > /dev/null 2>&1; then
    echo "✓ Redis is accessible at localhost:6379"
else
    echo "✗ Redis is NOT accessible at localhost:6379"
fi

echo ""
echo "Environment setup complete!"
echo "Run tests with: go test ./...
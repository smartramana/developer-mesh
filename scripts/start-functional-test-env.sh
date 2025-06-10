#!/bin/bash
# Start all services needed for functional testing with AWS

set -e

echo "Starting functional test environment..."

# Check if PostgreSQL container exists
if ! docker ps -a | grep -q devops-postgres; then
    echo "Starting PostgreSQL container..."
    docker run -d --name devops-postgres -p 5432:5432 \
        -e POSTGRES_USER=postgres \
        -e POSTGRES_PASSWORD=postgres \
        -e POSTGRES_DB=devops_mcp_dev \
        pgvector/pgvector:pg17
    sleep 5
elif ! docker ps | grep -q devops-postgres; then
    echo "Starting existing PostgreSQL container..."
    docker start devops-postgres
    sleep 3
else
    echo "PostgreSQL already running"
fi

# Initialize database if needed
echo "Initializing database schema..."
docker exec -i devops-postgres psql -U postgres -d devops_mcp_dev < scripts/db/init.sql 2>/dev/null || echo "Schema already exists"

# Check if SSH tunnel is needed
if [ "$USE_SSH_TUNNEL_FOR_REDIS" = "true" ]; then
    if ! nc -zv localhost 6379 2>&1 | grep -q succeeded; then
        echo ""
        echo "⚠️  SSH tunnel to ElastiCache is not active!"
        echo "Please run in another terminal:"
        echo "  ./scripts/aws/connect-elasticache.sh"
        echo ""
        echo "Press Enter when tunnel is ready..."
        read
    else
        echo "✓ SSH tunnel to ElastiCache is active"
    fi
fi

# Source environment
source .env

# Start services
echo "Starting MCP Server..."
MCP_CONFIG_FILE=configs/config.development.yaml ./apps/mcp-server/mcp-server &> logs/mcp-server.log &
MCP_PID=$!

echo "Starting REST API..."
MCP_CONFIG_FILE=configs/config.development.yaml API_PORT=8081 ./apps/rest-api/rest-api &> logs/rest-api.log &
API_PID=$!

echo "Starting Worker..."
./apps/worker/worker &> logs/worker.log &
WORKER_PID=$!

# Create PID file
echo "MCP_PID=$MCP_PID" > .test-pids
echo "API_PID=$API_PID" >> .test-pids
echo "WORKER_PID=$WORKER_PID" >> .test-pids

echo ""
echo "Services started with PIDs:"
echo "  MCP Server: $MCP_PID"
echo "  REST API: $API_PID"
echo "  Worker: $WORKER_PID"
echo ""
echo "Waiting for services to be ready..."
sleep 5

# Check health
if curl -s http://localhost:8080/health | jq -e '.status == "healthy"' >/dev/null; then
    echo "✓ MCP Server is healthy"
else
    echo "✗ MCP Server failed to start"
    cat logs/mcp-server.log | tail -20
fi

if curl -s http://localhost:8081/health >/dev/null; then
    echo "✓ REST API is running"
else
    echo "✗ REST API failed to start"
    cat logs/rest-api.log | tail -20
fi

echo ""
echo "Functional test environment is ready!"
echo "Run tests with: make test-functional"
echo "Stop services with: ./scripts/stop-functional-test-env.sh"
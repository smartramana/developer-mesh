#!/bin/bash
# Start functional test environment using AWS infrastructure

set -e

echo "Starting functional test environment with AWS services..."

# Source environment
source .env

# Set test mode environment variables
export MCP_TEST_MODE=true
export TEST_AUTH_ENABLED=true
export ENVIRONMENT=test

echo "Test mode configuration:"
echo "  MCP_TEST_MODE=$MCP_TEST_MODE"
echo "  TEST_AUTH_ENABLED=$TEST_AUTH_ENABLED"
echo "  ENVIRONMENT=$ENVIRONMENT"

# Check AWS connectivity
echo "Checking AWS services..."
if aws s3api head-bucket --bucket $S3_BUCKET &>/dev/null; then
    echo "✓ S3 bucket accessible: $S3_BUCKET"
else
    echo "✗ S3 bucket not accessible: $S3_BUCKET"
    aws s3api head-bucket --bucket $S3_BUCKET 2>&1
    exit 1
fi

if aws sqs get-queue-attributes --queue-url $SQS_QUEUE_URL &>/dev/null; then
    echo "✓ SQS queue accessible"
else
    echo "✗ SQS queue not accessible"
    exit 1
fi

# Check and set up SSH tunnels for RDS/ElastiCache
echo ""
echo "Checking SSH tunnels for private resources..."

# Check if tunnels are already running
if ! nc -zv localhost 6379 2>&1 | grep -q succeeded || ! nc -zv localhost 5432 2>&1 | grep -q succeeded; then
    echo "SSH tunnels not detected. Setting them up..."
    
    # Check if SSH key exists
    KEY_FILE=$(eval echo ${SSH_KEY_PATH:-~/.ssh/dev-bastion-key.pem})
    if [ ! -f "$KEY_FILE" ]; then
        echo "ERROR: SSH key file not found at: $KEY_FILE"
        echo "Please update SSH_KEY_PATH in .env with the correct path to your EC2 key"
        exit 1
    fi
    
    # Start SSH tunnel in background
    echo "Starting SSH tunnels in background..."
    ./setup-ssh-tunnels.sh &
    SSH_TUNNEL_PID=$!
    
    # Wait for tunnels to be established
    echo "Waiting for tunnels to be ready..."
    for i in {1..10}; do
        if nc -zv localhost 6379 2>&1 | grep -q succeeded && nc -zv localhost 5432 2>&1 | grep -q succeeded; then
            echo "✓ SSH tunnels established"
            echo "SSH_TUNNEL_PID=$SSH_TUNNEL_PID" >> .test-pids
            break
        fi
        sleep 2
    done
    
    # Verify tunnels are working
    if ! nc -zv localhost 6379 2>&1 | grep -q succeeded; then
        echo "✗ Failed to establish Redis tunnel"
        kill $SSH_TUNNEL_PID 2>/dev/null
        exit 1
    fi
    
    if ! nc -zv localhost 5432 2>&1 | grep -q succeeded; then
        echo "✗ Failed to establish PostgreSQL tunnel"
        kill $SSH_TUNNEL_PID 2>/dev/null
        exit 1
    fi
else
    echo "✓ SSH tunnels already active"
fi

# Get RDS password from Parameter Store if needed
if [ -z "$DATABASE_PASSWORD" ] || [ "$DATABASE_PASSWORD" == "postgres" ]; then
    echo "Retrieving database password from Parameter Store..."
    DB_PASSWORD=$(aws ssm get-parameter --name "/developer-mesh/rds/password" --with-decryption --query 'Parameter.Value' --output text --region us-east-1 2>/dev/null || echo "")
    if [ -n "$DB_PASSWORD" ]; then
        export DATABASE_PASSWORD="$DB_PASSWORD"
        export DB_PASSWORD="$DB_PASSWORD"
        echo "✓ Database password retrieved"
    else
        echo "⚠️  Could not retrieve password from Parameter Store"
    fi
fi

# Export database variables for the config file to pick up
export DB_USER="${DATABASE_USER:-dbadmin}"
export DB_PASSWORD="${DATABASE_PASSWORD}"
export DATABASE_SSL_MODE="${DATABASE_SSL_MODE:-require}"

# Create logs directory if it doesn't exist
mkdir -p logs

# Start services
echo ""
echo "Starting services..."

echo "Starting MCP Server..."
echo "DEBUG: DB_USER=$DB_USER"
echo "DEBUG: DB_PASSWORD=${DB_PASSWORD:0:10}..."
echo "DEBUG: DATABASE_SSL_MODE=$DATABASE_SSL_MODE"
echo "DEBUG: MCP_DATABASE_SSL_MODE=$MCP_DATABASE_SSL_MODE"
# Export with MCP prefix for MCP server
export MCP_DB_USER="$DB_USER"
export MCP_DB_PASSWORD="$DB_PASSWORD"
export MCP_DATABASE_SSL_MODE="$DATABASE_SSL_MODE"
MCP_TEST_MODE=$MCP_TEST_MODE TEST_AUTH_ENABLED=$TEST_AUTH_ENABLED ENVIRONMENT=$ENVIRONMENT MCP_CONFIG_FILE=configs/config.development.yaml ./apps/mcp-server/mcp-server &> logs/mcp-server.log &
MCP_PID=$!

echo "Starting REST API..."
MCP_TEST_MODE=$MCP_TEST_MODE TEST_AUTH_ENABLED=$TEST_AUTH_ENABLED ENVIRONMENT=$ENVIRONMENT MCP_CONFIG_FILE=configs/config.development.yaml MCP_API_LISTEN_ADDRESS=:8081 ./apps/rest-api/rest-api &> logs/rest-api.log &
API_PID=$!

echo "Starting Worker..."
MCP_TEST_MODE=$MCP_TEST_MODE TEST_AUTH_ENABLED=$TEST_AUTH_ENABLED ENVIRONMENT=$ENVIRONMENT ./apps/worker/worker &> logs/worker.log &
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
if curl -s http://localhost:8080/health | jq -e '.status == "healthy"' >/dev/null 2>&1; then
    echo "✓ MCP Server is healthy"
else
    echo "✗ MCP Server failed to start"
    echo "Last 20 lines of log:"
    tail -20 logs/mcp-server.log
fi

if curl -s http://localhost:8081/health >/dev/null 2>&1; then
    echo "✓ REST API is running"
else
    echo "✗ REST API failed to start"
    echo "Last 20 lines of log:"
    tail -20 logs/rest-api.log
fi

echo ""
echo "AWS Infrastructure Status:"
echo "  RDS Endpoint: $DATABASE_HOST"
echo "  ElastiCache Endpoint: ${REDIS_ADDR%:*}"
echo "  S3 Bucket: $S3_BUCKET"
echo "  SQS Queue: $SQS_QUEUE_URL"
echo ""
echo "Functional test environment is ready!"
echo "Run tests with: make test-functional-local"
echo "Stop services with: ./scripts/stop-functional-test-env.sh"
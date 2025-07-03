#!/bin/bash

# run_functional_tests_flexible.sh - Run functional tests against local or live endpoints
# Usage: 
#   ./run_functional_tests_flexible.sh          # Run against local endpoints
#   ./run_functional_tests_flexible.sh live     # Run against live endpoints

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get the project root directory
SCRIPT_DIR="$(dirname "$0")"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Check if we're running against live endpoints
TARGET="${1:-local}"

if [ "$TARGET" = "live" ]; then
    echo "Starting functional tests against LIVE endpoints from $PROJECT_ROOT"
    
    # Set live endpoints
    export MCP_WEBSOCKET_URL="wss://mcp.dev-mesh.io/ws"
    export REST_API_URL="https://api.dev-mesh.io"
    export MCP_SERVER_URL="https://mcp.dev-mesh.io"
    export MOCKSERVER_URL="" # Not available in production
    export SKIP_MOCK_TESTS=true
    export TEST_ENV="production"
    
    ENDPOINT_TYPE="LIVE"
    HEALTH_CHECK_MCP="https://mcp.dev-mesh.io/health"
    HEALTH_CHECK_API="https://api.dev-mesh.io/health"
else
    echo "Starting functional tests against LOCAL endpoints from $PROJECT_ROOT"
    
    # Set local endpoints
    export MCP_WEBSOCKET_URL="ws://localhost:8080/ws"
    export REST_API_URL="http://localhost:8081"
    export MCP_SERVER_URL="http://localhost:8080"
    export MOCKSERVER_URL="http://localhost:8082"
    export SKIP_MOCK_TESTS=false
    export TEST_ENV="local"
    
    ENDPOINT_TYPE="LOCAL"
    HEALTH_CHECK_MCP="http://localhost:8080/health"
    HEALTH_CHECK_API="http://localhost:8081/health"
fi

# Change to project root
cd "$PROJECT_ROOT"

# Source environment
echo "Loading environment variables..."
set -a
source .env
set +a

# Override API key for tests
export MCP_API_KEY="dev-admin-key-1234567890"
export ADMIN_API_KEY="dev-admin-key-1234567890"

# Display configuration
echo -e "\n${YELLOW}${ENDPOINT_TYPE} Endpoints Configuration:${NC}"
echo "  • MCP WebSocket: $MCP_WEBSOCKET_URL"
echo "  • REST API: $REST_API_URL"
echo "  • MCP Server: $MCP_SERVER_URL"
if [ "$TARGET" != "live" ]; then
    echo "  • Mock Server: $MOCKSERVER_URL"
fi
echo "  • API Key: $MCP_API_KEY"
echo "  • Test Environment: $TEST_ENV"

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"

# Function to check if a service is healthy
check_service() {
    local name=$1
    local url=$2
    
    if curl -s -o /dev/null -w "%{http_code}" "$url" | grep -q "200"; then
        echo -e "${GREEN}✓ $name is healthy${NC}"
        return 0
    else
        echo -e "${RED}✗ $name is not healthy${NC}"
        return 1
    fi
}

services_ok=true
check_service "MCP Server" "$HEALTH_CHECK_MCP" || services_ok=false
check_service "REST API" "$HEALTH_CHECK_API" || services_ok=false

if [ "$services_ok" = "false" ]; then
    echo -e "\n${RED}Services are not healthy!${NC}"
    if [ "$TARGET" = "local" ]; then
        echo "Run: make dev-native or ./scripts/start-functional-env-aws.sh"
    fi
    exit 1
fi

# Check AWS connectivity (for both local and live)
echo -e "\n${YELLOW}Checking AWS services...${NC}"

# Check S3
if aws s3 ls s3://$S3_BUCKET --region $AWS_REGION >/dev/null 2>&1; then
    echo -e "${GREEN}✓ S3 bucket accessible${NC}"
else
    echo -e "${YELLOW}⚠ S3 bucket not accessible${NC}"
fi

# Check SQS
if aws sqs get-queue-attributes --queue-url $SQS_QUEUE_URL --attribute-names All --region $AWS_REGION >/dev/null 2>&1; then
    echo -e "${GREEN}✓ SQS queue accessible${NC}"
else
    echo -e "${YELLOW}⚠ SQS queue not accessible${NC}"
fi

# Change to functional test directory
cd "$PROJECT_ROOT/test/functional"

# Setup test environment (only for local tests)
if [ "$TARGET" = "local" ]; then
    echo -e "\n${YELLOW}Setting up test environment...${NC}"
    source ./setup_env.sh
else
    echo -e "\n${YELLOW}Using live endpoint configuration...${NC}"
    # Set additional environment variables needed by tests
    export GITHUB_TOKEN="test-github-token"
    export GITHUB_REPO="test-repo"
    export GITHUB_OWNER="test-org"
    export MCP_GITHUB_WEBHOOK_SECRET="docker-github-webhook-secret"
fi

# Find Ginkgo
GINKGO_PATH=$(which ginkgo 2>/dev/null || echo "$HOME/go/bin/ginkgo")
if [ ! -x "$GINKGO_PATH" ]; then
    echo -e "${RED}Ginkgo not found. Installing...${NC}"
    go install github.com/onsi/ginkgo/v2/ginkgo@latest
    GINKGO_PATH="$HOME/go/bin/ginkgo"
fi

echo "Using Ginkgo at: $GINKGO_PATH"

# Run the tests
echo -e "\n${YELLOW}Running functional tests against ${ENDPOINT_TYPE} endpoints...${NC}"

# Parse any additional arguments passed to the script
GINKGO_ARGS=""
if [ ! -z "$FOCUS" ]; then
    GINKGO_ARGS="$GINKGO_ARGS --focus=\"$FOCUS\""
fi

if [ "$VERBOSE" = "1" ]; then
    GINKGO_ARGS="$GINKGO_ARGS -v"
fi

# Add timeout for live tests
if [ "$TARGET" = "live" ]; then
    GINKGO_ARGS="$GINKGO_ARGS --timeout=5m"
fi

# Run Ginkgo tests
if $GINKGO_PATH $GINKGO_ARGS ./...; then
    echo -e "\n${GREEN}✅ All tests passed against ${ENDPOINT_TYPE} endpoints!${NC}"
    exit_code=0
else
    echo -e "\n${RED}❌ Some tests failed against ${ENDPOINT_TYPE} endpoints${NC}"
    exit_code=1
fi

exit $exit_code
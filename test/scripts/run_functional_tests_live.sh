#!/bin/bash

# run_functional_tests_live.sh - Run functional tests against live production endpoints
# This script runs the functional test suite against the deployed application

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get the project root directory
SCRIPT_DIR="$(dirname "$0")"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "Starting functional tests against LIVE endpoints from $PROJECT_ROOT"

# Change to project root
cd "$PROJECT_ROOT"

# Set environment variables for live endpoints
export MCP_WEBSOCKET_URL="wss://mcp.dev-mesh.io/ws"
export REST_API_URL="https://api.dev-mesh.io"
export MOCKSERVER_URL="" # Not available in production

# Source production environment settings
echo "Loading environment variables..."
set -a
source .env
set +a

# Override with production values
export ENVIRONMENT="production"
export MCP_ENV="production"

# Display live endpoints configuration
echo -e "\n${YELLOW}Live Endpoints Configuration:${NC}"
echo "  • MCP WebSocket: $MCP_WEBSOCKET_URL"
echo "  • REST API: $REST_API_URL"
echo "  • API Key: Using dev-admin-key"

# Check prerequisites
echo -e "\n${YELLOW}Checking live endpoints...${NC}"

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
check_service "MCP Server" "https://mcp.dev-mesh.io/health" || services_ok=false
check_service "REST API" "https://api.dev-mesh.io/health" || services_ok=false

if [ "$services_ok" = "false" ]; then
    echo -e "\n${RED}Live services are not healthy!${NC}"
    exit 1
fi

# Check AWS connectivity (using live credentials)
echo -e "\n${YELLOW}Checking AWS services...${NC}"

# These should use the production AWS resources
if aws s3 ls s3://$S3_BUCKET --region $AWS_REGION >/dev/null 2>&1; then
    echo -e "${GREEN}✓ S3 bucket accessible${NC}"
else
    echo -e "${YELLOW}⚠ S3 bucket not accessible (may be IP restricted)${NC}"
fi

# Run functional tests
echo -e "\n${YELLOW}Running functional tests against live endpoints...${NC}"

# Export test configuration
export TEST_TIMEOUT=60s
export TEST_ENV=production
export ADMIN_API_KEY="dev-admin-key-1234567890"

# Disable tests that require mock server
export SKIP_MOCK_TESTS=true

# Change to functional test directory
cd test/functional

# Run only tests that work with live endpoints
echo -e "\n${YELLOW}Running API tests...${NC}"
go test ./api \
    -v \
    -count=1 \
    -timeout 5m \
    -tags="functional live" \
    -run="Test.*API|Test.*Health" \
    2>&1 | tee ../../test-results-api.log

echo -e "\n${YELLOW}Running MCP protocol tests...${NC}"
go test ./mcp \
    -v \
    -count=1 \
    -timeout 5m \
    -tags="functional live" \
    -run="Test.*Protocol|Test.*REST" \
    2>&1 | tee ../../test-results-mcp.log

# Return to project root
cd ../..

# Check if tests passed
if grep -q "FAIL" test-results-*.log; then
    echo -e "\n${RED}Some tests failed!${NC}"
    echo "Check test-results-*.log for details"
    exit 1
else
    echo -e "\n${GREEN}All functional tests passed!${NC}"
fi

# Cleanup
rm -f test-results-*.log

echo -e "\n${GREEN}✅ Functional tests completed successfully against live endpoints!${NC}"
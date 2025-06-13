#!/bin/bash

# run_functional_tests_local.sh - Run functional tests with local services and real AWS
# This script assumes services are running locally (not in Docker)

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get the project root directory
SCRIPT_DIR="$(dirname "$0")"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "Starting functional tests from $PROJECT_ROOT"

# Change to project root
cd "$PROJECT_ROOT"

# Source environment
echo "Loading environment variables..."
set -a
source .env
set +a

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"

# Check if PostgreSQL is running
if ! docker ps | grep -q devops-postgres; then
    echo -e "${RED}✗ PostgreSQL container is not running${NC}"
    echo "  Run: docker start devops-postgres"
    exit 1
else
    echo -e "${GREEN}✓ PostgreSQL is running${NC}"
fi

# Check if SSH tunnel is active (for ElastiCache)
if [ "$USE_SSH_TUNNEL_FOR_REDIS" = "true" ]; then
    if ! nc -zv localhost 6379 2>&1 | grep -q succeeded; then
        echo -e "${RED}✗ SSH tunnel to ElastiCache is not active${NC}"
        echo "  Run: ./scripts/aws/connect-elasticache.sh"
        exit 1
    else
        echo -e "${GREEN}✓ SSH tunnel is active${NC}"
    fi
fi

# Check if services are running
echo -e "\n${YELLOW}Checking services...${NC}"

check_service() {
    local name=$1
    local url=$2
    
    if curl -f -s "$url" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ $name is running${NC}"
        return 0
    else
        echo -e "${RED}✗ $name is not running${NC}"
        return 1
    fi
}

services_ok=true
check_service "MCP Server" "http://localhost:8080/health" || services_ok=false
check_service "REST API" "http://localhost:8081/health" || services_ok=false

if [ "$services_ok" = "false" ]; then
    echo -e "\n${RED}Services are not running!${NC}"
    echo "Run: ./scripts/start-functional-test-env.sh"
    exit 1
fi

# Check AWS connectivity
echo -e "\n${YELLOW}Checking AWS services...${NC}"

# Check S3
if aws s3 ls s3://$S3_BUCKET --region $AWS_REGION >/dev/null 2>&1; then
    echo -e "${GREEN}✓ S3 bucket accessible${NC}"
else
    echo -e "${RED}✗ S3 bucket not accessible${NC}"
fi

# Check SQS
if aws sqs get-queue-attributes --queue-url $SQS_QUEUE_URL --attribute-names All --region $AWS_REGION >/dev/null 2>&1; then
    echo -e "${GREEN}✓ SQS queue accessible${NC}"
else
    echo -e "${RED}✗ SQS queue not accessible${NC}"
fi

# Change to functional test directory
cd "$PROJECT_ROOT/test/functional"

# Setup test environment
echo -e "\n${YELLOW}Setting up test environment...${NC}"
source ./setup_env.sh

# Find Ginkgo
GINKGO_PATH=$(which ginkgo 2>/dev/null || echo "$HOME/go/bin/ginkgo")
if [ ! -x "$GINKGO_PATH" ]; then
    echo -e "${RED}Ginkgo not found. Installing...${NC}"
    go install github.com/onsi/ginkgo/v2/ginkgo@latest
    GINKGO_PATH="$HOME/go/bin/ginkgo"
fi

echo "Using Ginkgo at: $GINKGO_PATH"

# Run the tests
echo -e "\n${YELLOW}Running functional tests...${NC}"

# Parse any additional arguments passed to the script
GINKGO_ARGS=""
if [ ! -z "$FOCUS" ]; then
    GINKGO_ARGS="$GINKGO_ARGS --focus=\"$FOCUS\""
fi

if [ "$VERBOSE" = "1" ]; then
    GINKGO_ARGS="$GINKGO_ARGS -v"
fi

# Run Ginkgo tests
if $GINKGO_PATH $GINKGO_ARGS ./...; then
    echo -e "\n${GREEN}✅ All tests passed!${NC}"
    exit_code=0
else
    echo -e "\n${RED}❌ Some tests failed${NC}"
    exit_code=1
fi

exit $exit_code
#!/bin/bash
# run_functional_tests_local.sh - Script to run functional tests locally
# This script sets up the environment and runs functional tests against the local Docker services

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${GREEN}Starting functional tests from $PROJECT_ROOT${NC}"

# Change to project root
cd "$PROJECT_ROOT"

# Load environment variables from .env.test
if [ -f "test/functional/.env.test" ]; then
    echo -e "${GREEN}Loading environment from test/functional/.env.test${NC}"
    export $(cat test/functional/.env.test | grep -v '^#' | xargs)
else
    echo -e "${RED}Error: test/functional/.env.test not found${NC}"
    exit 1
fi

# Function to check if a service is healthy
check_service() {
    local service_name=$1
    local url=$2
    local max_attempts=30
    local attempt=1
    
    echo -e "${YELLOW}Checking $service_name at $url...${NC}"
    while [ $attempt -le $max_attempts ]; do
        if curl -f -s "$url" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ $service_name is healthy!${NC}"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt+1))
    done
    
    echo -e "\n${RED}✗ $service_name is not responding after $max_attempts attempts${NC}"
    return 1
}

# Function to check if PostgreSQL is ready
check_postgres() {
    local max_attempts=30
    local attempt=1
    
    echo -e "${YELLOW}Checking PostgreSQL...${NC}"
    while [ $attempt -le $max_attempts ]; do
        if docker exec devops-mcp-database-1 pg_isready -U postgres > /dev/null 2>&1; then
            echo -e "${GREEN}✓ PostgreSQL is ready!${NC}"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt+1))
    done
    
    echo -e "\n${RED}✗ PostgreSQL is not ready after $max_attempts attempts${NC}"
    return 1
}

# Function to check if Redis is ready
check_redis() {
    local max_attempts=30
    local attempt=1
    
    echo -e "${YELLOW}Checking Redis...${NC}"
    while [ $attempt -le $max_attempts ]; do
        if docker exec devops-mcp-redis-1 redis-cli ping > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Redis is ready!${NC}"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt+1))
    done
    
    echo -e "\n${RED}✗ Redis is not ready after $max_attempts attempts${NC}"
    return 1
}

# Check if services are already running
echo -e "\n${YELLOW}Checking service status...${NC}"
services_running=true

# Check each required service
if ! docker ps | grep -q "devops-mcp-mcp-server-1"; then
    echo -e "${RED}MCP Server is not running${NC}"
    services_running=false
fi

if ! docker ps | grep -q "devops-mcp-rest-api-1"; then
    echo -e "${RED}REST API is not running${NC}"
    services_running=false
fi

if ! docker ps | grep -q "devops-mcp-database-1"; then
    echo -e "${RED}PostgreSQL is not running${NC}"
    services_running=false
fi

if ! docker ps | grep -q "devops-mcp-redis-1"; then
    echo -e "${RED}Redis is not running${NC}"
    services_running=false
fi

if [ "$services_running" = false ]; then
    echo -e "${YELLOW}Some services are not running. Please start them with:${NC}"
    echo -e "${GREEN}make local-dev${NC}"
    echo -e "${YELLOW}or${NC}"
    echo -e "${GREEN}docker-compose -f docker-compose.local.yml up -d${NC}"
    exit 1
fi

# Verify all services are healthy
echo -e "\n${YELLOW}Verifying service health...${NC}"

# Check PostgreSQL
if ! check_postgres; then
    echo -e "${RED}PostgreSQL health check failed${NC}"
    exit 1
fi

# Check Redis
if ! check_redis; then
    echo -e "${RED}Redis health check failed${NC}"
    exit 1
fi

# Check MCP Server
if ! check_service "MCP Server" "http://localhost:8080/health"; then
    echo -e "${RED}MCP Server health check failed${NC}"
    exit 1
fi

# Check REST API
if ! check_service "REST API" "http://localhost:8081/health"; then
    echo -e "${RED}REST API health check failed${NC}"
    exit 1
fi

# Install Ginkgo if not present
if ! command -v ginkgo &> /dev/null; then
    echo -e "${YELLOW}Installing Ginkgo test framework...${NC}"
    go install github.com/onsi/ginkgo/v2/ginkgo@latest
    export PATH=$PATH:$HOME/go/bin
fi

# Create a mock server for external dependencies
echo -e "\n${YELLOW}Starting mock server for external dependencies...${NC}"

# Check if mockserver is already running
MOCKSERVER_PID=""
if lsof -i:8082 > /dev/null 2>&1; then
    echo -e "${GREEN}Mock server already running on port 8082${NC}"
else
    # Start the mockserver from the apps directory
    echo -e "${YELLOW}Building mock server...${NC}"
    cd "$PROJECT_ROOT/apps/mockserver"
    if ! go build -o mockserver ./cmd/main.go; then
        echo -e "${RED}Failed to build mock server${NC}"
        exit 1
    fi
    
    echo -e "${YELLOW}Starting mock server...${NC}"
    nohup ./mockserver > /tmp/mockserver.log 2>&1 &
    MOCKSERVER_PID=$!
    cd "$PROJECT_ROOT"
    
    # Wait for mockserver to start
    sleep 3
    
    if ! check_service "Mock Server" "http://localhost:8082/health"; then
        echo -e "${RED}Failed to start mock server${NC}"
        if [ ! -z "$MOCKSERVER_PID" ]; then
            kill $MOCKSERVER_PID 2>/dev/null || true
        fi
        echo -e "${RED}Mock server logs:${NC}"
        tail -20 /tmp/mockserver.log
        exit 1
    fi
fi

# Function to cleanup on exit
cleanup() {
    if [ ! -z "$MOCKSERVER_PID" ]; then
        echo -e "\n${YELLOW}Stopping mock server...${NC}"
        kill $MOCKSERVER_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Set up test data
echo -e "\n${YELLOW}Setting up test data...${NC}"
if [ -f "$PROJECT_ROOT/test/scripts/setup_test_data.sh" ]; then
    bash "$PROJECT_ROOT/test/scripts/setup_test_data.sh"
else
    echo -e "${YELLOW}Warning: Test data setup script not found${NC}"
fi

# Run the functional tests
echo -e "\n${GREEN}Running functional tests...${NC}"

# Parse command line arguments
VERBOSE=""
FOCUS=""
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -f|--focus)
            FOCUS="--focus=$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

# Change to test directory
cd "$PROJECT_ROOT/test/functional"

# Run tests with Ginkgo
if [ -n "$FOCUS" ]; then
    echo -e "${YELLOW}Running tests matching: ${FOCUS#--focus=}${NC}"
fi

ginkgo $VERBOSE $FOCUS --randomize-all --fail-on-pending ./...

# Check test results
if [ $? -eq 0 ]; then
    echo -e "\n${GREEN}✓ All functional tests passed!${NC}"
else
    echo -e "\n${RED}✗ Some functional tests failed${NC}"
    exit 1
fi
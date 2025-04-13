#!/bin/bash

# Test script for the MCP Server with mock services
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Testing MCP Server with mock services...${NC}"

# Check if mock server is running
echo -n "Checking mock server... "
if curl -s http://localhost:8081/health > /dev/null; then
    echo -e "${GREEN}Running${NC}"
else
    echo -e "${RED}Not running${NC}"
    echo "Starting mock server in the background..."
    ./mockserver > mockserver.log 2>&1 &
    MOCK_PID=$!
    echo "Mock server started with PID $MOCK_PID"
    # Wait for it to start
    sleep 2
fi

# Check if MCP server is running
echo -n "Checking MCP server... "
if curl -s http://localhost:8080/health > /dev/null; then
    echo -e "${GREEN}Running${NC}"
else
    echo -e "${RED}Not running${NC}"
    echo "Starting MCP server in the background..."
    ./mcp-server > mcp_server.log 2>&1 &
    MCP_PID=$!
    echo "MCP server started with PID $MCP_PID"
    # Wait for it to start
    sleep 5
fi

# Test mock server endpoints
echo -e "\n${YELLOW}Testing mock server endpoints:${NC}"

test_endpoint() {
    local endpoint=$1
    local expected_status=$2
    echo -n "Testing $endpoint... "
    status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081$endpoint)
    if [ "$status" -eq "$expected_status" ]; then
        echo -e "${GREEN}Success ($status)${NC}"
        return 0
    else
        echo -e "${RED}Failed ($status)${NC}"
        return 1
    fi
}

test_endpoint "/health" 200
test_endpoint "/mock-github" 200
test_endpoint "/mock-harness" 200
test_endpoint "/mock-sonarqube" 200
test_endpoint "/mock-artifactory" 200
test_endpoint "/mock-xray" 200

# Test MCP server endpoints
echo -e "\n${YELLOW}Testing MCP server endpoints:${NC}"

test_mcp_endpoint() {
    local endpoint=$1
    local expected_status=$2
    echo -n "Testing $endpoint... "
    status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080$endpoint)
    if [ "$status" -eq "$expected_status" ]; then
        echo -e "${GREEN}Success ($status)${NC}"
        return 0
    else
        echo -e "${RED}Failed ($status)${NC}"
        return 1
    fi
}

test_mcp_endpoint "/health" 200
test_mcp_endpoint "/metrics" 200
test_mcp_endpoint "/api/v1/webhook/github" 200

# Clean up if we started the servers
if [ ! -z "$MOCK_PID" ]; then
    echo "Stopping mock server (PID $MOCK_PID)..."
    kill $MOCK_PID
fi

if [ ! -z "$MCP_PID" ]; then
    echo "Stopping MCP server (PID $MCP_PID)..."
    kill $MCP_PID
fi

echo -e "\n${GREEN}Tests completed!${NC}"
#!/bin/bash

# Pass-Through Authentication Test Script
# This script demonstrates the pass-through authentication feature

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Pass-Through Authentication Test Script${NC}"
echo "========================================"

# Check if MCP server is running
echo -e "\n${YELLOW}1. Checking if MCP server is running...${NC}"
if curl -s -f http://localhost:8080/health > /dev/null; then
    echo -e "${GREEN}✓ MCP server is running${NC}"
else
    echo -e "${RED}✗ MCP server is not running. Please start it first.${NC}"
    exit 1
fi

# Test 1: Request without credentials (should use service account)
echo -e "\n${YELLOW}2. Testing without user credentials (service account fallback)...${NC}"
curl -X POST http://localhost:8080/api/v1/tools/github/actions/list_repositories \
  -H "Authorization: Bearer ${MCP_API_KEY:-test-api-key}" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "list_repositories",
    "parameters": {
      "type": "public"
    }
  }' \
  -s | jq '.' || echo -e "${RED}Failed${NC}"

# Test 2: Request with user PAT
echo -e "\n${YELLOW}3. Testing with user PAT credentials...${NC}"
curl -X POST http://localhost:8080/api/v1/tools/github/actions/list_repositories \
  -H "Authorization: Bearer ${MCP_API_KEY:-test-api-key}" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "list_repositories",
    "parameters": {
      "type": "owner"
    },
    "credentials": {
      "github": {
        "token": "'${TEST_GITHUB_PAT:-ghp_test123456789}'",
        "type": "pat"
      }
    }
  }' \
  -s | jq '.' || echo -e "${RED}Failed${NC}"

# Test 3: Request with OAuth token
echo -e "\n${YELLOW}4. Testing with OAuth token...${NC}"
curl -X POST http://localhost:8080/api/v1/tools/github/actions/get_user \
  -H "Authorization: Bearer ${MCP_API_KEY:-test-api-key}" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "get_user",
    "parameters": {},
    "credentials": {
      "github": {
        "token": "'${TEST_GITHUB_OAUTH:-gho_oauth123456}'",
        "type": "oauth"
      }
    }
  }' \
  -s | jq '.' || echo -e "${RED}Failed${NC}"

# Test 4: Request with multiple tool credentials
echo -e "\n${YELLOW}5. Testing with multiple tool credentials...${NC}"
curl -X POST http://localhost:8080/api/v1/tools/github/actions/sync_with_jira \
  -H "Authorization: Bearer ${MCP_API_KEY:-test-api-key}" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "sync_with_jira",
    "parameters": {
      "project": "TEST"
    },
    "credentials": {
      "github": {
        "token": "'${TEST_GITHUB_PAT:-ghp_test123456789}'",
        "type": "pat"
      },
      "jira": {
        "token": "'${TEST_JIRA_TOKEN:-jira_test_token}'",
        "type": "basic",
        "username": "user@example.com"
      }
    }
  }' \
  -s | jq '.' || echo -e "${RED}Failed${NC}"

# Test 5: Check metrics endpoint
echo -e "\n${YELLOW}6. Checking metrics for authentication methods...${NC}"
if curl -s http://localhost:9090/metrics 2>/dev/null | grep -q "github_auth_method"; then
    echo -e "${GREEN}✓ Authentication metrics are being recorded${NC}"
    curl -s http://localhost:9090/metrics | grep "github_auth_method" | head -5
else
    echo -e "${YELLOW}⚠ Metrics endpoint not available or no auth metrics found${NC}"
fi

echo -e "\n${GREEN}Pass-through authentication tests completed!${NC}"
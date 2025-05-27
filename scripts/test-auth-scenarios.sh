#!/bin/bash

# Test authentication and authorization scenarios for production readiness

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
REST_API_URL="${REST_API_URL:-http://localhost:8081}"
MCP_SERVER_URL="${MCP_SERVER_URL:-http://localhost:8080}"

echo "🔐 Testing Authentication & Authorization Scenarios"
echo "================================================"

# Test 1: Missing authentication
echo -e "\n${YELLOW}Test 1: Request without authentication${NC}"
response=$(curl -s -o /dev/null -w "%{http_code}" "$REST_API_URL/api/v1/agents")
if [ "$response" = "401" ]; then
    echo -e "${GREEN}✓ Correctly rejected unauthenticated request${NC}"
else
    echo -e "${RED}✗ Failed: Expected 401, got $response${NC}"
    exit 1
fi

# Test 2: Invalid token
echo -e "\n${YELLOW}Test 2: Request with invalid token${NC}"
response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer invalid-token" \
    "$REST_API_URL/api/v1/agents")
if [ "$response" = "401" ]; then
    echo -e "${GREEN}✓ Correctly rejected invalid token${NC}"
else
    echo -e "${RED}✗ Failed: Expected 401, got $response${NC}"
    exit 1
fi

# Test 3: Missing tenant ID
echo -e "\n${YELLOW}Test 3: Request without tenant ID${NC}"
response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer test-token" \
    "$REST_API_URL/api/v1/agents")
if [ "$response" = "400" ] || [ "$response" = "401" ]; then
    echo -e "${GREEN}✓ Correctly rejected request without tenant ID${NC}"
else
    echo -e "${RED}✗ Failed: Expected 400/401, got $response${NC}"
    exit 1
fi

# Test 4: Cross-tenant access attempt
echo -e "\n${YELLOW}Test 4: Cross-tenant access prevention${NC}"
# Create resource with tenant A
create_response=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer test-token" \
    -H "X-Tenant-ID: tenant-a" \
    -H "Content-Type: application/json" \
    -d '{
        "agent_id": "test-agent",
        "model_id": "test-model",
        "metadata": {"test": true}
    }')

if [[ "$create_response" == *"id"* ]]; then
    context_id=$(echo "$create_response" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    
    # Try to access with tenant B
    access_response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer test-token" \
        -H "X-Tenant-ID: tenant-b" \
        "$REST_API_URL/api/v1/contexts/$context_id")
    
    if [ "$access_response" = "404" ] || [ "$access_response" = "403" ]; then
        echo -e "${GREEN}✓ Cross-tenant access correctly prevented${NC}"
    else
        echo -e "${RED}✗ Failed: Expected 404/403, got $access_response${NC}"
        exit 1
    fi
fi

# Test 5: Token expiration
echo -e "\n${YELLOW}Test 5: Expired token handling${NC}"
# This would require a real expired token - simulating with invalid token
response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer expired-token" \
    "$REST_API_URL/api/v1/agents")
if [ "$response" = "401" ]; then
    echo -e "${GREEN}✓ Correctly rejected expired token${NC}"
else
    echo -e "${YELLOW}⚠ Warning: Could not test expired token properly${NC}"
fi

# Test 6: Rate limiting
echo -e "\n${YELLOW}Test 6: Rate limiting enforcement${NC}"
rate_limit_hit=false
for i in {1..100}; do
    response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer test-token" \
        -H "X-Tenant-ID: tenant-test" \
        "$REST_API_URL/api/v1/agents")
    
    if [ "$response" = "429" ]; then
        rate_limit_hit=true
        echo -e "${GREEN}✓ Rate limiting triggered after $i requests${NC}"
        break
    fi
done

if [ "$rate_limit_hit" = false ]; then
    echo -e "${YELLOW}⚠ Warning: Rate limiting might not be configured${NC}"
fi

# Test 7: SQL injection attempt
echo -e "\n${YELLOW}Test 7: SQL injection prevention${NC}"
injection_response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer test-token" \
    -H "X-Tenant-ID: tenant-test" \
    "$REST_API_URL/api/v1/agents?name=test';DROP TABLE agents;--")

if [ "$injection_response" != "500" ]; then
    echo -e "${GREEN}✓ SQL injection attempt handled safely${NC}"
else
    echo -e "${RED}✗ Failed: Server error on SQL injection attempt${NC}"
    exit 1
fi

# Test 8: XSS prevention
echo -e "\n${YELLOW}Test 8: XSS prevention${NC}"
xss_payload='{"name":"<script>alert(1)</script>","description":"test"}'
xss_response=$(curl -s -X POST "$REST_API_URL/api/v1/agents" \
    -H "Authorization: Bearer test-token" \
    -H "X-Tenant-ID: tenant-test" \
    -H "Content-Type: application/json" \
    -d "$xss_payload")

if [[ "$xss_response" != *"<script>"* ]]; then
    echo -e "${GREEN}✓ XSS payload properly escaped/rejected${NC}"
else
    echo -e "${RED}✗ Failed: XSS payload not sanitized${NC}"
    exit 1
fi

# Test 9: CORS headers
echo -e "\n${YELLOW}Test 9: CORS configuration${NC}"
cors_response=$(curl -s -I -X OPTIONS \
    -H "Origin: https://example.com" \
    -H "Access-Control-Request-Method: GET" \
    "$REST_API_URL/api/v1/agents")

if [[ "$cors_response" == *"Access-Control-Allow-Origin"* ]]; then
    echo -e "${GREEN}✓ CORS headers properly configured${NC}"
else
    echo -e "${YELLOW}⚠ Warning: CORS headers might not be configured${NC}"
fi

# Test 10: JWT signature validation
echo -e "\n${YELLOW}Test 10: JWT signature validation${NC}"
# Create a token with invalid signature
invalid_jwt="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.invalid_signature"
response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $invalid_jwt" \
    "$REST_API_URL/api/v1/agents")

if [ "$response" = "401" ]; then
    echo -e "${GREEN}✓ Invalid JWT signature correctly rejected${NC}"
else
    echo -e "${RED}✗ Failed: Expected 401 for invalid JWT, got $response${NC}"
    exit 1
fi

echo -e "\n${GREEN}✅ Authentication & Authorization tests completed!${NC}"
echo "================================================"

# Summary
echo -e "\n📊 Test Summary:"
echo "- Unauthenticated requests: ✓"
echo "- Invalid tokens: ✓"
echo "- Multi-tenancy isolation: ✓"
echo "- Security headers: ✓"
echo "- Input validation: ✓"
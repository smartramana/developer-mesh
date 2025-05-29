#!/bin/bash

# Test write operation validation for all API endpoints

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
REST_API_URL="${REST_API_URL:-http://localhost:8081}"
AUTH_TOKEN="${AUTH_TOKEN:-test-token}"
TENANT_ID="${TENANT_ID:-test-tenant}"

echo "✍️  Testing Write Operation Validation"
echo "===================================="

# Function to test endpoint validation
test_validation() {
    local method=$1
    local endpoint=$2
    local test_name=$3
    local payload=$4
    local expected_status=$5
    
    echo -e "\n${YELLOW}Testing: $test_name${NC}"
    
    response=$(curl -s -w "\n%{http_code}" -X "$method" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$REST_API_URL$endpoint")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)
    
    if [ "$http_code" = "$expected_status" ]; then
        echo -e "${GREEN}✓ Got expected status: $http_code${NC}"
        return 0
    else
        echo -e "${RED}✗ Expected $expected_status, got $http_code${NC}"
        echo "Response body: $body"
        return 1
    fi
}

# Test Context Creation Validation
echo -e "\n${YELLOW}=== Context Creation Validation ===${NC}"

# Test 1: Missing required fields
test_validation "POST" "/api/v1/contexts" \
    "Missing agent_id" \
    '{"model_id": "test-model"}' \
    "400"

test_validation "POST" "/api/v1/contexts" \
    "Missing model_id" \
    '{"agent_id": "test-agent"}' \
    "400"

# Test 2: Invalid field types
test_validation "POST" "/api/v1/contexts" \
    "Invalid max_tokens type" \
    '{"agent_id": "test-agent", "model_id": "test-model", "max_tokens": "invalid"}' \
    "400"

# Test 3: Field length validation
test_validation "POST" "/api/v1/contexts" \
    "Agent ID too long" \
    '{"agent_id": "'$(printf 'a%.0s' {1..300})'", "model_id": "test-model"}' \
    "400"

# Test 4: Valid context creation
test_validation "POST" "/api/v1/contexts" \
    "Valid context creation" \
    '{"agent_id": "test-agent", "model_id": "test-model", "max_tokens": 4000, "metadata": {"test": true}}' \
    "201"

# Extract context ID for update tests
if [ "$http_code" = "201" ]; then
    CONTEXT_ID=$(echo "$body" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    echo "Created context ID: $CONTEXT_ID"
fi

# Test Context Update Validation
echo -e "\n${YELLOW}=== Context Update Validation ===${NC}"

if [ -n "$CONTEXT_ID" ]; then
    # Test 5: Invalid content structure
    test_validation "PUT" "/api/v1/contexts/$CONTEXT_ID" \
        "Invalid content structure" \
        '{"content": "invalid"}' \
        "400"
    
    # Test 6: Missing role in content
    test_validation "PUT" "/api/v1/contexts/$CONTEXT_ID" \
        "Missing role in content" \
        '{"content": [{"content": "test"}]}' \
        "400"
    
    # Test 7: Invalid role
    test_validation "PUT" "/api/v1/contexts/$CONTEXT_ID" \
        "Invalid role" \
        '{"content": [{"role": "invalid", "content": "test"}]}' \
        "400"
    
    # Test 8: Content too long
    long_content=$(printf 'x%.0s' {1..50000})
    test_validation "PUT" "/api/v1/contexts/$CONTEXT_ID" \
        "Content exceeds token limit" \
        '{"content": [{"role": "user", "content": "'$long_content'"}]}' \
        "400"
    
    # Test 9: Valid update
    test_validation "PUT" "/api/v1/contexts/$CONTEXT_ID" \
        "Valid context update" \
        '{"content": [{"role": "user", "content": "Test message"}]}' \
        "200"
fi

# Test Agent Creation Validation
echo -e "\n${YELLOW}=== Agent Creation Validation ===${NC}"

# Test 10: Missing name
test_validation "POST" "/api/v1/agents" \
    "Missing agent name" \
    '{"description": "Test agent"}' \
    "400"

# Test 11: Duplicate name (if enforced)
test_validation "POST" "/api/v1/agents" \
    "Create first agent" \
    '{"name": "duplicate-test", "description": "First agent"}' \
    "201"

test_validation "POST" "/api/v1/agents" \
    "Duplicate agent name" \
    '{"name": "duplicate-test", "description": "Second agent"}' \
    "409"

# Test 12: Special characters in name
test_validation "POST" "/api/v1/agents" \
    "Special characters in name" \
    '{"name": "test<script>alert(1)</script>", "description": "XSS test"}' \
    "400"

# Test Model Creation Validation
echo -e "\n${YELLOW}=== Model Creation Validation ===${NC}"

# Test 13: Invalid provider
test_validation "POST" "/api/v1/models" \
    "Invalid provider" \
    '{"name": "test-model", "provider": "invalid-provider", "model_type": "chat"}' \
    "400"

# Test 14: Missing model type
test_validation "POST" "/api/v1/models" \
    "Missing model type" \
    '{"name": "test-model", "provider": "openai"}' \
    "400"

# Test Bulk Operations
echo -e "\n${YELLOW}=== Bulk Operation Validation ===${NC}"

# Test 15: Bulk create with mixed valid/invalid
test_validation "POST" "/api/v1/contexts/bulk" \
    "Bulk create with validation errors" \
    '[
        {"agent_id": "test-agent", "model_id": "test-model"},
        {"model_id": "test-model"},
        {"agent_id": "test-agent"}
    ]' \
    "400"

# Test 16: Bulk update
if [ -n "$CONTEXT_ID" ]; then
    test_validation "PUT" "/api/v1/contexts/bulk" \
        "Bulk update contexts" \
        '[
            {"id": "'$CONTEXT_ID'", "metadata": {"bulk": true}},
            {"id": "invalid-id", "metadata": {"bulk": true}}
        ]' \
        "207"  # Multi-status for partial success
fi

# Test Delete Operations
echo -e "\n${YELLOW}=== Delete Operation Validation ===${NC}"

# Test 17: Delete non-existent resource
test_validation "DELETE" "/api/v1/contexts/non-existent-id" \
    "Delete non-existent context" \
    "" \
    "404"

# Test 18: Delete without permission (wrong tenant)
if [ -n "$CONTEXT_ID" ]; then
    response=$(curl -s -w "\n%{http_code}" -X DELETE \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: different-tenant" \
        "$REST_API_URL/api/v1/contexts/$CONTEXT_ID")
    
    http_code=$(echo "$response" | tail -n1)
    if [ "$http_code" = "404" ] || [ "$http_code" = "403" ]; then
        echo -e "${GREEN}✓ Cross-tenant delete prevented${NC}"
    else
        echo -e "${RED}✗ Cross-tenant delete not prevented (got $http_code)${NC}"
    fi
fi

# Test JSON Validation
echo -e "\n${YELLOW}=== JSON Structure Validation ===${NC}"

# Test 19: Malformed JSON
response=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{invalid json}' \
    "$REST_API_URL/api/v1/contexts")

http_code=$(echo "$response" | tail -n1)
if [ "$http_code" = "400" ]; then
    echo -e "${GREEN}✓ Malformed JSON rejected${NC}"
else
    echo -e "${RED}✗ Malformed JSON not properly rejected${NC}"
fi

# Test 20: Nested object validation
test_validation "POST" "/api/v1/contexts" \
    "Invalid nested metadata" \
    '{"agent_id": "test", "model_id": "test", "metadata": "not-an-object"}' \
    "400"

# Summary
echo -e "\n${GREEN}✅ Write validation tests completed!${NC}"
echo "===================================="
echo "Key validations tested:"
echo "- Required field validation"
echo "- Field type validation"
echo "- Field length limits"
echo "- Cross-tenant isolation"
echo "- Bulk operation handling"
echo "- JSON structure validation"
#!/bin/bash

# Test idempotency of write operations

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
REST_API_URL="${REST_API_URL:-http://localhost:8081}"
AUTH_TOKEN="${AUTH_TOKEN:-test-token}"
TENANT_ID="${TENANT_ID:-test-tenant}"

echo "🔄 Testing Write Operation Idempotency"
echo "====================================="

# Function to compare JSON responses
compare_json() {
    local json1=$1
    local json2=$2
    local exclude_fields=$3
    
    # Remove fields that are expected to change (timestamps, etc.)
    if [ -n "$exclude_fields" ]; then
        for field in $exclude_fields; do
            json1=$(echo "$json1" | jq "del(.$field)")
            json2=$(echo "$json2" | jq "del(.$field)")
        done
    fi
    
    # Compare normalized JSON
    if [ "$(echo "$json1" | jq -S .)" = "$(echo "$json2" | jq -S .)" ]; then
        return 0
    else
        return 1
    fi
}

# Test PUT idempotency
echo -e "\n${YELLOW}=== Testing PUT Idempotency ===${NC}"

# Create a context first
echo "Creating test context..."
create_response=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "agent_id": "idempotency-test-agent",
        "model_id": "test-model",
        "metadata": {"test": "idempotency"}
    }')

CONTEXT_ID=$(echo "$create_response" | jq -r '.id')
echo "Created context: $CONTEXT_ID"

# First PUT request
echo -e "\n${BLUE}Sending first PUT request...${NC}"
update_payload='{
    "metadata": {
        "updated": true,
        "version": 1,
        "timestamp": "2024-01-01T00:00:00Z"
    },
    "content": [
        {"role": "system", "content": "You are a helpful assistant"},
        {"role": "user", "content": "Hello"}
    ]
}'

response1=$(curl -s -X PUT "$REST_API_URL/api/v1/contexts/$CONTEXT_ID" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d "$update_payload")

# Second identical PUT request
echo -e "${BLUE}Sending identical PUT request...${NC}"
sleep 1  # Small delay to ensure different timestamp if not idempotent

response2=$(curl -s -X PUT "$REST_API_URL/api/v1/contexts/$CONTEXT_ID" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d "$update_payload")

# Compare responses (excluding timestamps)
if compare_json "$response1" "$response2" "updated_at"; then
    echo -e "${GREEN}✓ PUT operation is idempotent${NC}"
else
    echo -e "${RED}✗ PUT operation is NOT idempotent${NC}"
    echo "First response: $(echo "$response1" | jq -c .)"
    echo "Second response: $(echo "$response2" | jq -c .)"
fi

# Test PATCH idempotency (if supported)
echo -e "\n${YELLOW}=== Testing PATCH Idempotency ===${NC}"

patch_payload='{
    "metadata": {
        "patch_test": true,
        "patch_version": 1
    }
}'

echo -e "${BLUE}Sending first PATCH request...${NC}"
patch_response1=$(curl -s -X PATCH "$REST_API_URL/api/v1/contexts/$CONTEXT_ID" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d "$patch_payload")

patch_status1=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$REST_API_URL/api/v1/contexts/$CONTEXT_ID" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d "$patch_payload")

if [ "$patch_status1" = "200" ] || [ "$patch_status1" = "204" ]; then
    echo -e "${BLUE}Sending identical PATCH request...${NC}"
    patch_response2=$(curl -s -X PATCH "$REST_API_URL/api/v1/contexts/$CONTEXT_ID" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -H "Content-Type: application/json" \
        -d "$patch_payload")
    
    if compare_json "$patch_response1" "$patch_response2" "updated_at"; then
        echo -e "${GREEN}✓ PATCH operation is idempotent${NC}"
    else
        echo -e "${RED}✗ PATCH operation is NOT idempotent${NC}"
    fi
else
    echo -e "${YELLOW}⚠ PATCH not supported, skipping${NC}"
fi

# Test DELETE idempotency
echo -e "\n${YELLOW}=== Testing DELETE Idempotency ===${NC}"

# Create a context to delete
delete_context=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "agent_id": "delete-test-agent",
        "model_id": "test-model"
    }')

DELETE_ID=$(echo "$delete_context" | jq -r '.id')

echo -e "${BLUE}Sending first DELETE request...${NC}"
delete_status1=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
    "$REST_API_URL/api/v1/contexts/$DELETE_ID" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID")

echo "First DELETE status: $delete_status1"

echo -e "${BLUE}Sending second DELETE request...${NC}"
delete_status2=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
    "$REST_API_URL/api/v1/contexts/$DELETE_ID" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID")

echo "Second DELETE status: $delete_status2"

if [ "$delete_status1" = "204" ] || [ "$delete_status1" = "200" ]; then
    if [ "$delete_status2" = "404" ] || [ "$delete_status2" = "204" ]; then
        echo -e "${GREEN}✓ DELETE operation handles idempotency correctly${NC}"
        echo "  (Returns 404 or 204 for already deleted resource)"
    else
        echo -e "${RED}✗ DELETE operation idempotency issue${NC}"
    fi
fi

# Test idempotency with request IDs
echo -e "\n${YELLOW}=== Testing Request ID Idempotency ===${NC}"

REQUEST_ID="test-request-$(date +%s)"

echo -e "${BLUE}Sending request with Idempotency-Key...${NC}"
idem_response1=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $REQUEST_ID" \
    -d '{
        "agent_id": "idem-key-agent",
        "model_id": "test-model",
        "metadata": {"request_id": "'$REQUEST_ID'"}
    }')

idem_status1=$(echo "$idem_response1" | jq -r '.id' 2>/dev/null)

if [ -n "$idem_status1" ] && [ "$idem_status1" != "null" ]; then
    echo "First request created context: $idem_status1"
    
    echo -e "${BLUE}Sending duplicate request with same Idempotency-Key...${NC}"
    idem_response2=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -H "Content-Type: application/json" \
        -H "Idempotency-Key: $REQUEST_ID" \
        -d '{
            "agent_id": "different-agent",
            "model_id": "different-model",
            "metadata": {"should_be_ignored": true}
        }')
    
    idem_status2=$(echo "$idem_response2" | jq -r '.id' 2>/dev/null)
    
    if [ "$idem_status1" = "$idem_status2" ]; then
        echo -e "${GREEN}✓ Idempotency-Key prevents duplicate creation${NC}"
    else
        echo -e "${YELLOW}⚠ Idempotency-Key not supported or not working${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Could not test Idempotency-Key feature${NC}"
fi

# Test concurrent identical requests
echo -e "\n${YELLOW}=== Testing Concurrent Identical Requests ===${NC}"

concurrent_payload='{
    "agent_id": "concurrent-test-agent",
    "model_id": "test-model",
    "metadata": {"concurrent": true}
}'

echo -e "${BLUE}Sending 5 concurrent identical requests...${NC}"

# Send requests in background
for i in {1..5}; do
    curl -s -X POST "$REST_API_URL/api/v1/contexts" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -H "Content-Type: application/json" \
        -d "$concurrent_payload" > /tmp/concurrent_response_$i.json &
done

# Wait for all requests to complete
wait

# Check if all responses are identical or properly handled
unique_ids=$(cat /tmp/concurrent_response_*.json | jq -r '.id' | sort -u | wc -l)
echo "Unique IDs created: $unique_ids"

if [ "$unique_ids" -eq 5 ]; then
    echo -e "${GREEN}✓ Concurrent requests created separate resources (expected behavior)${NC}"
elif [ "$unique_ids" -eq 1 ]; then
    echo -e "${GREEN}✓ Concurrent requests were deduplicated (idempotent)${NC}"
else
    echo -e "${YELLOW}⚠ Unexpected behavior with concurrent requests${NC}"
fi

# Cleanup
rm -f /tmp/concurrent_response_*.json

# Summary
echo -e "\n${GREEN}✅ Idempotency tests completed!${NC}"
echo "====================================="
echo "Summary of findings:"
echo "- PUT operations should be naturally idempotent"
echo "- DELETE operations should handle repeated calls gracefully"
echo "- Idempotency-Key header support varies by implementation"
echo "- Concurrent identical requests behavior documented"
#!/bin/bash

# Test multi-tenant isolation for write operations

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

echo "🏢 Testing Multi-Tenant Write Isolation"
echo "======================================"

# Test tenants
TENANT_A="tenant-alpha"
TENANT_B="tenant-beta"
TENANT_C="tenant-gamma"

# Function to create resource for tenant
create_for_tenant() {
    local tenant=$1
    local resource_type=$2
    local payload=$3
    
    response=$(curl -s -X POST "$REST_API_URL/api/v1/$resource_type" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $tenant" \
        -H "Content-Type: application/json" \
        -d "$payload")
    
    echo "$response" | jq -r '.id' 2>/dev/null || echo ""
}

# Function to attempt access from different tenant
attempt_cross_tenant_access() {
    local owner_tenant=$1
    local accessor_tenant=$2
    local resource_type=$3
    local resource_id=$4
    local operation=$5
    
    case $operation in
        "READ")
            status=$(curl -s -o /dev/null -w "%{http_code}" \
                -H "Authorization: Bearer $AUTH_TOKEN" \
                -H "X-Tenant-ID: $accessor_tenant" \
                "$REST_API_URL/api/v1/$resource_type/$resource_id")
            ;;
        "UPDATE")
            status=$(curl -s -o /dev/null -w "%{http_code}" -X PUT \
                -H "Authorization: Bearer $AUTH_TOKEN" \
                -H "X-Tenant-ID: $accessor_tenant" \
                -H "Content-Type: application/json" \
                -d '{"metadata": {"hacked": true}}' \
                "$REST_API_URL/api/v1/$resource_type/$resource_id")
            ;;
        "DELETE")
            status=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE \
                -H "Authorization: Bearer $AUTH_TOKEN" \
                -H "X-Tenant-ID: $accessor_tenant" \
                "$REST_API_URL/api/v1/$resource_type/$resource_id")
            ;;
    esac
    
    if [ "$status" = "404" ] || [ "$status" = "403" ]; then
        echo -e "${GREEN}✓ $operation blocked (status: $status)${NC}"
        return 0
    else
        echo -e "${RED}✗ $operation NOT blocked (status: $status)${NC}"
        return 1
    fi
}

# Test 1: Context isolation
echo -e "\n${YELLOW}=== Testing Context Isolation ===${NC}"

# Create contexts for each tenant
echo "Creating contexts for each tenant..."
CONTEXT_A=$(create_for_tenant "$TENANT_A" "contexts" '{
    "agent_id": "agent-a",
    "model_id": "model-a",
    "metadata": {"tenant": "A", "sensitive": "data-a"}
}')

CONTEXT_B=$(create_for_tenant "$TENANT_B" "contexts" '{
    "agent_id": "agent-b",
    "model_id": "model-b",
    "metadata": {"tenant": "B", "sensitive": "data-b"}
}')

echo "Tenant A context: $CONTEXT_A"
echo "Tenant B context: $CONTEXT_B"

# Test cross-tenant access attempts
echo -e "\n${BLUE}Testing Tenant B attempting to access Tenant A's context...${NC}"
attempt_cross_tenant_access "$TENANT_A" "$TENANT_B" "contexts" "$CONTEXT_A" "READ"
attempt_cross_tenant_access "$TENANT_A" "$TENANT_B" "contexts" "$CONTEXT_A" "UPDATE"
attempt_cross_tenant_access "$TENANT_A" "$TENANT_B" "contexts" "$CONTEXT_A" "DELETE"

# Test 2: Agent isolation
echo -e "\n${YELLOW}=== Testing Agent Isolation ===${NC}"

AGENT_A=$(create_for_tenant "$TENANT_A" "agents" '{
    "name": "agent-tenant-a",
    "description": "Private agent for tenant A"
}')

echo "Tenant A agent: $AGENT_A"

echo -e "\n${BLUE}Testing Tenant C attempting to modify Tenant A's agent...${NC}"
attempt_cross_tenant_access "$TENANT_A" "$TENANT_C" "agents" "$AGENT_A" "UPDATE"
attempt_cross_tenant_access "$TENANT_A" "$TENANT_C" "agents" "$AGENT_A" "DELETE"

# Test 3: Bulk operations isolation
echo -e "\n${YELLOW}=== Testing Bulk Operations Isolation ===${NC}"

# Create multiple contexts for tenant A
echo "Creating multiple contexts for Tenant A..."
BULK_IDS=()
for i in {1..3}; do
    id=$(create_for_tenant "$TENANT_A" "contexts" '{
        "agent_id": "bulk-agent-'$i'",
        "model_id": "model-a",
        "metadata": {"bulk": true, "index": '$i'}
    }')
    BULK_IDS+=("$id")
done

# Attempt bulk update from different tenant
echo -e "\n${BLUE}Testing bulk update from Tenant B on Tenant A's resources...${NC}"
bulk_payload='['
for id in "${BULK_IDS[@]}"; do
    bulk_payload+='{"id":"'$id'","metadata":{"compromised":true}},'
done
bulk_payload="${bulk_payload%,}]"

bulk_status=$(curl -s -o /dev/null -w "%{http_code}" -X PUT \
    "$REST_API_URL/api/v1/contexts/bulk" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_B" \
    -H "Content-Type: application/json" \
    -d "$bulk_payload")

if [ "$bulk_status" = "404" ] || [ "$bulk_status" = "403" ] || [ "$bulk_status" = "207" ]; then
    echo -e "${GREEN}✓ Bulk update properly isolated (status: $bulk_status)${NC}"
else
    echo -e "${RED}✗ Bulk update NOT properly isolated (status: $bulk_status)${NC}"
fi

# Test 4: Search isolation
echo -e "\n${YELLOW}=== Testing Search Isolation ===${NC}"

# Search from Tenant A (should see their own data)
echo -e "\n${BLUE}Tenant A searching for contexts...${NC}"
search_a=$(curl -s "$REST_API_URL/api/v1/contexts?limit=100" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_A")

count_a=$(echo "$search_a" | jq '.data | length' 2>/dev/null || echo "0")
echo "Tenant A sees $count_a contexts"

# Search from Tenant B (should NOT see Tenant A's data)
echo -e "\n${BLUE}Tenant B searching for contexts...${NC}"
search_b=$(curl -s "$REST_API_URL/api/v1/contexts?limit=100" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_B")

count_b=$(echo "$search_b" | jq '.data | length' 2>/dev/null || echo "0")
echo "Tenant B sees $count_b contexts"

# Verify no cross-contamination
if echo "$search_b" | grep -q "$CONTEXT_A"; then
    echo -e "${RED}✗ Tenant B can see Tenant A's context ID!${NC}"
else
    echo -e "${GREEN}✓ Search results properly isolated${NC}"
fi

# Test 5: Relationship/Reference isolation
echo -e "\n${YELLOW}=== Testing Relationship Isolation ===${NC}"

# Create model in Tenant A
MODEL_A=$(create_for_tenant "$TENANT_A" "models" '{
    "name": "private-model-a",
    "provider": "openai",
    "model_type": "chat"
}')

# Try to create context in Tenant B referencing Tenant A's model
echo -e "\n${BLUE}Tenant B trying to reference Tenant A's model...${NC}"
ref_response=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_B" \
    -H "Content-Type: application/json" \
    -d '{
        "agent_id": "agent-b",
        "model_id": "'$MODEL_A'",
        "metadata": {"attempted_reference": true}
    }')

ref_status=$(echo "$ref_response" | jq -r '.error' 2>/dev/null)
if [ -n "$ref_status" ] && [ "$ref_status" != "null" ]; then
    echo -e "${GREEN}✓ Cross-tenant reference blocked${NC}"
else
    echo -e "${RED}✗ Cross-tenant reference NOT blocked${NC}"
fi

# Test 6: Tenant ID injection attempts
echo -e "\n${YELLOW}=== Testing Tenant ID Injection ===${NC}"

# Try to inject tenant ID in payload
echo -e "\n${BLUE}Attempting to override tenant ID in payload...${NC}"
injection_response=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_B" \
    -H "Content-Type: application/json" \
    -d '{
        "agent_id": "injection-test",
        "model_id": "test-model",
        "tenant_id": "'$TENANT_A'",
        "metadata": {"tenant_id": "'$TENANT_A'", "injected": true}
    }')

injection_id=$(echo "$injection_response" | jq -r '.id' 2>/dev/null)
if [ -n "$injection_id" ] && [ "$injection_id" != "null" ]; then
    # Verify it was created for the correct tenant
    verify_status=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_A" \
        "$REST_API_URL/api/v1/contexts/$injection_id")
    
    if [ "$verify_status" = "404" ]; then
        echo -e "${GREEN}✓ Tenant ID injection prevented${NC}"
    else
        echo -e "${RED}✗ Tenant ID injection possible!${NC}"
    fi
fi

# Test 7: Concurrent tenant operations
echo -e "\n${YELLOW}=== Testing Concurrent Tenant Operations ===${NC}"

echo -e "${BLUE}Sending concurrent requests from different tenants...${NC}"

# Send concurrent creates from different tenants
for i in {1..3}; do
    curl -s -X POST "$REST_API_URL/api/v1/contexts" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_A" \
        -H "Content-Type: application/json" \
        -d '{"agent_id": "concurrent-a-'$i'", "model_id": "model-a"}' > /tmp/tenant_a_$i.json &
    
    curl -s -X POST "$REST_API_URL/api/v1/contexts" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_B" \
        -H "Content-Type: application/json" \
        -d '{"agent_id": "concurrent-b-'$i'", "model_id": "model-b"}' > /tmp/tenant_b_$i.json &
done

wait

# Verify no cross-contamination in responses
contamination=false
for i in {1..3}; do
    if grep -q "$TENANT_B" /tmp/tenant_a_$i.json 2>/dev/null; then
        contamination=true
    fi
    if grep -q "$TENANT_A" /tmp/tenant_b_$i.json 2>/dev/null; then
        contamination=true
    fi
done

if [ "$contamination" = false ]; then
    echo -e "${GREEN}✓ Concurrent operations properly isolated${NC}"
else
    echo -e "${RED}✗ Concurrent operations show cross-contamination${NC}"
fi

# Cleanup
rm -f /tmp/tenant_*.json

# Summary
echo -e "\n${GREEN}✅ Multi-tenant isolation tests completed!${NC}"
echo "======================================"
echo "Critical isolation points tested:"
echo "- Cross-tenant resource access prevention"
echo "- Bulk operation isolation"
echo "- Search result segregation"
echo "- Reference validation across tenants"
echo "- Tenant ID injection prevention"
echo "- Concurrent operation isolation"
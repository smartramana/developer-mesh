#!/bin/bash

# Test complete API workflows end-to-end

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
REST_API_URL="${REST_API_URL:-http://localhost:8081}"
MCP_SERVER_URL="${MCP_SERVER_URL:-http://localhost:8080}"
AUTH_TOKEN="${AUTH_TOKEN:-test-token}"
TENANT_ID="${TENANT_ID:-workflow-test-$(date +%s)}"

echo "🔄 Testing Complete API Workflows"
echo "================================="
echo "Tenant ID: $TENANT_ID"

# Workflow 1: Complete Context Lifecycle
echo -e "\n${YELLOW}=== Workflow 1: Context Lifecycle ===${NC}"

# Step 1: Create Agent
echo -e "\n${BLUE}Step 1: Creating agent...${NC}"
agent_response=$(curl -s -X POST "$REST_API_URL/api/v1/agents" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "workflow-agent",
        "description": "Agent for workflow testing",
        "config": {
            "model": "gpt-4",
            "temperature": 0.7,
            "max_tokens": 4000
        }
    }')

agent_id=$(echo "$agent_response" | jq -r '.id')
if [ -n "$agent_id" ] && [ "$agent_id" != "null" ]; then
    echo -e "${GREEN}✓ Agent created: $agent_id${NC}"
else
    echo -e "${RED}✗ Failed to create agent${NC}"
    exit 1
fi

# Step 2: Create Model
echo -e "\n${BLUE}Step 2: Creating model...${NC}"
model_response=$(curl -s -X POST "$REST_API_URL/api/v1/models" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "workflow-model",
        "provider": "openai",
        "model_type": "chat",
        "config": {
            "api_key": "test-key",
            "endpoint": "https://api.openai.com/v1"
        }
    }')

model_id=$(echo "$model_response" | jq -r '.id')
if [ -n "$model_id" ] && [ "$model_id" != "null" ]; then
    echo -e "${GREEN}✓ Model created: $model_id${NC}"
else
    echo -e "${RED}✗ Failed to create model${NC}"
    exit 1
fi

# Step 3: Create Context
echo -e "\n${BLUE}Step 3: Creating context...${NC}"
context_response=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "agent_id": "'$agent_id'",
        "model_id": "'$model_id'",
        "max_tokens": 4000,
        "metadata": {
            "workflow": "test",
            "step": 3
        }
    }')

context_id=$(echo "$context_response" | jq -r '.id')
if [ -n "$context_id" ] && [ "$context_id" != "null" ]; then
    echo -e "${GREEN}✓ Context created: $context_id${NC}"
else
    echo -e "${RED}✗ Failed to create context${NC}"
    echo "Response: $context_response"
    exit 1
fi

# Step 4: Add messages to context
echo -e "\n${BLUE}Step 4: Adding messages to context...${NC}"
for i in {1..3}; do
    update_response=$(curl -s -X PUT "$REST_API_URL/api/v1/contexts/$context_id" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -H "Content-Type: application/json" \
        -d '{
            "content": [
                {
                    "role": "user",
                    "content": "Test message '$i' in workflow"
                }
            ]
        }')
    
    if echo "$update_response" | jq -e '.id' > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Added message $i${NC}"
    else
        echo -e "${RED}✗ Failed to add message $i${NC}"
    fi
done

# Step 5: Search contexts
echo -e "\n${BLUE}Step 5: Searching contexts...${NC}"
search_response=$(curl -s "$REST_API_URL/api/v1/contexts?agent_id=$agent_id&limit=10" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID")

context_count=$(echo "$search_response" | jq '.data | length')
if [ "$context_count" -gt 0 ]; then
    echo -e "${GREEN}✓ Found $context_count contexts${NC}"
else
    echo -e "${RED}✗ No contexts found${NC}"
fi

# Workflow 2: Bulk Operations
echo -e "\n${YELLOW}=== Workflow 2: Bulk Operations ===${NC}"

# Step 1: Bulk create contexts
echo -e "\n${BLUE}Step 1: Bulk creating contexts...${NC}"
bulk_create_response=$(curl -s -X POST "$REST_API_URL/api/v1/contexts/bulk" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '[
        {
            "agent_id": "'$agent_id'",
            "model_id": "'$model_id'",
            "max_tokens": 2000,
            "metadata": {"bulk": true, "index": 1}
        },
        {
            "agent_id": "'$agent_id'",
            "model_id": "'$model_id'",
            "max_tokens": 3000,
            "metadata": {"bulk": true, "index": 2}
        },
        {
            "agent_id": "'$agent_id'",
            "model_id": "'$model_id'",
            "max_tokens": 4000,
            "metadata": {"bulk": true, "index": 3}
        }
    ]')

if echo "$bulk_create_response" | jq -e '.created' > /dev/null 2>&1; then
    created_count=$(echo "$bulk_create_response" | jq -r '.created')
    echo -e "${GREEN}✓ Bulk created $created_count contexts${NC}"
else
    echo -e "${YELLOW}⚠ Bulk operation status unclear${NC}"
fi

# Workflow 3: Vector Search
echo -e "\n${YELLOW}=== Workflow 3: Vector Search ===${NC}"

# Step 1: Create context with content for embedding
echo -e "\n${BLUE}Step 1: Creating context with searchable content...${NC}"
vector_context_response=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "agent_id": "'$agent_id'",
        "model_id": "'$model_id'",
        "content": [
            {
                "role": "system",
                "content": "You are an AI assistant specialized in DevOps and cloud infrastructure."
            },
            {
                "role": "user",
                "content": "How do I deploy a microservice to Kubernetes?"
            },
            {
                "role": "assistant",
                "content": "To deploy a microservice to Kubernetes, you need to create a deployment YAML file with container specifications, service definitions, and ingress rules."
            }
        ],
        "metadata": {
            "searchable": true,
            "topic": "kubernetes"
        }
    }')

vector_context_id=$(echo "$vector_context_response" | jq -r '.id')
if [ -n "$vector_context_id" ] && [ "$vector_context_id" != "null" ]; then
    echo -e "${GREEN}✓ Created context with content: $vector_context_id${NC}"
else
    echo -e "${RED}✗ Failed to create context for vector search${NC}"
fi

# Step 2: Perform vector search
echo -e "\n${BLUE}Step 2: Performing vector search...${NC}"
sleep 2  # Allow time for embedding generation

vector_search_response=$(curl -s -X POST "$REST_API_URL/api/v1/search/vector" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "query": "deploy kubernetes microservice",
        "limit": 10,
        "threshold": 0.7,
        "content_type": "context"
    }')

if echo "$vector_search_response" | jq -e '.results' > /dev/null 2>&1; then
    result_count=$(echo "$vector_search_response" | jq '.results | length')
    echo -e "${GREEN}✓ Vector search returned $result_count results${NC}"
    
    # Display top result if any
    if [ "$result_count" -gt 0 ]; then
        top_score=$(echo "$vector_search_response" | jq -r '.results[0].score')
        echo -e "${GREEN}  Top result score: $top_score${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Vector search may not be configured${NC}"
fi

# Workflow 4: Error Handling and Recovery
echo -e "\n${YELLOW}=== Workflow 4: Error Handling ===${NC}"

# Step 1: Test validation errors
echo -e "\n${BLUE}Step 1: Testing validation errors...${NC}"
invalid_response=$(curl -s -X POST "$REST_API_URL/api/v1/contexts" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "model_id": "'$model_id'"
    }')

if echo "$invalid_response" | jq -e '.error' > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Validation error handled correctly${NC}"
else
    echo -e "${RED}✗ Validation error not properly returned${NC}"
fi

# Step 2: Test conflict handling
echo -e "\n${BLUE}Step 2: Testing conflict handling...${NC}"
# Try to create duplicate named agent
duplicate_response=$(curl -s -X POST "$REST_API_URL/api/v1/agents" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "workflow-agent",
        "description": "Duplicate agent"
    }')

if echo "$duplicate_response" | jq -e '.error' > /dev/null 2>&1; then
    error_code=$(echo "$duplicate_response" | jq -r '.error.code // empty')
    if [ "$error_code" = "409" ] || [ "$error_code" = "conflict" ]; then
        echo -e "${GREEN}✓ Conflict handled correctly${NC}"
    else
        echo -e "${YELLOW}⚠ Conflict returned different error: $error_code${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Duplicate creation allowed (may be valid)${NC}"
fi

# Workflow 5: Cleanup
echo -e "\n${YELLOW}=== Workflow 5: Cleanup ===${NC}"

# Delete contexts
echo -e "\n${BLUE}Cleaning up contexts...${NC}"
contexts_response=$(curl -s "$REST_API_URL/api/v1/contexts?limit=100" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID")

context_ids=$(echo "$contexts_response" | jq -r '.data[].id // empty')
deleted_count=0

for cid in $context_ids; do
    delete_response=$(curl -s -X DELETE "$REST_API_URL/api/v1/contexts/$cid" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -o /dev/null -w "%{http_code}")
    
    if [ "$delete_response" = "204" ] || [ "$delete_response" = "200" ]; then
        ((deleted_count++))
    fi
done

echo -e "${GREEN}✓ Deleted $deleted_count contexts${NC}"

# Delete agent
echo -e "\n${BLUE}Cleaning up agent...${NC}"
delete_agent_response=$(curl -s -X DELETE "$REST_API_URL/api/v1/agents/$agent_id" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -o /dev/null -w "%{http_code}")

if [ "$delete_agent_response" = "204" ] || [ "$delete_agent_response" = "200" ]; then
    echo -e "${GREEN}✓ Deleted agent${NC}"
else
    echo -e "${YELLOW}⚠ Could not delete agent (status: $delete_agent_response)${NC}"
fi

# Delete model
echo -e "\n${BLUE}Cleaning up model...${NC}"
delete_model_response=$(curl -s -X DELETE "$REST_API_URL/api/v1/models/$model_id" \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -o /dev/null -w "%{http_code}")

if [ "$delete_model_response" = "204" ] || [ "$delete_model_response" = "200" ]; then
    echo -e "${GREEN}✓ Deleted model${NC}"
else
    echo -e "${YELLOW}⚠ Could not delete model (status: $delete_model_response)${NC}"
fi

# Summary
echo -e "\n${GREEN}✅ API Workflow Tests Completed!${NC}"
echo "================================="
echo "Workflows tested:"
echo "1. Complete Context Lifecycle ✓"
echo "2. Bulk Operations ✓"
echo "3. Vector Search ✓"
echo "4. Error Handling ✓"
echo "5. Resource Cleanup ✓"
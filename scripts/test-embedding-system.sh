#!/bin/bash
set -e

# Multi-Agent Embedding System Test Script
# This script tests the complete embedding system including REST API and MCP Server

# Configuration
API_URL="${REST_API_URL:-http://localhost:8081/api}"
MCP_URL="${MCP_SERVER_URL:-http://localhost:8080/api/v1}"
API_KEY="${API_KEY:-dev-api-key}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

check_response() {
    local response="$1"
    local expected_status="$2"
    local description="$3"
    
    local status=$(echo "$response" | head -n 1 | cut -d' ' -f2)
    
    if [ "$status" = "$expected_status" ]; then
        log_success "$description - Status: $status"
        return 0
    else
        log_error "$description - Expected: $expected_status, Got: $status"
        echo "$response" | tail -n +2 | jq . 2>/dev/null || echo "$response"
        return 1
    fi
}

# Check prerequisites
command -v curl >/dev/null 2>&1 || { log_error "curl is required but not installed."; exit 1; }
command -v jq >/dev/null 2>&1 || { log_error "jq is required but not installed."; exit 1; }

echo "============================================"
echo "=== Testing Multi-Agent Embedding System ==="
echo "============================================"
echo ""
log_info "REST API URL: $API_URL"
log_info "MCP Server URL: $MCP_URL"
log_info "API Key: ${API_KEY:0:10}..."
echo ""

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Generate unique agent ID for this test run
AGENT_ID="test-agent-$(date +%s)"
log_info "Using agent ID: $AGENT_ID"
echo ""

# 1. Health check
echo "=== 1. Provider Health Check ==="
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$API_URL/embeddings/providers/health" \
  -H "X-API-Key: $API_KEY")

if check_response "$RESPONSE" "200" "Provider health check"; then
    HEALTH_DATA=$(echo "$RESPONSE" | tail -n +2)
    echo "$HEALTH_DATA" | jq -r '.providers | to_entries[] | "\(.key): \(.value.status)"' | while read line; do
        log_info "  $line"
    done
    
    # Check if at least one provider is healthy
    HEALTHY_COUNT=$(echo "$HEALTH_DATA" | jq '[.providers[] | select(.status == "healthy")] | length')
    if [ "$HEALTHY_COUNT" -gt 0 ]; then
        log_success "Found $HEALTHY_COUNT healthy provider(s)"
        ((TESTS_PASSED++))
    else
        log_warning "No healthy providers found - some tests may fail"
        ((TESTS_FAILED++))
    fi
else
    ((TESTS_FAILED++))
fi
echo ""

# 2. Create agent configuration
echo "=== 2. Creating Agent Configuration ==="
AGENT_CONFIG='{
  "agent_id": "'$AGENT_ID'",
  "embedding_strategy": "quality",
  "model_preferences": [{
    "task_type": "general_qa",
    "primary_models": ["text-embedding-3-large"],
    "fallback_models": ["text-embedding-3-small", "text-embedding-ada-002"]
  }],
  "constraints": {
    "max_cost_per_month_usd": 100.0,
    "rate_limits": {
      "requests_per_minute": 100
    }
  }
}'

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/embeddings/agents" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "$AGENT_CONFIG")

if check_response "$RESPONSE" "201" "Create agent configuration"; then
    CONFIG_DATA=$(echo "$RESPONSE" | tail -n +2)
    log_info "Agent created with ID: $(echo "$CONFIG_DATA" | jq -r .agent_id)"
    log_info "Strategy: $(echo "$CONFIG_DATA" | jq -r .embedding_strategy)"
    ((TESTS_PASSED++))
else
    ((TESTS_FAILED++))
    log_error "Failed to create agent - subsequent tests may fail"
fi
echo ""

# 3. Generate embedding via REST
echo "=== 3. Generating Embedding via REST API ==="
EMBEDDING_REQUEST='{
  "agent_id": "'$AGENT_ID'",
  "text": "What is the capital of France? Paris is a beautiful city known for its art, culture, and cuisine.",
  "task_type": "general_qa",
  "metadata": {
    "test": true,
    "source": "manual_test"
  }
}'

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/embeddings" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "$EMBEDDING_REQUEST")

if check_response "$RESPONSE" "200" "Generate embedding"; then
    EMBEDDING_DATA=$(echo "$RESPONSE" | tail -n +2)
    EMBEDDING_ID=$(echo "$EMBEDDING_DATA" | jq -r .embedding_id)
    
    log_info "Embedding ID: $EMBEDDING_ID"
    log_info "Model used: $(echo "$EMBEDDING_DATA" | jq -r .model_used)"
    log_info "Provider: $(echo "$EMBEDDING_DATA" | jq -r .provider)"
    log_info "Dimensions: $(echo "$EMBEDDING_DATA" | jq -r .dimensions)"
    log_info "Cost (USD): $(echo "$EMBEDDING_DATA" | jq -r .cost_usd)"
    log_info "Generation time: $(echo "$EMBEDDING_DATA" | jq -r .generation_time_ms)ms"
    log_info "Cached: $(echo "$EMBEDDING_DATA" | jq -r .cached)"
    ((TESTS_PASSED++))
else
    ((TESTS_FAILED++))
fi
echo ""

# 4. Batch generate embeddings
echo "=== 4. Batch Generate Embeddings ==="
BATCH_REQUEST='[
  {
    "agent_id": "'$AGENT_ID'",
    "text": "Machine learning is transforming how we build software",
    "task_type": "general_qa"
  },
  {
    "agent_id": "'$AGENT_ID'",
    "text": "Natural language processing enables computers to understand human language",
    "task_type": "general_qa"
  },
  {
    "agent_id": "'$AGENT_ID'",
    "text": "Deep learning models can recognize patterns in complex data",
    "task_type": "general_qa"
  }
]'

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/embeddings/batch" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "$BATCH_REQUEST")

if check_response "$RESPONSE" "200" "Batch generate embeddings"; then
    BATCH_DATA=$(echo "$RESPONSE" | tail -n +2)
    COUNT=$(echo "$BATCH_DATA" | jq -r .count)
    log_info "Generated $COUNT embeddings in batch"
    
    # Show summary of batch results
    echo "$BATCH_DATA" | jq -r '.embeddings[] | "  - ID: \(.embedding_id | .[0:8])... Model: \(.model_used) Time: \(.generation_time_ms)ms"'
    ((TESTS_PASSED++))
else
    ((TESTS_FAILED++))
fi
echo ""

# 5. Test MCP integration
echo "=== 5. Testing MCP Embedding Generation ==="
MCP_REQUEST='{
  "action": "generate_embedding",
  "agent_id": "'$AGENT_ID'",
  "text": "The Eiffel Tower is an iconic landmark in Paris, France"
}'

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$MCP_URL/request" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "$MCP_REQUEST")

STATUS=$(echo "$RESPONSE" | tail -n 1)
if [ "$STATUS" = "200" ]; then
    log_success "MCP embedding generation - Status: $STATUS"
    MCP_DATA=$(echo "$RESPONSE" | tail -n +2)
    log_info "Embedding ID: $(echo "$MCP_DATA" | jq -r .embedding_id)"
    log_info "Model used: $(echo "$MCP_DATA" | jq -r .model_used)"
    ((TESTS_PASSED++))
elif [ "$STATUS" = "404" ] || [ "$STATUS" = "000" ]; then
    log_warning "MCP server not available - skipping MCP tests"
else
    log_error "MCP embedding generation failed - Status: $STATUS"
    echo "$RESPONSE" | tail -n +2 | jq . 2>/dev/null || echo "$RESPONSE"
    ((TESTS_FAILED++))
fi
echo ""

# 6. Search test
echo "=== 6. Testing Search Functionality ==="
SEARCH_REQUEST='{
  "agent_id": "'$AGENT_ID'",
  "query": "capital city France Paris",
  "limit": 5,
  "threshold": 0.5
}'

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/embeddings/search" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "$SEARCH_REQUEST")

STATUS=$(echo "$RESPONSE" | tail -n 1)
if [ "$STATUS" = "200" ]; then
    log_success "Search embeddings - Status: $STATUS"
    SEARCH_DATA=$(echo "$RESPONSE" | tail -n +2)
    RESULTS_COUNT=$(echo "$SEARCH_DATA" | jq '.results | length')
    log_info "Found $RESULTS_COUNT results"
    ((TESTS_PASSED++))
elif [ "$STATUS" = "501" ]; then
    log_warning "Search functionality not yet implemented"
else
    log_error "Search failed - Status: $STATUS"
    echo "$RESPONSE" | tail -n +2 | jq . 2>/dev/null || echo "$RESPONSE"
    ((TESTS_FAILED++))
fi
echo ""

# 7. Get agent configuration
echo "=== 7. Retrieving Agent Configuration ==="
RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$API_URL/embeddings/agents/$AGENT_ID" \
  -H "X-API-Key: $API_KEY")

if check_response "$RESPONSE" "200" "Get agent configuration"; then
    CONFIG_DATA=$(echo "$RESPONSE" | tail -n +2)
    log_info "Agent ID: $(echo "$CONFIG_DATA" | jq -r .agent_id)"
    log_info "Strategy: $(echo "$CONFIG_DATA" | jq -r .embedding_strategy)"
    log_info "Active: $(echo "$CONFIG_DATA" | jq -r .is_active)"
    ((TESTS_PASSED++))
else
    ((TESTS_FAILED++))
fi
echo ""

# 8. Update agent configuration
echo "=== 8. Updating Agent Configuration ==="
UPDATE_REQUEST='{
  "embedding_strategy": "balanced"
}'

RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT "$API_URL/embeddings/agents/$AGENT_ID" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "$UPDATE_REQUEST")

if check_response "$RESPONSE" "200" "Update agent configuration"; then
    UPDATE_DATA=$(echo "$RESPONSE" | tail -n +2)
    NEW_STRATEGY=$(echo "$UPDATE_DATA" | jq -r .embedding_strategy)
    if [ "$NEW_STRATEGY" = "balanced" ]; then
        log_success "Strategy updated to: $NEW_STRATEGY"
        ((TESTS_PASSED++))
    else
        log_error "Strategy update failed - expected 'balanced', got '$NEW_STRATEGY'"
        ((TESTS_FAILED++))
    fi
else
    ((TESTS_FAILED++))
fi
echo ""

# 9. Test error handling
echo "=== 9. Testing Error Handling ==="

# Test with non-existent agent
log_info "Testing with non-existent agent..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/embeddings" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{"agent_id": "non-existent-agent", "text": "test"}')

STATUS=$(echo "$RESPONSE" | tail -n 1)
if [ "$STATUS" -ge "400" ] && [ "$STATUS" -lt "500" ]; then
    log_success "Correctly rejected non-existent agent - Status: $STATUS"
    ((TESTS_PASSED++))
else
    log_error "Should have rejected non-existent agent - Status: $STATUS"
    ((TESTS_FAILED++))
fi

# Test with missing required field
log_info "Testing with missing agent_id..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/embeddings" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{"text": "test without agent"}')

STATUS=$(echo "$RESPONSE" | tail -n 1)
if [ "$STATUS" = "400" ]; then
    log_success "Correctly rejected missing agent_id - Status: $STATUS"
    ((TESTS_PASSED++))
else
    log_error "Should have rejected missing agent_id - Status: $STATUS"
    ((TESTS_FAILED++))
fi
echo ""

# 10. Test cross-model search (if available)
echo "=== 10. Testing Cross-Model Search ==="
CROSS_MODEL_REQUEST='{
  "query": "artificial intelligence machine learning",
  "search_model": "text-embedding-3-small",
  "include_models": ["text-embedding-3-small", "text-embedding-3-large"],
  "limit": 10,
  "min_similarity": 0.5
}'

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/embeddings/search/cross-model" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "$CROSS_MODEL_REQUEST")

STATUS=$(echo "$RESPONSE" | tail -n 1)
if [ "$STATUS" = "200" ]; then
    log_success "Cross-model search - Status: $STATUS"
    CROSS_MODEL_DATA=$(echo "$RESPONSE" | tail -n +2)
    log_info "Results from search: $(echo "$CROSS_MODEL_DATA" | jq -r '.results | length') items"
    ((TESTS_PASSED++))
elif [ "$STATUS" = "501" ]; then
    log_warning "Cross-model search not yet implemented"
else
    log_error "Cross-model search failed - Status: $STATUS"
    ((TESTS_FAILED++))
fi
echo ""

# Summary
echo "============================================"
echo "=== Test Summary ==="
echo "============================================"
log_info "Total tests: $((TESTS_PASSED + TESTS_FAILED))"
log_success "Passed: $TESTS_PASSED"
if [ $TESTS_FAILED -gt 0 ]; then
    log_error "Failed: $TESTS_FAILED"
else
    log_info "Failed: 0"
fi
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    log_success "All tests completed successfully! 🎉"
    exit 0
else
    log_error "Some tests failed. Please check the output above."
    exit 1
fi
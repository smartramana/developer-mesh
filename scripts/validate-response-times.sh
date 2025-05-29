#!/bin/bash

# Validate API response times meet SLA requirements

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
REST_API_URL="${REST_API_URL:-http://localhost:8081}"
MCP_SERVER_URL="${MCP_SERVER_URL:-http://localhost:8080}"
MAX_RESPONSE_TIME_MS=100
ITERATIONS=100

echo "⏱️  Validating API Response Times (SLA: <${MAX_RESPONSE_TIME_MS}ms)"
echo "================================================"

# Function to measure response time
measure_endpoint() {
    local url=$1
    local method=$2
    local name=$3
    local data=$4
    
    echo -e "\n${YELLOW}Testing: $name${NC}"
    
    times=()
    failures=0
    
    for i in $(seq 1 $ITERATIONS); do
        if [ -z "$data" ]; then
            response_time=$(curl -s -o /dev/null -w "%{time_total}" \
                -X "$method" \
                -H "Authorization: Bearer test-token" \
                -H "X-Tenant-ID: test-tenant" \
                "$url")
        else
            response_time=$(curl -s -o /dev/null -w "%{time_total}" \
                -X "$method" \
                -H "Authorization: Bearer test-token" \
                -H "X-Tenant-ID: test-tenant" \
                -H "Content-Type: application/json" \
                -d "$data" \
                "$url")
        fi
        
        # Convert to milliseconds
        response_time_ms=$(echo "$response_time * 1000" | bc)
        times+=("$response_time_ms")
        
        if (( $(echo "$response_time_ms > $MAX_RESPONSE_TIME_MS" | bc -l) )); then
            ((failures++))
        fi
        
        # Progress indicator
        if [ $((i % 10)) -eq 0 ]; then
            echo -n "."
        fi
    done
    echo
    
    # Calculate statistics
    avg_time=$(printf '%s\n' "${times[@]}" | awk '{sum+=$1} END {print sum/NR}')
    min_time=$(printf '%s\n' "${times[@]}" | sort -n | head -1)
    max_time=$(printf '%s\n' "${times[@]}" | sort -n | tail -1)
    
    # Calculate p50, p95, p99
    sorted_times=($(printf '%s\n' "${times[@]}" | sort -n))
    p50_index=$((ITERATIONS * 50 / 100))
    p95_index=$((ITERATIONS * 95 / 100))
    p99_index=$((ITERATIONS * 99 / 100))
    
    p50="${sorted_times[$p50_index]}"
    p95="${sorted_times[$p95_index]}"
    p99="${sorted_times[$p99_index]}"
    
    # Display results
    echo "  Average: ${avg_time}ms"
    echo "  Min: ${min_time}ms / Max: ${max_time}ms"
    echo "  P50: ${p50}ms / P95: ${p95}ms / P99: ${p99}ms"
    echo "  Failures (>${MAX_RESPONSE_TIME_MS}ms): $failures/$ITERATIONS"
    
    # Check SLA
    if (( $(echo "$p99 <= $MAX_RESPONSE_TIME_MS" | bc -l) )); then
        echo -e "  ${GREEN}✓ PASSED: P99 within SLA${NC}"
        return 0
    else
        echo -e "  ${RED}✗ FAILED: P99 exceeds SLA${NC}"
        return 1
    fi
}

# Test endpoints
all_passed=true

# REST API endpoints
measure_endpoint "$REST_API_URL/health" "GET" "REST API Health Check" || all_passed=false
measure_endpoint "$REST_API_URL/api/v1/agents" "GET" "List Agents" || all_passed=false
measure_endpoint "$REST_API_URL/api/v1/models" "GET" "List Models" || all_passed=false
measure_endpoint "$REST_API_URL/api/v1/contexts" "GET" "List Contexts" || all_passed=false

# Test write operations
context_data='{
    "agent_id": "perf-test-agent",
    "model_id": "perf-test-model",
    "metadata": {"performance_test": true}
}'
measure_endpoint "$REST_API_URL/api/v1/contexts" "POST" "Create Context" "$context_data" || all_passed=false

# MCP Server endpoints
measure_endpoint "$MCP_SERVER_URL/health" "GET" "MCP Server Health Check" || all_passed=false

# Vector search endpoint (if available)
vector_search_data='{
    "query": "test query",
    "limit": 10,
    "threshold": 0.7
}'
measure_endpoint "$REST_API_URL/api/v1/search/vector" "POST" "Vector Search" "$vector_search_data" || all_passed=false

echo -e "\n================================================"
if [ "$all_passed" = true ]; then
    echo -e "${GREEN}✅ All endpoints meet response time SLA!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some endpoints exceed response time SLA${NC}"
    exit 1
fi
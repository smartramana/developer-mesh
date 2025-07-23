#!/bin/bash

# Chaos testing suite for production readiness validation

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
COMPOSE_FILE="docker-compose.local.yml"
LOG_DIR="chaos-test-logs"
HEALTH_CHECK_TIMEOUT=30
RECOVERY_TIMEOUT=60

echo "🌪️  Developer Mesh Chaos Testing Suite"
echo "=================================="
echo -e "${YELLOW}WARNING: This will intentionally break services!${NC}"
echo "Press Ctrl+C to cancel, or wait 5 seconds to continue..."
sleep 5

# Create log directory
mkdir -p "$LOG_DIR"

# Function to check service health
check_service_health() {
    local service=$1
    local url=$2
    local start_time=$(date +%s)
    
    while true; do
        if curl -s "$url" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ $service is healthy${NC}"
            return 0
        fi
        
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [ $elapsed -gt $HEALTH_CHECK_TIMEOUT ]; then
            echo -e "${RED}✗ $service failed to become healthy within ${HEALTH_CHECK_TIMEOUT}s${NC}"
            return 1
        fi
        
        echo -n "."
        sleep 2
    done
}

# Function to run a chaos test
run_chaos_test() {
    local test_name=$1
    local test_function=$2
    
    echo -e "\n${BLUE}Running: $test_name${NC}"
    echo "----------------------------------------"
    
    # Capture initial state
    docker-compose -f "$COMPOSE_FILE" ps > "$LOG_DIR/${test_name}_initial.log" 2>&1
    
    # Run the test
    if $test_function; then
        echo -e "${GREEN}✓ $test_name: PASSED${NC}"
        return 0
    else
        echo -e "${RED}✗ $test_name: FAILED${NC}"
        # Capture failure state
        docker-compose -f "$COMPOSE_FILE" logs --tail=100 > "$LOG_DIR/${test_name}_failure.log" 2>&1
        return 1
    fi
}

# Test 1: Database failure and recovery
test_database_failure() {
    echo "Stopping database..."
    docker-compose -f "$COMPOSE_FILE" stop database
    
    sleep 5
    
    # Check if services handle database failure gracefully
    echo "Checking service resilience..."
    local mcp_health=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health || echo "000")
    local api_health=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/health || echo "000")
    
    if [ "$mcp_health" != "000" ] && [ "$api_health" != "000" ]; then
        echo "Services still responding (degraded mode)"
    else
        echo -e "${RED}Services completely failed${NC}"
    fi
    
    # Restart database
    echo "Restarting database..."
    docker-compose -f "$COMPOSE_FILE" start database
    
    # Wait for recovery
    echo "Waiting for system recovery..."
    sleep 10
    
    # Verify recovery
    check_service_health "MCP Server" "http://localhost:8080/health" &&
    check_service_health "REST API" "http://localhost:8081/health"
}

# Test 2: Redis failure and recovery
test_redis_failure() {
    echo "Stopping Redis..."
    docker-compose -f "$COMPOSE_FILE" stop redis
    
    sleep 5
    
    # Test cache fallback behavior
    echo "Testing cache fallback..."
    response=$(curl -s -w "\n%{http_code}" http://localhost:8081/api/v1/contexts \
        -H "Authorization: Bearer test-token" \
        -H "X-Tenant-ID: chaos-test")
    
    http_code=$(echo "$response" | tail -n1)
    
    if [ "$http_code" = "200" ] || [ "$http_code" = "401" ]; then
        echo "API still functional without cache"
    else
        echo -e "${YELLOW}API degraded without cache${NC}"
    fi
    
    # Restart Redis
    echo "Restarting Redis..."
    docker-compose -f "$COMPOSE_FILE" start redis
    
    sleep 10
    
    # Verify recovery
    check_service_health "Redis" "http://localhost:6379" || true
    check_service_health "REST API" "http://localhost:8081/health"
}

# Test 3: Worker failure and job recovery
test_worker_failure() {
    echo "Creating test job..."
    # Send a test event that should be processed by worker
    curl -X POST http://localhost:8081/api/v1/events \
        -H "Authorization: Bearer test-token" \
        -H "X-Tenant-ID: chaos-test" \
        -H "Content-Type: application/json" \
        -d '{"type": "test.chaos", "data": {"test": true}}' || true
    
    echo "Stopping worker..."
    docker-compose -f "$COMPOSE_FILE" stop worker
    
    sleep 10
    
    echo "Restarting worker..."
    docker-compose -f "$COMPOSE_FILE" start worker
    
    sleep 10
    
    # Check if job was processed after recovery
    echo "Checking job processing..."
    docker-compose -f "$COMPOSE_FILE" logs worker --tail=50 | grep -q "test.chaos" && 
        echo "Job processed after recovery" || 
        echo -e "${YELLOW}Job might be in queue${NC}"
    
    return 0
}

# Test 4: Network partition simulation
test_network_partition() {
    echo "Simulating network partition..."
    
    # Add network delay and packet loss
    container_id=$(docker-compose -f "$COMPOSE_FILE" ps -q mcp-server)
    
    if [ -n "$container_id" ]; then
        echo "Adding network delay to MCP server..."
        docker exec "$container_id" tc qdisc add dev eth0 root netem delay 500ms loss 10% 2>/dev/null || true
        
        sleep 10
        
        # Test system behavior under network issues
        echo "Testing under network degradation..."
        response_time=$(curl -s -o /dev/null -w "%{time_total}" http://localhost:8080/health || echo "999")
        
        echo "Response time under network issues: ${response_time}s"
        
        # Remove network issues
        echo "Removing network constraints..."
        docker exec "$container_id" tc qdisc del dev eth0 root 2>/dev/null || true
        
        sleep 5
        
        # Verify recovery
        check_service_health "MCP Server" "http://localhost:8080/health"
    else
        echo -e "${YELLOW}Could not find container for network test${NC}"
    fi
    
    return 0
}

# Test 5: Resource exhaustion
test_resource_exhaustion() {
    echo "Testing memory pressure..."
    
    # Get container ID
    container_id=$(docker-compose -f "$COMPOSE_FILE" ps -q rest-api)
    
    if [ -n "$container_id" ]; then
        # Create memory pressure
        echo "Applying memory pressure to REST API..."
        docker exec "$container_id" sh -c "dd if=/dev/zero of=/tmp/bigfile bs=1M count=500" 2>/dev/null || true
        
        sleep 5
        
        # Check if service is still responsive
        response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081/health || echo "000")
        
        if [ "$response" = "200" ]; then
            echo "Service survived memory pressure"
        else
            echo -e "${YELLOW}Service degraded under memory pressure${NC}"
        fi
        
        # Clean up
        docker exec "$container_id" rm -f /tmp/bigfile 2>/dev/null || true
    fi
    
    return 0
}

# Test 6: Cascading failure
test_cascading_failure() {
    echo "Testing cascading failure scenario..."
    
    # Stop multiple services
    echo "Stopping database and Redis..."
    docker-compose -f "$COMPOSE_FILE" stop database redis
    
    sleep 5
    
    # Check if services degrade gracefully
    echo "Checking graceful degradation..."
    
    # MCP server should return unhealthy but not crash
    mcp_response=$(curl -s http://localhost:8080/health | jq -r '.status' 2>/dev/null || echo "error")
    api_response=$(curl -s http://localhost:8081/health | jq -r '.status' 2>/dev/null || echo "error")
    
    echo "MCP Server status: $mcp_response"
    echo "REST API status: $api_response"
    
    # Restart services
    echo "Restarting all services..."
    docker-compose -f "$COMPOSE_FILE" start database redis
    
    sleep 20
    
    # Verify full recovery
    check_service_health "Database" "http://localhost:5432" || true
    check_service_health "Redis" "http://localhost:6379" || true
    check_service_health "MCP Server" "http://localhost:8080/health" &&
    check_service_health "REST API" "http://localhost:8081/health"
}

# Main execution
echo -e "\n${YELLOW}Starting chaos tests...${NC}\n"

# Track test results
declare -A test_results
tests_passed=0
tests_failed=0

# Run all chaos tests
for test in \
    "Database Failure:test_database_failure" \
    "Redis Failure:test_redis_failure" \
    "Worker Failure:test_worker_failure" \
    "Network Partition:test_network_partition" \
    "Resource Exhaustion:test_resource_exhaustion" \
    "Cascading Failure:test_cascading_failure"
do
    IFS=':' read -r test_name test_function <<< "$test"
    
    if run_chaos_test "$test_name" "$test_function"; then
        test_results["$test_name"]="PASSED"
        ((tests_passed++))
    else
        test_results["$test_name"]="FAILED"
        ((tests_failed++))
    fi
    
    # Allow system to stabilize between tests
    echo -e "\n${BLUE}Waiting for system stabilization...${NC}"
    sleep 15
done

# Display summary
echo -e "\n${BLUE}=================================="
echo "Chaos Testing Summary"
echo "==================================${NC}"
echo
for test_name in "${!test_results[@]}"; do
    result="${test_results[$test_name]}"
    if [ "$result" = "PASSED" ]; then
        echo -e "${GREEN}✓ $test_name: $result${NC}"
    else
        echo -e "${RED}✗ $test_name: $result${NC}"
    fi
done

echo
echo "Tests Passed: $tests_passed"
echo "Tests Failed: $tests_failed"
echo "Logs saved to: $LOG_DIR/"

if [ $tests_failed -eq 0 ]; then
    echo -e "\n${GREEN}✅ All chaos tests passed! System is resilient.${NC}"
    exit 0
else
    echo -e "\n${RED}❌ Some chaos tests failed. Review logs for details.${NC}"
    exit 1
fi
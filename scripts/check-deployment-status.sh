#!/bin/bash

# Script to check deployment status of Developer Mesh services
# Can be run locally or on the EC2 instance

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "======================================="
echo "Developer Mesh Deployment Status Check"
echo "======================================="
echo ""

# Function to check service health
check_service() {
    local name=$1
    local url=$2
    local expected_status=${3:-200}
    
    printf "Checking %s... " "$name"
    
    response=$(curl -s -o /dev/null -w "%{http_code}" "$url" || echo "000")
    
    if [ "$response" = "$expected_status" ]; then
        echo -e "${GREEN}✓${NC} (HTTP $response)"
        return 0
    else
        echo -e "${RED}✗${NC} (HTTP $response)"
        return 1
    fi
}

# Function to check WebSocket endpoint
check_websocket() {
    local name=$1
    local url=$2
    
    printf "Checking %s... " "$name"
    
    response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Connection: Upgrade" \
        -H "Upgrade: websocket" \
        -H "Sec-WebSocket-Version: 13" \
        -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
        -H "Sec-WebSocket-Protocol: mcp.v1" \
        "$url" || echo "000")
    
    # 101 = Switching Protocols (success)
    # 401 = Unauthorized (endpoint exists but needs auth)
    # 426 = Upgrade Required (endpoint exists)
    if [ "$response" = "101" ] || [ "$response" = "401" ] || [ "$response" = "426" ]; then
        echo -e "${GREEN}✓${NC} (HTTP $response)"
        return 0
    elif [ "$response" = "502" ]; then
        echo -e "${RED}✗${NC} (HTTP $response - Backend not reachable)"
        return 1
    else
        echo -e "${RED}✗${NC} (HTTP $response)"
        return 1
    fi
}

# Function to check Docker containers
check_containers() {
    echo -e "\n${YELLOW}Docker Containers:${NC}"
    
    if command -v docker &> /dev/null; then
        echo "Looking for Developer Mesh containers..."
        containers=$(docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "mcp-server|rest-api|worker" || echo "")
        
        if [ -z "$containers" ]; then
            echo -e "${RED}No Developer Mesh containers found running${NC}"
            echo ""
            echo "To start the services:"
            echo "  docker-compose -f docker-compose.production.yml up -d"
        else
            echo -e "${GREEN}Found running containers:${NC}"
            echo "$containers"
        fi
    else
        echo "Docker not available on this system"
    fi
}

# Main checks
echo -e "${YELLOW}External Endpoints:${NC}"
check_service "API Health" "https://api.dev-mesh.io/health"
check_service "MCP Health" "https://mcp.dev-mesh.io/health"
check_websocket "WebSocket" "https://mcp.dev-mesh.io/ws"

echo -e "\n${YELLOW}Local Services (if on EC2):${NC}"
check_service "Local API" "http://localhost:8081/health"
check_service "Local MCP" "http://localhost:8080/health"
check_websocket "Local WebSocket" "http://localhost:8080/ws"

# Check containers
check_containers

# Check nginx
echo -e "\n${YELLOW}Nginx Status:${NC}"
if command -v nginx &> /dev/null; then
    if sudo nginx -t 2>/dev/null; then
        echo -e "${GREEN}✓${NC} Nginx configuration is valid"
    else
        echo -e "${RED}✗${NC} Nginx configuration has errors"
    fi
    
    if systemctl is-active --quiet nginx; then
        echo -e "${GREEN}✓${NC} Nginx is running"
    else
        echo -e "${RED}✗${NC} Nginx is not running"
    fi
else
    echo "Nginx not available on this system"
fi

echo ""
echo "======================================="
echo "Status check complete!"
echo "======================================="
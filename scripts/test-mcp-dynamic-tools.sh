#!/bin/bash

# Test MCP dynamic tools integration, specifically GitHub tool
# This script validates that dynamic tools from REST API are exposed via MCP protocol

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

MCP_URL="ws://localhost:8080/ws"
API_KEY="dev-admin-key-1234567890"
TENANT_ID="00000000-0000-0000-0000-000000000001"

echo -e "${BLUE}=== Testing MCP Dynamic Tools Integration ===${NC}"
echo -e "${YELLOW}Endpoint: $MCP_URL${NC}"
echo

# Function to send MCP message and get response
send_mcp_message() {
    local message="$1"
    echo "$message" | websocat --one-message --header="Authorization: Bearer $API_KEY" "$MCP_URL" 2>/dev/null
}

# Test 1: Initialize connection
echo -e "${YELLOW}1. Initializing MCP connection...${NC}"
INIT_REQUEST='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test-dynamic-tools","version":"1.0.0"}}}'
INIT_RESPONSE=$(send_mcp_message "$INIT_REQUEST")

if echo "$INIT_RESPONSE" | grep -q '"protocolVersion":"2025-06-18"'; then
    echo -e "${GREEN}✓ Connection initialized successfully${NC}"
else
    echo -e "${RED}✗ Failed to initialize connection${NC}"
    echo "Response: $INIT_RESPONSE"
    exit 1
fi

# Test 2: Confirm initialization
echo -e "${YELLOW}2. Confirming initialization...${NC}"
INITIALIZED_REQUEST='{"jsonrpc":"2.0","id":2,"method":"initialized","params":{}}'
send_mcp_message "$INITIALIZED_REQUEST" > /dev/null

# Test 3: List tools and check for GitHub
echo -e "${YELLOW}3. Listing tools and checking for GitHub...${NC}"
TOOLS_REQUEST='{"jsonrpc":"2.0","id":3,"method":"tools/list"}'
TOOLS_RESPONSE=$(send_mcp_message "$TOOLS_REQUEST")

# Check if response contains tools
if echo "$TOOLS_RESPONSE" | grep -q '"tools"'; then
    echo -e "${GREEN}✓ Tools list retrieved successfully${NC}"
    
    # Check for DevMesh tools
    if echo "$TOOLS_RESPONSE" | grep -q '"devmesh\.'; then
        echo -e "${GREEN}✓ DevMesh native tools found${NC}"
    else
        echo -e "${YELLOW}⚠ No DevMesh native tools found${NC}"
    fi
    
    # Check for adapter tools (legacy protocol)
    if echo "$TOOLS_RESPONSE" | grep -q '"agent\.'; then
        echo -e "${GREEN}✓ Adapter tools (legacy protocol) found${NC}"
    else
        echo -e "${YELLOW}⚠ No adapter tools found${NC}"
    fi
    
    # Check for GitHub dynamic tool
    if echo "$TOOLS_RESPONSE" | grep -q '"GitHub"' || echo "$TOOLS_RESPONSE" | grep -q '"github"'; then
        echo -e "${GREEN}✓ GitHub dynamic tool found in MCP tools list!${NC}"
        
        # Extract and display GitHub tool details
        echo -e "${BLUE}GitHub tool details:${NC}"
        echo "$TOOLS_RESPONSE" | python3 -m json.tool 2>/dev/null | grep -A 5 -i github || true
    else
        echo -e "${RED}✗ GitHub tool NOT found in MCP tools list${NC}"
        echo -e "${YELLOW}Available tools:${NC}"
        echo "$TOOLS_RESPONSE" | python3 -c "
import json
import sys
data = json.load(sys.stdin)
if 'result' in data and 'tools' in data['result']:
    for tool in data['result']['tools']:
        print(f\"  - {tool.get('name', 'unknown')}\")" 2>/dev/null || echo "$TOOLS_RESPONSE"
    fi
else
    echo -e "${RED}✗ Failed to retrieve tools list${NC}"
    echo "Response: $TOOLS_RESPONSE"
    exit 1
fi

echo

# Test 4: Test GitHub tool execution (if found)
if echo "$TOOLS_RESPONSE" | grep -q '"GitHub"' || echo "$TOOLS_RESPONSE" | grep -q '"github"'; then
    echo -e "${YELLOW}4. Testing GitHub tool execution...${NC}"
    
    # Try to call the GitHub tool to list repositories
    GITHUB_CALL_REQUEST='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"github","arguments":{"action":"list_repos","owner":"anthropics"}}}'
    GITHUB_RESPONSE=$(send_mcp_message "$GITHUB_CALL_REQUEST")
    
    if echo "$GITHUB_RESPONSE" | grep -q '"content"' || echo "$GITHUB_RESPONSE" | grep -q '"result"'; then
        echo -e "${GREEN}✓ GitHub tool executed successfully${NC}"
        echo -e "${BLUE}Response preview:${NC}"
        echo "$GITHUB_RESPONSE" | python3 -m json.tool 2>/dev/null | head -20 || echo "$GITHUB_RESPONSE" | head -100
    elif echo "$GITHUB_RESPONSE" | grep -q '"error"'; then
        echo -e "${YELLOW}⚠ GitHub tool returned an error (might need valid credentials)${NC}"
        echo "$GITHUB_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$GITHUB_RESPONSE"
    else
        echo -e "${RED}✗ Unexpected response from GitHub tool${NC}"
        echo "$GITHUB_RESPONSE"
    fi
fi

echo
echo -e "${BLUE}=== Summary ===${NC}"

# Summary
if echo "$TOOLS_RESPONSE" | grep -q '"GitHub"' || echo "$TOOLS_RESPONSE" | grep -q '"github"'; then
    echo -e "${GREEN}✅ Dynamic tools integration is working!${NC}"
    echo -e "${GREEN}✅ GitHub tool is properly exposed via MCP protocol${NC}"
else
    echo -e "${RED}❌ GitHub tool is not exposed via MCP${NC}"
    echo -e "${YELLOW}Troubleshooting steps:${NC}"
    echo "  1. Check if REST API is running: curl http://localhost:8081/health"
    echo "  2. Check if GitHub tool is in database: SELECT * FROM mcp.tool_configurations WHERE tool_name='github';"
    echo "  3. Check MCP server logs: docker-compose logs -f mcp-server | grep -i github"
    echo "  4. Verify REST API client in MCP server is configured correctly"
fi
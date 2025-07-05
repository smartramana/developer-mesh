#!/bin/bash

# Load .env file
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

echo "🔍 E2E Test Connection Validation"
echo "================================="
echo "MCP Base URL: $MCP_BASE_URL"
echo "API Base URL: $API_BASE_URL"
echo "API Key: ${E2E_API_KEY:0:8}...${E2E_API_KEY: -8} (length: ${#E2E_API_KEY})"
echo ""

# Test API health
echo "Testing API Health Endpoint..."
api_response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $E2E_API_KEY" \
    "$API_BASE_URL/health")
if [ "$api_response" = "200" ]; then
    echo "✅ API is healthy (HTTP $api_response)"
else
    echo "❌ API health check failed (HTTP $api_response)"
fi

# Test MCP health
echo ""
echo "Testing MCP Health Endpoint..."
mcp_response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $E2E_API_KEY" \
    "$MCP_BASE_URL/health")
if [ "$mcp_response" = "200" ]; then
    echo "✅ MCP is healthy (HTTP $mcp_response)"
else
    echo "❌ MCP health check failed (HTTP $mcp_response)"
fi

# Test WebSocket endpoint
echo ""
echo "Testing WebSocket Endpoint..."
ws_key=$(openssl rand -base64 16)
ws_response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $E2E_API_KEY" \
    -H "Connection: Upgrade" \
    -H "Upgrade: websocket" \
    -H "Sec-WebSocket-Version: 13" \
    -H "Sec-WebSocket-Key: $ws_key" \
    "$MCP_BASE_URL/ws")
if [ "$ws_response" = "101" ] || [ "$ws_response" = "401" ] || [ "$ws_response" = "426" ]; then
    echo "✅ WebSocket endpoint is accessible (HTTP $ws_response)"
else
    echo "❌ WebSocket endpoint check failed (HTTP $ws_response)"
fi

echo ""
echo "================================="
echo "Ready to run E2E tests with: make test"
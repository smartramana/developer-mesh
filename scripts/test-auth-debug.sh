#!/bin/bash

# Test auth with debug output

API_KEY="${E2E_API_KEY:-test-api-key-cacacb6b379e}"
MCP_URL="${MCP_BASE_URL:-mcp.dev-mesh.io}"

echo "Testing auth against $MCP_URL with API key: ${API_KEY:0:20}..."

# Test WebSocket auth
echo -e "\n1. Testing WebSocket endpoint:"
curl -v -H "Authorization: Bearer $API_KEY" \
     -H "Connection: Upgrade" \
     -H "Upgrade: websocket" \
     -H "Sec-WebSocket-Version: 13" \
     -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
     -H "Sec-WebSocket-Protocol: mcp.v1" \
     "https://$MCP_URL/ws" 2>&1 | grep -E "(< HTTP|401|200|101)"

echo -e "\n2. Testing REST API health:"
curl -s -H "Authorization: Bearer $API_KEY" "https://api.dev-mesh.io/api/v1/" | jq -r '.status' || echo "Failed"

echo -e "\n3. Testing without auth (should fail):"
curl -s "https://$MCP_URL/ws" -w "\nHTTP Status: %{http_code}\n"
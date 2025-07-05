#!/bin/bash

# Load environment
source .env

echo "=== WebSocket Authentication Debug ==="
echo "MCP Base URL: $MCP_BASE_URL"
echo "API Key: ${E2E_API_KEY:0:8}...${E2E_API_KEY: -8}"
echo ""

# Test 1: Health endpoint (no auth required)
echo "1. Testing health endpoint (no auth):"
curl -s "$MCP_BASE_URL/health" | jq -r '.status' || echo "Failed"

# Test 2: Health endpoint with auth
echo -e "\n2. Testing health endpoint with auth:"
curl -s -H "Authorization: Bearer $E2E_API_KEY" "$MCP_BASE_URL/health" | jq -r '.status' || echo "Failed"

# Test 3: WebSocket endpoint with auth
echo -e "\n3. Testing WebSocket endpoint:"
ws_key=$(openssl rand -base64 16)
response=$(curl -s -i -H "Authorization: Bearer $E2E_API_KEY" \
    -H "Connection: Upgrade" \
    -H "Upgrade: websocket" \
    -H "Sec-WebSocket-Version: 13" \
    -H "Sec-WebSocket-Key: $ws_key" \
    "$MCP_BASE_URL/ws")

# Extract status code
status_code=$(echo "$response" | head -n 1 | cut -d' ' -f2)
echo "HTTP Status: $status_code"

# Show response headers if not 101
if [ "$status_code" != "101" ]; then
    echo -e "\nResponse headers:"
    echo "$response" | head -n 20
fi

# Test 4: Try with wscat if available
if command -v wscat &> /dev/null; then
    echo -e "\n4. Testing with wscat:"
    timeout 5 wscat -H "Authorization: Bearer $E2E_API_KEY" -c "$MCP_BASE_URL/ws" 2>&1 | head -5 || echo "Connection failed"
fi

echo -e "\n=== End Debug ==="
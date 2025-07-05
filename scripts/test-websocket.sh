#!/bin/bash

echo "Testing WebSocket endpoints on mcp.dev-mesh.io"
echo "=============================================="

# Test with curl
echo -e "\n1. Testing with curl (expecting 101 Switching Protocols or 400/401):"
for endpoint in "/ws" "/v1/ws"; do
    echo -n "  Testing wss://mcp.dev-mesh.io${endpoint}... "
    # Generate a random WebSocket key (16 bytes base64 encoded)
    ws_key=$(openssl rand -base64 16)
    response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Connection: Upgrade" \
        -H "Upgrade: websocket" \
        -H "Sec-WebSocket-Version: 13" \
        -H "Sec-WebSocket-Key: $ws_key" \
        "https://mcp.dev-mesh.io${endpoint}")
    echo "HTTP $response"
done

# Test with wscat if available
if command -v wscat &> /dev/null; then
    echo -e "\n2. Testing with wscat:"
    echo "  Attempting connection to wss://mcp.dev-mesh.io/v1/ws"
    timeout 5 wscat -c "wss://mcp.dev-mesh.io/v1/ws" 2>&1 | head -10
else
    echo -e "\n2. wscat not installed. Install with: npm install -g wscat"
fi

# Test nginx config on remote server
echo -e "\n3. Checking nginx configuration on server:"
echo "  To check nginx config, SSH to the server and run:"
echo "  ssh ec2-user@54.86.185.227 'sudo grep -A 10 \"location.*ws\" /etc/nginx/conf.d/mcp.conf'"

# Test if MCP server is running
echo -e "\n4. Checking if MCP server is running:"
curl -s https://mcp.dev-mesh.io/health | jq '.' 2>/dev/null || echo "  Unable to parse health response"

echo -e "\n=============================================="
echo "Troubleshooting steps:"
echo "1. If getting 404, nginx may not be configured for WebSocket"
echo "2. If getting 502, the MCP server may not be running"
echo "3. If getting 401, authentication may be required"
echo "4. Run the nginx fix script on the server: ./fix-nginx-websocket.sh"
#!/bin/bash

# Script to apply nginx WebSocket configuration fix to remote EC2 instance
# This script can be run from your local machine

set -e

# Configuration
EC2_IP="${EC2_INSTANCE_IP:-54.86.185.227}"
SSH_KEY="${SSH_KEY_PATH:-~/.ssh/ec2-key.pem}"
NGINX_CONFIG_PATH="/Users/seancorkum/projects/developer-mesh/deployments/nginx/mcp.conf"

echo "======================================="
echo "Applying Nginx WebSocket Fix"
echo "======================================="
echo "Target: ec2-user@${EC2_IP}"
echo ""

# Check if SSH key exists
if [ ! -f "${SSH_KEY}" ]; then
    echo "Error: SSH key not found at ${SSH_KEY}"
    echo "Please set SSH_KEY_PATH environment variable or place key at ~/.ssh/ec2-key.pem"
    exit 1
fi

# Check if nginx config exists
if [ ! -f "${NGINX_CONFIG_PATH}" ]; then
    echo "Error: Nginx config not found at ${NGINX_CONFIG_PATH}"
    exit 1
fi

# Create backup on remote server
echo "1. Creating backup of current nginx configuration..."
ssh -i "${SSH_KEY}" "ec2-user@${EC2_IP}" "sudo cp /etc/nginx/conf.d/mcp.conf /etc/nginx/conf.d/mcp.conf.backup.\$(date +%Y%m%d_%H%M%S) 2>/dev/null || true"

# Copy new configuration
echo "2. Copying new nginx configuration..."
scp -i "${SSH_KEY}" "${NGINX_CONFIG_PATH}" "ec2-user@${EC2_IP}:/tmp/mcp.conf"

# Apply configuration and test
echo "3. Applying configuration..."
ssh -i "${SSH_KEY}" "ec2-user@${EC2_IP}" << 'EOF'
    set -e
    
    # Move config to proper location
    sudo mv /tmp/mcp.conf /etc/nginx/conf.d/mcp.conf
    
    # Test nginx configuration
    echo "Testing nginx configuration..."
    if sudo nginx -t; then
        echo "✓ Configuration is valid"
        
        # Reload nginx
        echo "Reloading nginx..."
        sudo systemctl reload nginx
        echo "✓ Nginx reloaded successfully"
    else
        echo "✗ Configuration test failed! Restoring backup..."
        sudo cp /etc/nginx/conf.d/mcp.conf.backup.$(ls -t /etc/nginx/conf.d/mcp.conf.backup.* | head -1 | cut -d. -f4) /etc/nginx/conf.d/mcp.conf
        exit 1
    fi
EOF

# Test WebSocket endpoint
echo ""
echo "4. Testing WebSocket endpoint..."
response=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Connection: Upgrade" \
    -H "Upgrade: websocket" \
    -H "Sec-WebSocket-Version: 13" \
    -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
    "https://mcp.dev-mesh.io/ws")

if [ "$response" = "101" ] || [ "$response" = "400" ] || [ "$response" = "401" ]; then
    echo "✓ WebSocket endpoint accessible (HTTP $response)"
else
    echo "✗ WebSocket endpoint returned HTTP $response"
fi

echo ""
echo "======================================="
echo "Nginx WebSocket fix applied!"
echo "======================================="
echo ""
echo "You can now run the E2E tests:"
echo "cd test/e2e && MCP_BASE_URL=mcp.dev-mesh.io API_BASE_URL=api.dev-mesh.io E2E_API_KEY=your-key make test"
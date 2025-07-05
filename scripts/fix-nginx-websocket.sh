#!/bin/bash

# Script to fix nginx configuration for WebSocket support
# Run this on the EC2 instance as ec2-user with sudo permissions

set -e

echo "======================================="
echo "Nginx WebSocket Configuration Fix"
echo "======================================="

# Check if running with proper permissions
if [[ $EUID -eq 0 ]]; then
   echo "Please run this script as ec2-user, not root"
   exit 1
fi

# Backup current configuration
echo "Creating backup of current nginx configuration..."
sudo cp /etc/nginx/conf.d/mcp.conf /etc/nginx/conf.d/mcp.conf.backup.$(date +%Y%m%d_%H%M%S)

# Show current configuration
echo -e "\nCurrent nginx configuration:"
echo "-------------------------------"
sudo cat /etc/nginx/conf.d/mcp.conf

# Create updated configuration with WebSocket support
echo -e "\nCreating updated configuration with WebSocket support..."
sudo tee /etc/nginx/conf.d/mcp.conf > /dev/null << 'EOF'
# MCP Server Configuration
server {
    listen 80;
    server_name mcp.dev-mesh.io;
    
    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl;
    server_name mcp.dev-mesh.io;
    
    # SSL configuration (managed by certbot)
    ssl_certificate /etc/letsencrypt/live/mcp.dev-mesh.io/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/mcp.dev-mesh.io/privkey.pem;
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    # Main location block for HTTP requests
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    # WebSocket endpoints
    location /ws {
        proxy_pass http://localhost:8080;
        
        # WebSocket specific headers
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Standard proxy headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket timeouts (longer than HTTP)
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
        
        # Disable buffering for WebSocket
        proxy_buffering off;
    }
}

# API Server Configuration
server {
    listen 80;
    server_name api.dev-mesh.io;
    
    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl;
    server_name api.dev-mesh.io;
    
    # SSL configuration (managed by certbot)
    ssl_certificate /etc/letsencrypt/live/api.dev-mesh.io/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.dev-mesh.io/privkey.pem;
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    location / {
        proxy_pass http://localhost:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
EOF

# Test nginx configuration
echo -e "\nTesting nginx configuration..."
if sudo nginx -t; then
    echo "Configuration is valid!"
else
    echo "Configuration test failed! Restoring backup..."
    sudo cp /etc/nginx/conf.d/mcp.conf.backup.$(date +%Y%m%d_%H%M%S) /etc/nginx/conf.d/mcp.conf
    exit 1
fi

# Reload nginx
echo -e "\nReloading nginx..."
sudo systemctl reload nginx

# Test WebSocket endpoint
echo -e "\nTesting WebSocket endpoints..."
for endpoint in "/ws"; do
    echo -n "Testing wss://mcp.dev-mesh.io${endpoint}... "
    # Generate a random WebSocket key (16 bytes base64 encoded)
    ws_key=$(openssl rand -base64 16)
    response=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Connection: Upgrade" \
        -H "Upgrade: websocket" \
        -H "Sec-WebSocket-Version: 13" \
        -H "Sec-WebSocket-Key: $ws_key" \
        "https://mcp.dev-mesh.io${endpoint}")
    
    if [ "$response" = "101" ] || [ "$response" = "400" ] || [ "$response" = "401" ]; then
        echo "✓ Endpoint accessible (HTTP $response)"
    else
        echo "✗ Endpoint not working (HTTP $response)"
    fi
done

echo -e "\n======================================="
echo "Nginx WebSocket configuration updated!"
echo "======================================="
echo ""
echo "Next steps:"
echo "1. Verify WebSocket connections work with: wscat -c wss://mcp.dev-mesh.io/ws"
echo "2. Run E2E tests: cd /path/to/devops-mcp/test/e2e && make test"
echo "3. If issues persist, check application logs: docker logs devops-mcp_mcp-server_1"
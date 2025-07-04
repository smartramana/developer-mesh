#!/bin/bash
# Copy and paste these commands into your SSH session on the EC2 instance

# 1. Backup current nginx config
sudo cp /etc/nginx/conf.d/mcp.conf /etc/nginx/conf.d/mcp.conf.backup.$(date +%Y%m%d_%H%M%S)

# 2. Create new nginx configuration with WebSocket support
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
    
    # WebSocket endpoint
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

# 3. Test nginx configuration
sudo nginx -t

# 4. If test passes, reload nginx
sudo systemctl reload nginx

# 5. Test WebSocket endpoint
echo "Testing WebSocket endpoint..."
curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" \
    -H "Connection: Upgrade" \
    -H "Upgrade: websocket" \
    -H "Sec-WebSocket-Version: 13" \
    -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
    "https://mcp.dev-mesh.io/ws"
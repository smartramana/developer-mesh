# Nginx WebSocket Configuration Fix

## Quick Fix (Apply Immediately)

Run this command to apply the nginx fix to your EC2 instance:

```bash
# Option 1: If you have the SSH key in a standard location
./scripts/apply-nginx-fix.sh

# Option 2: If your SSH key is in a different location
SSH_KEY_PATH=/path/to/your/key.pem ./scripts/apply-nginx-fix.sh

# Option 3: Manual steps
# 1. Copy the nginx config to the server
scp -i /path/to/your/key.pem deployments/nginx/mcp.conf ec2-user@54.86.185.227:/tmp/

# 2. SSH to the server
ssh -i /path/to/your/key.pem ec2-user@54.86.185.227

# 3. Apply the configuration
sudo cp /etc/nginx/conf.d/mcp.conf /etc/nginx/conf.d/mcp.conf.backup
sudo mv /tmp/mcp.conf /etc/nginx/conf.d/mcp.conf
sudo nginx -t && sudo systemctl reload nginx
```

## What This Fixes

1. **WebSocket Support**: Adds proper WebSocket upgrade headers for the `/ws` endpoint
2. **Connection Persistence**: Sets appropriate timeouts for long-lived WebSocket connections
3. **Buffering**: Disables proxy buffering for WebSocket connections

## Changes Made

### 1. Created Nginx Configuration
- File: `deployments/nginx/mcp.conf`
- Includes WebSocket support for `/ws` endpoint
- Proper SSL configuration for both domains

### 2. Updated Deployment Script
- File: `.github/workflows/deploy-production-v2.yml`
- Now copies nginx configuration during deployment
- Tests WebSocket endpoint after deployment

### 3. Created Helper Scripts
- `scripts/apply-nginx-fix.sh` - Applies nginx fix remotely
- `scripts/test-websocket.sh` - Tests WebSocket connectivity

## Testing

After applying the fix, test WebSocket connectivity:

```bash
# Test with curl
curl -v https://mcp.dev-mesh.io/ws \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ=="

# Expected response: HTTP 101, 400, or 401 (not 404)
```

## Run E2E Tests

Once nginx is fixed, run the E2E tests:

```bash
cd test/e2e
MCP_BASE_URL=mcp.dev-mesh.io \
API_BASE_URL=api.dev-mesh.io \
E2E_API_KEY=your-api-key \
make test
```

## Troubleshooting

If WebSocket still returns 404:
1. Check if nginx reloaded: `sudo systemctl status nginx`
2. Check nginx error logs: `sudo tail -f /var/log/nginx/error.log`
3. Verify MCP server is running: `docker ps | grep mcp-server`
4. Check MCP server logs: `docker logs devops-mcp_mcp-server_1`
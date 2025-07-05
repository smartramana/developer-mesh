# E2E Test Troubleshooting Guide

## Issue: E2E Tests Failing with 401 Unauthorized

### Summary
E2E tests were failing with WebSocket authentication errors (401 Unauthorized) when connecting to production, while REST endpoints worked fine with the same API key.

### Root Causes Identified

1. **Nginx Configuration Issues**:
   - Authorization headers were not being explicitly forwarded to backend services
   - HTTP version wasn't set to 1.1 for all proxy locations (causing "HTTP/1.0" errors)

2. **Key Findings**:
   - REST endpoints (`/health`) work fine with the API key
   - WebSocket endpoint (`/ws`) was returning 401 Unauthorized
   - The ADMIN_API_KEY on the server matches the E2E_API_KEY in tests
   - The MCP server has proper authentication code that validates API keys

### Solution Applied

Updated nginx configuration (`deployments/nginx/mcp.conf`):

```nginx
# For all location blocks, added:
proxy_http_version 1.1;
proxy_set_header Authorization $http_authorization;
```

This ensures:
- Authorization headers are properly forwarded from client to backend
- HTTP/1.1 is used (required for WebSocket upgrades)
- Both REST and WebSocket endpoints receive auth headers

### Testing E2E Connectivity

1. **Manual validation**:
   ```bash
   cd test/e2e
   source .env
   
   # Test REST endpoint
   curl -H "Authorization: Bearer $E2E_API_KEY" "$MCP_BASE_URL/health"
   
   # Test WebSocket endpoint
   ./debug_ws_auth.sh
   ```

2. **Run E2E tests**:
   ```bash
   make test-single  # Run single agent tests
   make test         # Run full E2E suite
   ```

### Environment Variables

The E2E tests require these environment variables in `test/e2e/.env`:
- `MCP_BASE_URL`: https://mcp.dev-mesh.io
- `API_BASE_URL`: https://api.dev-mesh.io
- `E2E_API_KEY`: The API key that matches ADMIN_API_KEY on the server

### Deployment Notes

- The deployment workflow uses `:latest` tags by default
- Recent deployment broke due to using old database password
- Ensure IMAGE_TAG is properly set in deployment to use specific SHA tags
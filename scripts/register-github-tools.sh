#!/bin/bash

# Register GitHub tools in DevMesh database
# This makes Edge MCP tools available through the DevMesh MCP server

set -e

# Configuration
API_URL="http://localhost:8081"
TENANT_ID="00000000-0000-0000-0000-000000000001"
API_KEY="dev-admin-key-1234567890"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Registering GitHub tools in DevMesh...${NC}"

# Register GitHub tool
echo -e "${YELLOW}Creating GitHub tool configuration...${NC}"

curl -X POST "$API_URL/api/v1/tools" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "name": "github_repos",
    "base_url": "https://api.github.com",
    "documentation_url": "https://docs.github.com/rest",
    "openapi_url": "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json",
    "provider": "github",
    "config": {
      "description": "GitHub repository operations",
      "version": "v1",
      "tool_type": "github",
      "display_name": "GitHub Repositories",
      "auth_type": "bearer",
      "endpoints": {
        "get_repo": {
          "path": "/repos/{owner}/{repo}",
          "method": "GET",
          "description": "Get repository information"
        },
        "create_repo": {
          "path": "/user/repos",
          "method": "POST",
          "description": "Create a new repository"
        },
        "list_repos": {
          "path": "/user/repos",
          "method": "GET",
          "description": "List user repositories"
        }
      }
    },
    "credential": {
      "type": "bearer",
      "token": "${GITHUB_TOKEN:-your-github-token}"
    }
  }'

echo -e "\n${GREEN}✓ GitHub tool registered${NC}"

# Register more GitHub tools
echo -e "${YELLOW}Registering additional GitHub tools...${NC}"

# GitHub Issues
curl -X POST "$API_URL/api/v1/tools" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "name": "github_issues",
    "base_url": "https://api.github.com",
    "provider": "github",
    "config": {
      "description": "GitHub issue management",
      "tool_type": "github",
      "display_name": "GitHub Issues",
      "endpoints": {
        "list_issues": {
          "path": "/repos/{owner}/{repo}/issues",
          "method": "GET"
        },
        "create_issue": {
          "path": "/repos/{owner}/{repo}/issues",
          "method": "POST"
        }
      }
    }
  }'

# GitHub Pull Requests
curl -X POST "$API_URL/api/v1/tools" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "name": "github_pulls",
    "base_url": "https://api.github.com",
    "provider": "github",
    "config": {
      "description": "GitHub pull request management",
      "tool_type": "github",
      "display_name": "GitHub Pull Requests",
      "endpoints": {
        "list_pulls": {
          "path": "/repos/{owner}/{repo}/pulls",
          "method": "GET"
        },
        "create_pull": {
          "path": "/repos/{owner}/{repo}/pulls",
          "method": "POST"
        }
      }
    }
  }'

echo -e "\n${YELLOW}Verifying registered tools...${NC}"

# List all tools to verify
TOOLS=$(curl -s "$API_URL/api/v1/tools" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $API_KEY" | jq -r '.count')

echo -e "${GREEN}✓ Successfully registered tools. Total tools: $TOOLS${NC}"

# Now test if they appear in MCP
echo -e "\n${YELLOW}Testing MCP tool exposure...${NC}"

# Create a simple test script
cat > /tmp/test-mcp-tools.sh << 'EOF'
#!/bin/bash
WS_URL="ws://localhost:8080/ws"
AUTH="Bearer dev-admin-key-1234567890"

# Send initialization and list tools
(
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1.0.0"}}}'
sleep 0.5
echo '{"jsonrpc":"2.0","id":2,"method":"initialized","params":{}}'
sleep 0.5
echo '{"jsonrpc":"2.0","id":3,"method":"tools/list"}'
sleep 1
) | websocat --header="Authorization: $AUTH" "$WS_URL" 2>/dev/null | \
  grep -o '"name":"[^"]*"' | cut -d'"' -f4 | sort -u
EOF

chmod +x /tmp/test-mcp-tools.sh
echo "Available MCP tools:"
/tmp/test-mcp-tools.sh

echo -e "\n${GREEN}✓ Tool registration complete!${NC}"
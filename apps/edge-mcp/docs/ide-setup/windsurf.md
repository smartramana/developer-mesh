# Windsurf Configuration

## Prerequisites

1. DevMesh account with API key and tenant ID
2. Edge MCP installed and in PATH

## Configuration

Create `.windsurf/mcp-config.json` in your project root:

```json
{
  "version": "1.0.0",
  "servers": {
    "devmesh": {
      "type": "local",
      "executable": "edge-mcp",
      "arguments": ["--port=8082"],
      "environment": {
        "CORE_PLATFORM_URL": "https://api.devmesh.ai",
        "CORE_PLATFORM_API_KEY": "your-api-key",
        "TENANT_ID": "your-tenant-id"
      }
    }
  },
  "activeServer": "devmesh"
}
```

Replace `your-api-key` and `your-tenant-id` with values from your DevMesh dashboard.

## Verification

After restarting Windsurf:

1. Check the MCP panel shows "Connected"
2. Tools should be listed automatically (no manual configuration needed)
3. Test with a simple command

## Available Tools

Tools are dynamically discovered from your DevMesh tenant. Common tools include:

- **GitHub**: Complete API access for all GitHub operations
- **AWS**: Full AWS service integration (S3, Lambda, EC2, etc.)
- **Slack**: Messaging, channels, and workspace management
- **Jira**: Issues, projects, and sprint management
- Plus any custom tools configured in your tenant

## Features

Windsurf provides excellent MCP integration:

- **Auto-start**: Automatically starts Edge MCP when you open a project
- **Tool Discovery**: Automatically discovers available tools via MCP protocol
- **Status Indicators**: Shows connection status in the UI
- **Error Reporting**: Clear error messages for troubleshooting

## Alternative: Environment Variables

If you prefer not to hardcode credentials, use environment variables:

```json
{
  "version": "1.0.0",
  "servers": {
    "devmesh": {
      "type": "local",
      "executable": "edge-mcp",
      "arguments": ["--port=8082"],
      "environment": {
        "CORE_PLATFORM_URL": "${env:CORE_PLATFORM_URL}",
        "CORE_PLATFORM_API_KEY": "${env:CORE_PLATFORM_API_KEY}",
        "TENANT_ID": "${env:TENANT_ID}"
      }
    }
  },
  "activeServer": "devmesh"
}
```

Then set environment variables before starting Windsurf:

```bash
export CORE_PLATFORM_URL="https://api.devmesh.ai"
export CORE_PLATFORM_API_KEY="your-api-key"
export TENANT_ID="your-tenant-id"
```

## Troubleshooting

### Edge MCP not found
- Ensure Edge MCP is in PATH: `which edge-mcp`
- Or use full path in configuration: `"executable": "/usr/local/bin/edge-mcp"`

### Authentication failed
- Verify API key and tenant ID from DevMesh dashboard
- Check Core Platform URL is correct
- Look at Windsurf's output panel for detailed errors

### Tools not appearing
- Check your DevMesh tenant configuration
- Verify service credentials are added in DevMesh dashboard
- Restart Windsurf after configuration changes
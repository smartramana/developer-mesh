#!/bin/bash

# Edge MCP Tool Executor
# Workaround for Claude Code's inability to execute MCP tools directly
# Usage: ./edge-mcp-tool.sh <tool_name> '<json_arguments>'

set -e

TOOL_NAME="$1"
TOOL_ARGS="$2"

if [ -z "$TOOL_NAME" ]; then
    echo "Usage: $0 <tool_name> '<json_arguments>'"
    echo "Example: $0 github_repos '{\"action\":\"create\",\"owner\":\"user\",\"repo\":\"test\"}'"
    exit 1
fi

# Default to empty JSON if no arguments provided
if [ -z "$TOOL_ARGS" ]; then
    TOOL_ARGS="{}"
fi

# Use the REST API to execute the tool
API_URL="http://localhost:8081"
API_KEY="devmesh_0e07b54644e4c22e7bcec181f0f45de9ec48b02a537eaa15314204c0687c2c5b"
TENANT_ID="00000000-0000-0000-0000-000000000001"

# Execute the tool via REST API
curl -s -X POST "$API_URL/api/v1/tools/execute" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d "{
    \"tool_name\": \"$TOOL_NAME\",
    \"arguments\": $TOOL_ARGS
  }" | jq '.'
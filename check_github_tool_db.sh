#!/bin/bash

# Check GitHub tool in database

echo "========================================"
echo "Checking GitHub Tool in Database"
echo "========================================"
echo ""

# Connect to database and check tool configuration
docker exec -i devops-mcp-postgres-1 psql -U devmesh -d devmesh_development <<EOF
-- Check tool configuration
SELECT 
    id,
    tool_name,
    status,
    health_status,
    (config->>'openapi_url') as openapi_url,
    (config->>'base_url') as base_url,
    jsonb_array_length(COALESCE(config->'webhook_events', '[]'::jsonb)) as webhook_event_count,
    created_at
FROM mcp.tool_configurations 
WHERE id = '6e2d0ff9-09f9-40cc-8a49-ba5267a4aba1';

-- Check if discovery result is stored
SELECT 
    COUNT(*) as discovery_sessions
FROM mcp.tool_discovery_sessions
WHERE base_url LIKE '%github%';

-- Check webhook configuration
SELECT 
    id,
    tool_id,
    endpoint_path,
    jsonb_array_length(COALESCE(events, '[]'::jsonb)) as event_count,
    created_at
FROM mcp.webhook_configurations
WHERE tool_id = '6e2d0ff9-09f9-40cc-8a49-ba5267a4aba1'
LIMIT 1;
EOF

echo ""
echo "========================================"
echo "Checking Tool Registry Cache"
echo "========================================"

# Check if the MCP server has the tool in cache
curl -s http://localhost:8080/health | jq '.services.tool_registry' 2>/dev/null || echo "MCP Server health check"

# Try to get tool actions from MCP server directly
echo ""
echo "Getting tool capabilities from MCP server..."
curl -s -X GET "http://localhost:8080/api/v1/tools/6e2d0ff9-09f9-40cc-8a49-ba5267a4aba1/capabilities" \
  -H "X-Tenant-ID: default" 2>/dev/null | jq '.' || echo "Endpoint not available"
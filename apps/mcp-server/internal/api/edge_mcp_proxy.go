package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// EdgeMCPProxy proxies tool calls to the Edge MCP server
type EdgeMCPProxy struct {
	edgeMCPPath string // Path to edge-mcp executable
	logger      observability.Logger
}

// NewEdgeMCPProxy creates a new Edge MCP proxy
func NewEdgeMCPProxy(logger observability.Logger) *EdgeMCPProxy {
	return &EdgeMCPProxy{
		edgeMCPPath: "edge-mcp", // Assumes edge-mcp is in PATH
		logger:      logger,
	}
}

// GetEdgeTools returns the list of tools from Edge MCP
func (p *EdgeMCPProxy) GetEdgeTools(ctx context.Context) ([]map[string]interface{}, error) {
	// Execute edge-mcp to get tool list
	cmd := exec.CommandContext(ctx, p.edgeMCPPath, "tools", "list", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get edge tools: %w", err)
	}

	var tools []map[string]interface{}
	if err := json.Unmarshal(output, &tools); err != nil {
		return nil, fmt.Errorf("failed to parse edge tools: %w", err)
	}

	// Transform tool names to avoid conflicts
	for _, tool := range tools {
		if name, ok := tool["name"].(string); ok {
			// Prefix with edge_ to distinguish from DevMesh tools
			tool["name"] = "edge_" + name
			tool["description"] = fmt.Sprintf("[Edge MCP] %v", tool["description"])
		}
	}

	return tools, nil
}

// ExecuteEdgeTool executes a tool via Edge MCP
func (p *EdgeMCPProxy) ExecuteEdgeTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	// Remove edge_ prefix if present
	if len(toolName) > 5 && toolName[:5] == "edge_" {
		toolName = toolName[5:]
	}

	// Marshal arguments
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal arguments: %w", err)
	}

	// Execute tool via edge-mcp
	cmd := exec.CommandContext(ctx, p.edgeMCPPath, "tools", "execute", toolName, "--args", string(argsJSON), "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute edge tool %s: %w", toolName, err)
	}

	var result interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse edge tool result: %w", err)
	}

	return result, nil
}

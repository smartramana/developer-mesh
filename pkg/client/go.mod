module github.com/S-Corkum/devops-mcp/pkg/client

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp/pkg/embedding v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/observability v0.0.0
)

replace (
	github.com/S-Corkum/devops-mcp/pkg/embedding => ../embedding
	github.com/S-Corkum/devops-mcp/pkg/observability => ../observability
)

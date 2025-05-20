module github.com/S-Corkum/devops-mcp/pkg/events

go 1.20

require (
	github.com/S-Corkum/devops-mcp/pkg/mcp v0.0.0
	github.com/google/uuid v1.3.0
)

replace github.com/S-Corkum/devops-mcp/pkg/mcp => ../mcp

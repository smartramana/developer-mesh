module github.com/S-Corkum/devops-mcp/pkg/adapters

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp/pkg/events v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/events/system v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/mcp v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/observability v0.0.0
	github.com/google/uuid v1.3.0
	github.com/stretchr/testify v1.8.4
)

replace (
	github.com/S-Corkum/devops-mcp/pkg/events => ../events
	github.com/S-Corkum/devops-mcp/pkg/events/system => ../events/system
	github.com/S-Corkum/devops-mcp/pkg/mcp => ../mcp
	github.com/S-Corkum/devops-mcp/pkg/observability => ../observability
)

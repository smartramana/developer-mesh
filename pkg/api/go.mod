module github.com/S-Corkum/devops-mcp/pkg/api

go 1.20

require (
	// Core packages
	github.com/S-Corkum/devops-mcp/pkg/models v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/repository v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/repository/vector v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/observability v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/util v0.0.0

	// Additional dependencies
	github.com/S-Corkum/devops-mcp/pkg/core v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/database v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/config v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/embedding v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/interfaces v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/queue v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/common v0.0.0

	// External dependencies
	github.com/gin-gonic/gin v1.9.1
	github.com/stretchr/testify v1.8.3
)

replace (
	// Core module dependencies
	github.com/S-Corkum/devops-mcp/pkg/models => ../models
	github.com/S-Corkum/devops-mcp/pkg/repository => ../repository
	github.com/S-Corkum/devops-mcp/pkg/repository/vector => ../repository/vector
	github.com/S-Corkum/devops-mcp/pkg/observability => ../observability
	github.com/S-Corkum/devops-mcp/pkg/util => ../util

	// Required additional dependencies
	github.com/S-Corkum/devops-mcp/pkg/core => ../core
	github.com/S-Corkum/devops-mcp/pkg/database => ../database
	github.com/S-Corkum/devops-mcp/pkg/config => ../config
	github.com/S-Corkum/devops-mcp/pkg/embedding => ../embedding
	github.com/S-Corkum/devops-mcp/pkg/interfaces => ../interfaces
	github.com/S-Corkum/devops-mcp/pkg/queue => ../queue
	github.com/S-Corkum/devops-mcp/pkg/common => ../common
)

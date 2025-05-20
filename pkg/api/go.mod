module github.com/S-Corkum/devops-mcp/pkg/api

go 1.20

require (
	github.com/S-Corkum/devops-mcp/pkg/models v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/repository v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/util v0.0.0
	github.com/gin-gonic/gin v1.9.1
)

replace (
	github.com/S-Corkum/devops-mcp/pkg/models => ../models
	github.com/S-Corkum/devops-mcp/pkg/repository => ../repository
	github.com/S-Corkum/devops-mcp/pkg/util => ../util
)

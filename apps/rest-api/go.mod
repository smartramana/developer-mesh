module github.com/S-Corkum/devops-mcp/apps/rest-api

go 1.24

require (
	github.com/S-Corkum/devops-mcp/pkg/common v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/models v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/database v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/embedding v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/storage v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/migrations v0.0.0

	// External dependencies
	github.com/aws/aws-sdk-go v1.50.0
	github.com/gin-gonic/gin v1.9.1
	github.com/google/uuid v1.4.0
	github.com/lib/pq v1.10.9
	github.com/stretchr/testify v1.8.4
	github.com/swaggo/files v1.0.1
	github.com/swaggo/gin-swagger v1.6.0
	golang.org/x/time v0.5.0
	google.golang.org/grpc v1.54.0
)

// Replace directives for local development
replace (
	github.com/S-Corkum/devops-mcp/pkg/common => ../../pkg/common
	github.com/S-Corkum/devops-mcp/pkg/models => ../../pkg/models
	github.com/S-Corkum/devops-mcp/pkg/database => ../../pkg/database
	github.com/S-Corkum/devops-mcp/pkg/embedding => ../../pkg/embedding
	github.com/S-Corkum/devops-mcp/pkg/storage => ../../pkg/storage
	github.com/S-Corkum/devops-mcp/pkg/migrations => ../../pkg/migrations
)

// REST API service for vector search, embedding, and other functionality

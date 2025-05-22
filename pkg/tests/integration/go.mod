module github.com/S-Corkum/devops-mcp/pkg/tests/integration

go 1.20

require (
	github.com/S-Corkum/devops-mcp/pkg/aws v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/chunking v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/database v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/embedding v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/events v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/mcp v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/models v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/observability v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/repository v0.0.0
	github.com/S-Corkum/devops-mcp/pkg/repository/vector v0.0.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.40.0
	github.com/stretchr/testify v1.8.4
)

replace (
	github.com/S-Corkum/devops-mcp/pkg/aws => ../../aws
	github.com/S-Corkum/devops-mcp/pkg/chunking => ../../chunking
	github.com/S-Corkum/devops-mcp/pkg/database => ../../database
	github.com/S-Corkum/devops-mcp/pkg/embedding => ../../embedding
	github.com/S-Corkum/devops-mcp/pkg/events => ../../events
	github.com/S-Corkum/devops-mcp/pkg/mcp => ../../mcp
	github.com/S-Corkum/devops-mcp/pkg/models => ../../models
	github.com/S-Corkum/devops-mcp/pkg/observability => ../../observability
	github.com/S-Corkum/devops-mcp/pkg/repository => ../../repository
	github.com/S-Corkum/devops-mcp/pkg/repository/vector => ../../repository/vector
)

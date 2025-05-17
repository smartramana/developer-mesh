module github.com/S-Corkum/devops-mcp/apps/worker

go 1.24

require (
	github.com/aws/aws-sdk-go-v2 v1.20.0
	github.com/aws/aws-sdk-go-v2/config v1.18.29
	github.com/aws/aws-sdk-go-v2/service/sqs v1.24.0
	github.com/go-redis/redis/v8 v8.11.5
)

// Replace directives for local development
replace (
	github.com/S-Corkum/devops-mcp/pkg/chunking => ../../pkg/chunking
	github.com/S-Corkum/devops-mcp/pkg/common => ../../pkg/common
	github.com/S-Corkum/devops-mcp/pkg/embedding => ../../pkg/embedding
	github.com/S-Corkum/devops-mcp/pkg/models => ../../pkg/models
	github.com/S-Corkum/devops-mcp/pkg/storage => ../../pkg/storage
)

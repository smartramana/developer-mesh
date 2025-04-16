# MCP Server

MCP (Model Context Protocol) Server provides AI agents with both advanced context management capabilities and DevOps tool integrations. It serves two primary functions:

1. **Context Management**: A centralized platform for storing, retrieving, and manipulating conversation contexts, enabling agents to maintain coherent conversations and manage memory efficiently while directly interacting with model providers like Amazon Bedrock.

2. **DevOps Integration**: A unified API for AI agents to interact with popular DevOps tools like GitHub, Harness, SonarQube, and JFrog products, with automatic context tracking of all operations.

## Features

### Context Management
- **Context Storage**: Store, retrieve, and manipulate conversation contexts for AI agents
- **Context Windowing**: Intelligent context window management with truncation and relevance scoring
- **Memory Management**: Efficiently manage long-term and short-term agent memory
- **Event-Driven Architecture**: React to agent interactions and context changes in real-time
- **Context Search**: Search within contexts for relevant information
- **Vector Search**: Semantic search for contexts using vector embeddings and similarity matching
- **S3 Storage**: Efficiently store and retrieve large context data in Amazon S3 or S3-compatible services

### DevOps Integration
- **Unified API**: Interact with multiple DevOps tools through a consistent API
- **Contextual Operations**: All tool operations are automatically tracked in conversation context
- **Event Handling**: Process webhooks from DevOps tools and update relevant contexts
- **Tool Discovery**: Dynamically discover available tools and their capabilities

### Platform Capabilities
- **Extensible Design**: Easily add new tool integrations or context management strategies
- **Resilient Processing**: Built-in retry mechanisms, circuit breakers, and error handling
- **Performance Optimized**: Connection pooling, caching, and concurrency management
- **Comprehensive Authentication**: Secure API access and webhook verification
- **AWS Integration**: Seamless integration with AWS services using IAM Roles for Service Accounts (IRSA)

## How It Works with AI Agents

The MCP Server provides a context management system that AI agents can leverage when interacting with model providers like Amazon Bedrock. The typical workflow is:

1. **Agent Initialization**: The AI agent connects to both the MCP Server and Amazon Bedrock
2. **Context Creation**: The agent creates a context in the MCP Server to store conversation history
3. **Direct Inference**: The agent makes direct calls to Amazon Bedrock for model inference, providing the prompt/context
4. **Context Management**: The agent stores responses and manages conversation context through the MCP Server
5. **Context Retrieval**: When needed, the agent retrieves context from the MCP Server to maintain conversation continuity
6. **Tool Interaction**: The agent uses the MCP Server to interact with DevOps tools like GitHub while maintaining context

This architecture allows agents to leverage Amazon Bedrock's wide variety of models while maintaining sophisticated context management capabilities and interacting with real-world tools.

For a comprehensive list of everything an AI Agent can do with the MCP Server, see [AI Agent Capabilities](docs/agent-capabilities.md).

## Supported Model Providers via Amazon Bedrock

When used with Amazon Bedrock, the MCP Server can help agents work with models from:

1. **Anthropic**: Claude 3 Opus, Claude 3 Sonnet, Claude 3 Haiku, and others
2. **AI21**: Jurassic-2 models
3. **Cohere**: Command models
4. **Meta**: Llama 2 models
5. **Mistral**: Mistral models
6. **Amazon**: Titan models

## Getting Started

### Prerequisites

- Go 1.24 or higher
- Docker and Docker Compose (for local development)
- AWS credentials for Amazon Bedrock (if using Bedrock)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/S-Corkum/mcp-server.git
cd mcp-server
```

2. Copy the configuration template:

```bash
cp configs/config.yaml.template configs/config.yaml
```

3. Edit the configuration file with your credentials and settings.

4. Create an `.env` file with your environment variables:

```bash
# MCP Server configuration
MCP_API_LISTEN_ADDRESS=:8080
MCP_DATABASE_DSN=postgres://user:password@postgres:5432/mcp?sslmode=disable
MCP_AUTH_JWT_SECRET=your-jwt-secret
MCP_AUTH_API_KEYS_ADMIN=your-admin-api-key
MCP_AGENT_WEBHOOK_SECRET=your-webhook-secret

# AWS configuration (if using Bedrock)
AWS_REGION=us-east-1
AWS_PROFILE=default

# S3 configuration (if using S3 storage)
MCP_STORAGE_TYPE=s3
MCP_STORAGE_S3_REGION=us-west-2
MCP_STORAGE_S3_BUCKET=mcp-contexts
MCP_STORAGE_CONTEXT_STORAGE_PROVIDER=s3
```

### Running with Docker Compose

The easiest way to run the MCP Server locally is using Docker Compose:

```bash
docker-compose up -d
```

This will start the MCP Server along with its dependencies (PostgreSQL, Redis, Prometheus, and Grafana).

> **Note for Production Deployments**: The default configuration uses port 8080, which is suitable for development but not recommended for production. For production environments, you should configure HTTPS with TLS certificates on port 443. See the [Production Deployment Security Guide](docs/security/production-deployment-security.md) for details.

### Deploying to EKS

For production deployment on Amazon EKS, MCP Server includes Kubernetes manifests in the `kubernetes/` directory:

```bash
# Create the namespace
kubectl apply -f kubernetes/namespace.yaml

# Create the service account with IRSA annotations
kubectl apply -f kubernetes/serviceaccount.yaml

# Apply other resources
kubectl apply -f kubernetes/deployment.yaml
kubectl apply -f kubernetes/service.yaml
```

The deployment is configured to:
- Use IAM Roles for Service Accounts (IRSA) for secure AWS authentication
- Run on port 443 with TLS in production environments
- Set appropriate resource requests and limits
- Configure health checks and readiness probes
- Specify security contexts for least privilege

See the [AWS IRSA Setup Guide](docs/aws/aws-irsa-setup.md) for detailed configuration instructions for EKS deployments.

### Building and Running Locally

1. Install Go dependencies:

```bash
go mod download
```

2. Build the server:

```bash
go build -o mcp-server ./cmd/server
```

3. Update your `.env` file with real credentials.

4. Run the server:

```bash
./mcp-server
```

## Example Usage with AI Agent, Amazon Bedrock, and DevOps Tools

Here's an example of how an AI agent would use the MCP Server to manage context and interact with DevOps tools:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/client"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

func main() {
	// Initialize MCP client
	mcpClient := client.NewClient(
		"http://localhost:8080",
		client.WithAPIKey("your-api-key"),
		client.WithWebhookSecret("your-webhook-secret"),
	)
	
	// Initialize AWS Bedrock client
	cfg, err := config.LoadDefaultConfig(context.Background(), 
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	
	bedrockClient := bedrockruntime.NewFromConfig(cfg)
	
	// Create a new context for the conversation
	ctx := context.Background()
	conversationContext, err := mcpClient.CreateContext(ctx, &mcp.Context{
		AgentID:      "my-agent-1",
		ModelID:      "anthropic.claude-3-sonnet-20240229-v1:0", // Bedrock model ID
		SessionID:    "user-session-123",
		MaxTokens:    100000,
		Content:      []mcp.ContextItem{},
		Metadata: map[string]interface{}{
			"user_id": "user123",
			"source": "web",
		},
	})
	
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}
	
	log.Printf("Created context with ID: %s", conversationContext.ID)
	
	// Add system message to context
	systemMessage := mcp.ContextItem{
		Role:      "system",
		Content:   "You are a helpful DevOps assistant.",
		Timestamp: time.Now(),
		Tokens:    14,
	}
	
	conversationContext.Content = append(conversationContext.Content, systemMessage)
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context: %v", err)
	}
	
	// Add user message to context
	userMessage := mcp.ContextItem{
		Role:      "user",
		Content:   "Create a GitHub issue for the login button bug",
		Timestamp: time.Now(),
		Tokens:    10,
	}
	
	conversationContext.Content = append(conversationContext.Content, userMessage)
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context: %v", err)
	}
	
	// Prepare messages for Bedrock (Anthropic Claude format)
	anthropicMessages := []map[string]string{}
	var systemPrompt string
	
	for _, item := range conversationContext.Content {
		if item.Role == "system" {
			systemPrompt = item.Content
		} else {
			anthropicMessages = append(anthropicMessages, map[string]string{
				"role":    item.Role,
				"content": item.Content,
			})
		}
	}
	
	// Create Bedrock request
	bedrockRequest := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        1000,
		"temperature":       0.7,
		"messages":          anthropicMessages,
	}
	
	if systemPrompt != "" {
		bedrockRequest["system"] = systemPrompt
	}
	
	// Convert to JSON
	requestJSON, err := json.Marshal(bedrockRequest)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}
	
	// Send request to Bedrock
	response, err := bedrockClient.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String("anthropic.claude-3-sonnet-20240229-v1:0"),
		Body:        requestJSON,
		ContentType: aws.String("application/json"),
	})
	
	if err != nil {
		log.Fatalf("Failed to invoke model: %v", err)
	}
	
	// Parse response
	var bedrockResponse map[string]interface{}
	if err := json.Unmarshal(response.Body, &bedrockResponse); err != nil {
		log.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	// Extract assistant response
	assistantContent := ""
	if content, ok := bedrockResponse["content"].([]interface{}); ok && len(content) > 0 {
		if contentItem, ok := content[0].(map[string]interface{}); ok {
			if text, ok := contentItem["text"].(string); ok {
				assistantContent = text
			}
		}
	}
	
	log.Printf("Assistant response: %s", assistantContent)
	
	// Add assistant response to context
	assistantMessage := mcp.ContextItem{
		Role:      "assistant",
		Content:   assistantContent,
		Timestamp: time.Now(),
		Tokens:    len(strings.Split(assistantContent, " ")), // Approximate token count
	}
	
	conversationContext.Content = append(conversationContext.Content, assistantMessage)
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context: %v", err)
	}
	
	// Add user follow-up message
	userFollowupMessage := mcp.ContextItem{
		Role:      "user",
		Content:   "Create it in the web-frontend repo with high priority. Title: 'Login button not working on Safari'",
		Timestamp: time.Now(),
		Tokens:    15,
	}
	
	conversationContext.Content = append(conversationContext.Content, userFollowupMessage)
	conversationContext, err = mcpClient.UpdateContext(ctx, conversationContext.ID, conversationContext, nil)
	if err != nil {
		log.Fatalf("Failed to update context: %v", err)
	}
	
	// Get another Bedrock response (similar code omitted for brevity)
	// ...
	
	// Perform GitHub action using the Tool API
	issueParams := map[string]interface{}{
		"owner": "your-organization",
		"repo":  "web-frontend",
		"title": "Login button not working on Safari",
		"body":  "The login button is not working on Safari browsers. This was reported by multiple users.",
		"labels": []string{"bug", "frontend", "priority-high"},
	}
	
	// Execute the action with context
	issueResult, err := mcpClient.ExecuteToolAction(ctx, conversationContext.ID, "github", "create_issue", issueParams)
	if err != nil {
		log.Fatalf("Failed to execute GitHub action: %v", err)
	}
	
	// The issue creation is automatically recorded in the context by the AdapterContextBridge
	log.Printf("Created GitHub issue: %v", issueResult)
	
	// Send event about the completed task
	mcpClient.SendEvent(ctx, &mcp.Event{
		Source:    "agent",
		Type:      "task_complete",
		AgentID:   "my-agent-1",
		SessionID: "user-session-123",
		Data: map[string]interface{}{
			"context_id": conversationContext.ID,
			"task": "create_github_issue",
			"issue_number": issueResult["issue_number"],
		},
	})
	
	// Example of using vector search functionality
	// Generate embeddings for your context items using a model like Amazon Titan Embeddings
	// Store the embeddings in the MCP server
	// Later, search for semantically similar context items based on meaning rather than keywords
	// See examples/vector_search.go for a detailed example
}
```

This example demonstrates how an AI agent can:

1. Create and maintain a conversation context in the MCP server
2. Use Amazon Bedrock for natural language understanding and generation
3. Interact with DevOps tools (GitHub in this case) through the MCP server
4. Track all tool operations automatically in the conversation context
5. Handle multi-turn conversations that involve both AI model calls and tool operations

The MCP server acts as both a context manager and a tool integration hub, giving the agent a unified interface for managing its conversations and performing real-world actions.

### Running Tests

```bash
go test ./...
```

## Configuration

The MCP Server can be configured using a YAML configuration file and/or environment variables. See the `configs/config.yaml.template` file for all available options.

### Environment Variables

All configuration options can be set using environment variables with the `MCP_` prefix. For example:

- `MCP_API_LISTEN_ADDRESS=:8080`
- `MCP_DATABASE_DSN=postgres://user:password@localhost:5432/mcp`
- `MCP_ENGINE_GITHUB_API_TOKEN=your_token`
- `MCP_STORAGE_TYPE=s3` (for using S3 storage)
- `MCP_STORAGE_S3_BUCKET=mcp-contexts` (S3 bucket name)

## API Documentation

### Context API Endpoints

- Create Context: `POST /api/v1/contexts`
- Get Context: `GET /api/v1/contexts/:id`
- Update Context: `PUT /api/v1/contexts/:id`
- Delete Context: `DELETE /api/v1/contexts/:id`
- List Contexts: `GET /api/v1/contexts?agent_id=:agent_id&session_id=:session_id`
- Search Context: `POST /api/v1/contexts/:id/search`
- Summarize Context: `GET /api/v1/contexts/:id/summary`

### Vector API Endpoints

- Store Embedding: `POST /api/v1/vectors/store`
- Search Embeddings: `POST /api/v1/vectors/search`
- Get Context Embeddings: `GET /api/v1/vectors/context/:context_id`
- Delete Context Embeddings: `DELETE /api/v1/vectors/context/:context_id`

### Tool API Endpoints

- Execute Tool Action: `POST /api/v1/tools/:tool/actions/:action?context_id=:context_id`
  (Note: Safety restrictions prevent dangerous operations like deleting repositories)
- Query Tool Data: `POST /api/v1/tools/:tool/query?context_id=:context_id`
  (Note: Read-only access for tools like Artifactory)
- List Available Tools: `GET /api/v1/tools`
- List Allowed Actions: `GET /api/v1/tools/:tool/actions`

### Webhook Endpoints

- Agent Events: `POST /webhook/agent`
- GitHub: `POST /webhook/github`
- Harness: `POST /webhook/harness`
- SonarQube: `POST /webhook/sonarqube`
- Artifactory: `POST /webhook/artifactory`
- Xray: `POST /webhook/xray`

### Health and Metrics

- Health Check: `GET /health`
- Metrics: `GET /metrics`

## Monitoring

The MCP Server integrates with Prometheus and Grafana for monitoring and observability. The Docker Compose setup includes both services.

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

## Vector Search Functionality

The MCP Server supports vector-based semantic search for context management via the PostgreSQL `pg_vector` extension. This enables AI agents to find semantically similar contexts based on meaning rather than just keywords.

### Hybrid Approach

The MCP implements a hybrid approach to vector functionality:

- **MCP Server Responsibilities**:
  - Storing vector embeddings in the database
  - Providing efficient vector similarity search
  - Maintaining the relationship between embeddings and contexts
  - Handling vector indexing for performance

- **Agent Responsibilities**:
  - Generating embeddings using appropriate models
  - Deciding which context items to embed
  - Interpreting vector search results
  - Determining how to use the retrieved similar contexts

### Example Usage

For code examples of how an agent can use the vector functionality, see `examples/vector_search.go`. The typical workflow is:

1. Agent creates a context in MCP server
2. Agent communicates with an LLM service (like Amazon Bedrock)
3. Agent generates embeddings for context items using an embedding model
4. Agent stores these embeddings in the MCP server
5. When a new query arrives, agent generates an embedding for the query
6. Agent uses MCP's vector search to find semantically similar context items
7. Agent retrieves the most relevant context items to enhance its response

## AWS Service Integrations

MCP Server supports integration with AWS services using IAM Roles for Service Accounts (IRSA), providing secure, credential-less authentication for production deployments on EKS.

### IAM Roles for Service Accounts (IRSA)

Instead of managing static AWS credentials, MCP Server can use IRSA to assume IAM roles directly. This provides:

- No hardcoded access keys in configuration files or environment variables
- Temporary, automatically-rotated credentials
- Fine-grained access control using IAM policies
- Simplified security auditing and compliance

For detailed setup instructions, see the [AWS IRSA Setup Guide](docs/aws/aws-irsa-setup.md).

### RDS Aurora PostgreSQL Integration

MCP Server integrates with Amazon RDS Aurora PostgreSQL using IAM authentication:

- Connection pooling optimized for Aurora PostgreSQL
- Automatic token refresh for IAM authentication
- Fallback to standard authentication for local development
- Configurable timeouts and connection parameters

Configuration example:

```yaml
aws:
  rds:
    auth:
      region: "us-west-2"
    host: "your-aurora-cluster.cluster-xxxxxxxxx.us-west-2.rds.amazonaws.com"
    port: 5432
    database: "mcp"
    username: "mcp_admin"
    use_iam_auth: true
    token_expiration: 900 # 15 minutes in seconds
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: 5m
    enable_pooling: true
    min_pool_size: 2
    max_pool_size: 10
    connection_timeout: 30
```

### ElastiCache Redis Integration

MCP Server supports Amazon ElastiCache for Redis with advanced features:

- Support for Redis Cluster Mode with node discovery
- IAM-based authentication for enhanced security
- Automatic connection management and failover handling
- TLS encryption for secure communication
- Optimized connection pooling

Configuration example:

```yaml
aws:
  elasticache:
    auth:
      region: "us-west-2"
    cluster_mode: true
    cluster_name: "mcp-cache"
    username: "mcp_cache_user"
    use_iam_auth: true
    cluster_discovery: true
    use_tls: true
    max_retries: 3
    min_idle_connections: 2
    pool_size: 10
    dial_timeout: 5
    read_timeout: 3
    write_timeout: 3
    pool_timeout: 4
    token_expiration: 900 # 15 minutes in seconds
```

### S3 Storage Functionality

The MCP Server supports storing context data in Amazon S3 with enhanced IAM authentication:

```yaml
aws:
  s3:
    auth:
      region: "us-west-2"
    bucket: "mcp-contexts"
    use_iam_auth: true
    server_side_encryption: "AES256"
    upload_part_size: 5242880 # 5MB
    download_part_size: 5242880 # 5MB
    concurrency: 5
    request_timeout: 30s
```

storage:
  type: "s3"
  context_storage:
    provider: "s3"
    s3_path_prefix: "contexts"
```

For local development and testing, the Docker Compose setup includes LocalStack to emulate S3 functionality. See the [Local AWS Development Guide](docs/development/local-aws-auth.md) for more details.

## Architecture

The MCP Server is built with a modular architecture:

- **Adapters**: Interface with external systems (GitHub, Harness, etc.)
- **AdapterContextBridge**: Connects adapter operations with context management
- **ContextManager**: Manages context storage, retrieval, and manipulation
- **Core Engine**: Orchestrates the overall system and manages events
- **API Server**: Provides REST API endpoints for tools and context management
- **Database**: Persists contexts and system state
- **Cache**: Improves performance for frequently accessed contexts
- **Event System**: Handles events from agents and tools
- **Vector Repository**: Manages vector embeddings for semantic search
- **S3 Storage**: Efficiently stores large context data

### Performance Optimizations

- **Concurrency Management**: Worker pools with configurable limits
- **Caching Strategy**: Multi-level caching with intelligent invalidation
- **Context Truncation**: Several strategies for managing context size efficiently
- **Database Optimizations**: Connection pooling and prepared statements
- **Resilience Patterns**: Circuit breakers and retry mechanisms

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
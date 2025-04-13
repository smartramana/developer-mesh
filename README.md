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

## How It Works with AI Agents

The MCP Server provides a context management system that AI agents can leverage when interacting with model providers like Amazon Bedrock. The typical workflow is:

1. **Agent Initialization**: The AI agent connects to both the MCP Server and Amazon Bedrock
2. **Context Creation**: The agent creates a context in the MCP Server to store conversation history
3. **Direct Inference**: The agent makes direct calls to Amazon Bedrock for model inference, providing the prompt/context
4. **Context Management**: The agent stores responses and manages conversation context through the MCP Server
5. **Context Retrieval**: When needed, the agent retrieves context from the MCP Server to maintain conversation continuity
6. **Tool Interaction**: The agent uses the MCP Server to interact with DevOps tools like GitHub while maintaining context

This architecture allows agents to leverage Amazon Bedrock's wide variety of models while maintaining sophisticated context management capabilities and interacting with real-world tools.

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
```

### Running with Docker Compose

The easiest way to run the MCP Server is using Docker Compose:

```bash
docker-compose up -d
```

This will start the MCP Server along with its dependencies (PostgreSQL, Redis, Prometheus, and Grafana).

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
	
	// Extract assistant response (e.g. "I'll create a GitHub issue for the login button bug. 
	// What repository should I create it in, and what details would you like to include?")
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

## API Documentation

### Context API Endpoints

- Create Context: `POST /api/v1/contexts`
- Get Context: `GET /api/v1/contexts/:id`
- Update Context: `PUT /api/v1/contexts/:id`
- Delete Context: `DELETE /api/v1/contexts/:id`
- List Contexts: `GET /api/v1/contexts?agent_id=:agent_id&session_id=:session_id`
- Search Context: `POST /api/v1/contexts/:id/search`
- Summarize Context: `GET /api/v1/contexts/:id/summary`

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

### Performance Optimizations

- **Concurrency Management**: Worker pools with configurable limits
- **Caching Strategy**: Multi-level caching with intelligent invalidation
- **Context Truncation**: Several strategies for managing context size efficiently
- **Database Optimizations**: Connection pooling and prepared statements
- **Resilience Patterns**: Circuit breakers and retry mechanisms

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
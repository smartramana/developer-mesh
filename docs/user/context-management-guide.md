# Context Management Guide

This guide explains how to use the context management features of the MCP Server, which enables AI agents to maintain conversation histories, track tool operations, and provide more intelligent responses based on historical interactions.

## Table of Contents

1. [Overview](#overview)
2. [Context Structure](#context-structure)
3. [Creating Contexts](#creating-contexts)
4. [Retrieving Contexts](#retrieving-contexts)
5. [Updating Contexts](#updating-contexts)
6. [Truncation Strategies](#truncation-strategies)
7. [Searching Within Contexts](#searching-within-contexts)
8. [Multi-Tiered Storage](#multi-tiered-storage)
9. [Performance Considerations](#performance-considerations)
10. [Security Considerations](#security-considerations)
11. [AWS Integration](#aws-integration)
12. [Using the API](#using-the-api)
13. [Best Practices](#best-practices)

## Overview

The context management system enables AI agents to:

- Store and retrieve conversation histories
- Track tool operations and their results
- Maintain state across multiple interactions
- Search for relevant information in previous interactions
- Optimize token usage through intelligent truncation
- Handle both short-term and long-term memory needs

## Context Structure

A context consists of:

- **Metadata**: Information about the context itself
- **Content Items**: The actual conversation messages, events, and tool operations

### Context Object

The context object has the following structure:

```json
{
  "id": "context-123",                  // Unique identifier
  "agent_id": "claude",                 // ID of the AI agent
  "model_id": "claude-3-haiku",         // ID of the AI model
  "session_id": "user-session-456",     // Optional session identifier
  "current_tokens": 750,                // Current token count
  "max_tokens": 4000,                   // Maximum token limit
  "metadata": {                         // Optional metadata
    "user_id": "user-789",
    "language": "en-US",
    "custom_field": "custom value"
  },
  "content": [                          // Array of context items
    {
      "role": "system",
      "content": "You are a DevOps assistant.",
      "tokens": 6,
      "timestamp": "2025-04-19T10:30:00Z"
    },
    {
      "role": "user",
      "content": "Show me the open pull requests.",
      "tokens": 6,
      "timestamp": "2025-04-19T10:31:00Z"
    },
    {
      "role": "tool",
      "content": "github.list_pull_requests",
      "tokens": 3,
      "timestamp": "2025-04-19T10:31:05Z",
      "metadata": {
        "adapter_type": "github",
        "operation": "list_pull_requests",
        "params": {
          "owner": "org-name",
          "repo": "repo-name",
          "state": "open"
        },
        "result": "...(truncated)..."
      }
    },
    {
      "role": "assistant",
      "content": "Here are the open pull requests...",
      "tokens": 735,
      "timestamp": "2025-04-19T10:31:10Z"
    }
  ],
  "created_at": "2025-04-19T10:30:00Z",
  "updated_at": "2025-04-19T10:31:10Z",
  "expires_at": "2025-04-26T10:30:00Z"
}
```

### Context Item Roles

The system supports different roles for context items:

- **system**: Instructions and system prompts
- **user**: Messages from the user
- **assistant**: Responses from the AI assistant
- **tool**: Tool operations and their results
- **webhook**: Events from external systems via webhooks

## Creating Contexts

To create a new context, use the `POST /api/v1/contexts` endpoint:

```json
// Request
POST /api/v1/contexts
{
  "agent_id": "claude",
  "model_id": "claude-3-haiku",
  "session_id": "user-session-456",
  "max_tokens": 4000,
  "content": [
    {
      "role": "system",
      "content": "You are a DevOps assistant.",
      "tokens": 6
    }
  ],
  "metadata": {
    "user_id": "user-789"
  }
}

// Response
{
  "id": "context-123",
  "agent_id": "claude",
  "model_id": "claude-3-haiku",
  "session_id": "user-session-456",
  "current_tokens": 6,
  "max_tokens": 4000,
  "content": [
    {
      "id": "item-abc",
      "role": "system",
      "content": "You are a DevOps assistant.",
      "tokens": 6,
      "timestamp": "2025-04-19T10:30:00Z"
    }
  ],
  "metadata": {
    "user_id": "user-789"
  },
  "created_at": "2025-04-19T10:30:00Z",
  "updated_at": "2025-04-19T10:30:00Z",
  "_links": {
    "self": "/api/v1/contexts/context-123",
    "summary": "/api/v1/contexts/context-123/summary",
    "search": "/api/v1/contexts/context-123/search"
  }
}
```

Required fields:
- `agent_id`: Identifier for the AI agent
- `model_id`: Identifier for the AI model

Optional fields:
- `session_id`: User session identifier
- `max_tokens`: Maximum token limit (default: 4000)
- `content`: Initial context items
- `metadata`: Additional metadata

## Retrieving Contexts

### Get a Context by ID

```json
// Request
GET /api/v1/contexts/context-123

// Response
{
  "id": "context-123",
  "agent_id": "claude",
  "model_id": "claude-3-haiku",
  "session_id": "user-session-456",
  "current_tokens": 6,
  "max_tokens": 4000,
  "content": [ ... ],
  "metadata": { ... },
  "created_at": "2025-04-19T10:30:00Z",
  "updated_at": "2025-04-19T10:30:00Z",
  "_links": { ... }
}
```

You can also exclude content items for a lighter response:

```
GET /api/v1/contexts/context-123?include_content=false
```

### List Contexts for an Agent

```json
// Request
GET /api/v1/contexts?agent_id=claude&session_id=user-session-456&limit=10

// Response
[
  {
    "id": "context-123",
    "agent_id": "claude",
    "model_id": "claude-3-haiku",
    "session_id": "user-session-456",
    "current_tokens": 6,
    "max_tokens": 4000,
    "content": [],  // Content is empty in list responses
    "metadata": { ... },
    "created_at": "2025-04-19T10:30:00Z",
    "updated_at": "2025-04-19T10:30:00Z",
    "_links": { ... }
  },
  ...
]
```

Required parameters:
- `agent_id`: Agent identifier

Optional parameters:
- `session_id`: Filter by session
- `limit`: Maximum number of contexts to return (default: 20)

### Get a Context Summary

```json
// Request
GET /api/v1/contexts/context-123/summary

// Response
{
  "context_id": "context-123",
  "summary": "4 messages (1 user, 1 assistant, 1 system, 1 tool), 750/4000 tokens",
  "_links": {
    "self": "/api/v1/contexts/context-123/summary",
    "context": "/api/v1/contexts/context-123"
  }
}
```

## Updating Contexts

To update a context, use the `PUT /api/v1/contexts/:contextID` endpoint:

```json
// Request
PUT /api/v1/contexts/context-123
{
  "context": {
    "content": [
      {
        "role": "user",
        "content": "Show me the open pull requests.",
        "tokens": 6
      }
    ]
  },
  "options": {
    "truncate": true,
    "truncate_strategy": "oldest_first"
  }
}

// Response
{
  "id": "context-123",
  "agent_id": "claude",
  "model_id": "claude-3-haiku",
  "session_id": "user-session-456",
  "current_tokens": 12,
  "max_tokens": 4000,
  "content": [ ... ],
  "metadata": { ... },
  "created_at": "2025-04-19T10:30:00Z",
  "updated_at": "2025-04-19T10:31:00Z",
  "_links": { ... }
}
```

The `context` object supports partial updates:
- `content`: Additional content items to append
- `session_id`: Update session ID
- `metadata`: Update metadata

Update options:
- `truncate`: Whether to truncate the context if it exceeds max tokens
- `truncate_strategy`: Strategy for truncation (oldest_first, preserving_user, relevance_based)
- `replace_content`: Whether to replace the entire content instead of appending

## Truncation Strategies

The system provides multiple strategies for handling contexts that exceed the maximum token limit:

### Oldest First

Removes the oldest context items first until the token count is below the maximum limit.

```json
"options": {
  "truncate": true,
  "truncate_strategy": "oldest_first"
}
```

### Preserving User

Prioritizes removing assistant responses while preserving user messages.

```json
"options": {
  "truncate": true,
  "truncate_strategy": "preserving_user"
}
```

### Relevance Based

Uses vector embeddings to remove less relevant context items. (Note: This requires PostgreSQL with pgvector extension.)

```json
"options": {
  "truncate": true,
  "truncate_strategy": "relevance_based",
  "relevance_parameters": {
    "query": "current discussion topic"
  }
}
```

## Searching Within Contexts

You can search for specific content within a context:

```json
// Request
POST /api/v1/contexts/context-123/search
{
  "query": "pull request"
}

// Response
{
  "context_id": "context-123",
  "query": "pull request",
  "results": [
    {
      "id": "item-def",
      "role": "user",
      "content": "Show me the open pull requests.",
      "tokens": 6,
      "timestamp": "2025-04-19T10:31:00Z"
    },
    {
      "id": "item-ghi",
      "role": "assistant",
      "content": "Here are the open pull requests...",
      "tokens": 735,
      "timestamp": "2025-04-19T10:31:10Z"
    }
  ],
  "_links": {
    "self": "/api/v1/contexts/context-123/search",
    "context": "/api/v1/contexts/context-123"
  }
}
```

## Multi-Tiered Storage

The context management system uses a multi-tiered storage approach:

1. **Redis Cache**: 
   - For fast access to frequently used contexts
   - TTL-based caching (default: 24 hours)
   - Used for recently accessed contexts

2. **PostgreSQL Database**: 
   - For structured storage of context metadata and items
   - Efficient querying by agent ID, session ID, etc.
   - Vector search capabilities (with pgvector extension)

3. **S3 Storage**: 
   - For large context data
   - Efficient storage for long-term retention
   - Cost-effective for high-volume deployments

This architecture provides optimal performance while ensuring durability and cost-effectiveness.

## Performance Considerations

### Token Calculation

The system requires token counts for each context item to track the overall token usage. You can:

1. Calculate tokens on the client side and provide them in the request
2. Let the server estimate tokens based on a simple heuristic (1 token ≈ 4 characters)

For accurate tracking, it's recommended to calculate tokens on the client side using the appropriate tokenizer for your model.

### Batch Operations

For adding multiple items to a context, use a single update operation rather than multiple individual updates:

```json
// Good: Single update with multiple items
PUT /api/v1/contexts/context-123
{
  "context": {
    "content": [
      { "role": "user", "content": "Message 1", "tokens": 3 },
      { "role": "assistant", "content": "Response 1", "tokens": 3 },
      { "role": "user", "content": "Message 2", "tokens": 3 }
    ]
  }
}

// Bad: Multiple individual updates
PUT /api/v1/contexts/context-123
{
  "context": {
    "content": [
      { "role": "user", "content": "Message 1", "tokens": 3 }
    ]
  }
}

PUT /api/v1/contexts/context-123
{
  "context": {
    "content": [
      { "role": "assistant", "content": "Response 1", "tokens": 3 }
    ]
  }
}
```

### Caching Strategy

Contexts are automatically cached in Redis for fast access. The cache is:

- Updated on any context modification
- Invalidated on context deletion
- Subject to TTL-based expiration (default: 24 hours)

For high-traffic systems, consider adjusting the Redis connection pool settings and TTL values in the configuration.

## Security Considerations

### Authentication and Authorization

All context API endpoints require authentication using JWT tokens or API keys.

Authorization is enforced based on:
- Agent ID: Contexts are isolated by agent ID
- Session ID: Optional additional isolation by session

### Data Isolation

The system ensures strict isolation between contexts belonging to different agents. An agent can only access its own contexts.

### Access Control

API endpoints support fine-grained access control:
- `GET` endpoints: Read-only access
- `POST`, `PUT`, `DELETE` endpoints: Read-write access

For production deployments, consider implementing additional access controls using API gateway policies or custom middleware.

## AWS Integration

For deploying in AWS environments, the system supports:

### IAM Roles for Service Accounts (IRSA)

Use IRSA for secure, credential-less authentication with AWS services:

1. Create the IAM role and policy using the provided scripts
2. Apply the service account configuration to your EKS cluster
3. Configure the deployment to use the service account

### S3 Integration

Configure S3 storage for context data:

```yaml
aws:
  s3:
    region: "us-west-2"
    bucket: "mcp-contexts"
    prefix: "contexts/"
    use_iam_auth: true
```

### RDS Aurora Integration

Configure RDS Aurora PostgreSQL for database storage:

```yaml
aws:
  rds:
    host: "your-cluster.cluster-xxxxx.us-west-2.rds.amazonaws.com"
    port: 5432
    database: "mcp"
    username: "mcp_app"
    use_iam_auth: true
```

### ElastiCache Integration

Configure ElastiCache for Redis caching:

```yaml
aws:
  elasticache:
    cluster_mode: true
    cluster_name: "mcp-cache"
    use_iam_auth: true
```

## Using the API

### With MCP Client Library

```go
package main

import (
	"context"
	"log"

	"github.com/S-Corkum/mcp-server/pkg/client"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

func main() {
	// Initialize MCP client
	mcpClient := client.NewClient(
		"http://localhost:8080",
		client.WithAPIKey("your-api-key"),
	)
	
	ctx := context.Background()
	
	// Create a context
	contextData := &mcp.Context{
		AgentID: "claude",
		ModelID: "claude-3-haiku",
		MaxTokens: 4000,
		Content: []mcp.ContextItem{
			{
				Role: "system",
				Content: "You are a DevOps assistant.",
				Tokens: 6,
			},
		},
	}
	
	createdContext, err := mcpClient.CreateContext(ctx, contextData)
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}
	
	// Update the context
	userMessage := mcp.ContextItem{
		Role: "user",
		Content: "Show me the open pull requests.",
		Tokens: 6,
	}
	
	updateData := &mcp.Context{
		Content: []mcp.ContextItem{userMessage},
	}
	
	options := &mcp.ContextUpdateOptions{
		Truncate: true,
		TruncateStrategy: "oldest_first",
	}
	
	updatedContext, err := mcpClient.UpdateContext(ctx, createdContext.ID, updateData, options)
	if err != nil {
		log.Fatalf("Failed to update context: %v", err)
	}
	
	// Use the context ID when executing tool actions
	issueParams := map[string]interface{}{
		"owner": "your-organization",
		"repo":  "web-frontend",
		"title": "Login button not working on Safari",
		"body":  "The login button is not working on Safari browsers.",
	}
	
	issueResult, err := mcpClient.ExecuteToolAction(ctx, updatedContext.ID, "github", "create_issue", issueParams)
	if err != nil {
		log.Fatalf("Failed to execute GitHub action: %v", err)
	}
	
	// Get the updated context after the tool operation
	finalContext, err := mcpClient.GetContext(ctx, updatedContext.ID)
	if err != nil {
		log.Fatalf("Failed to get context: %v", err)
	}
	
	log.Printf("Context now has %d items and %d tokens", len(finalContext.Content), finalContext.CurrentTokens)
}
```

### With Direct HTTP Requests

```python
import requests
import json

BASE_URL = "http://localhost:8080/api/v1"
API_KEY = "your-api-key"

headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

# Create a context
create_payload = {
    "agent_id": "claude",
    "model_id": "claude-3-haiku",
    "max_tokens": 4000,
    "content": [
        {
            "role": "system",
            "content": "You are a DevOps assistant.",
            "tokens": 6
        }
    ]
}

response = requests.post(
    f"{BASE_URL}/contexts",
    headers=headers,
    json=create_payload
)

context_data = response.json()
context_id = context_data["id"]

# Update the context
update_payload = {
    "context": {
        "content": [
            {
                "role": "user",
                "content": "Show me the open pull requests.",
                "tokens": 6
            }
        ]
    },
    "options": {
        "truncate": True,
        "truncate_strategy": "oldest_first"
    }
}

response = requests.put(
    f"{BASE_URL}/contexts/{context_id}",
    headers=headers,
    json=update_payload
)

# Execute a tool action with the context
action_payload = {
    "owner": "your-organization",
    "repo": "web-frontend",
    "title": "Login button not working on Safari",
    "body": "The login button is not working on Safari browsers."
}

response = requests.post(
    f"{BASE_URL}/tools/github/actions/create_issue?context_id={context_id}",
    headers=headers,
    json=action_payload
)
```

## Best Practices

### Context Management

1. **Create Contexts for Each Conversation**: Create a new context for each distinct conversation.
2. **Session Tracking**: Use the `session_id` parameter to group contexts by user session.
3. **Token Management**: Regularly monitor token usage and apply truncation when needed.
4. **Context Expiration**: Set appropriate `expires_at` values for context lifetime management.

### Performance Optimization

1. **Limit Content Retrieval**: Use `include_content=false` when listing contexts.
2. **Batch Updates**: Combine multiple context updates into a single request.
3. **Caching**: Leverage the built-in caching for frequently accessed contexts.
4. **Truncation Planning**: Choose the appropriate truncation strategy based on your use case.

### Security

1. **Limit Access**: Restrict context access to authorized agents.
2. **Audit Logging**: Enable audit logging for context operations.
3. **Data Isolation**: Ensure strict isolation between different agents' contexts.
4. **Data Minimization**: Store only necessary information in contexts.

### Monitoring

1. **Token Usage**: Monitor context token usage to prevent excessive consumption.
2. **Storage Metrics**: Track storage usage across Redis, PostgreSQL, and S3.
3. **Operation Latencies**: Monitor context operation latencies for performance optimization.
4. **Error Rates**: Track error rates for context operations to identify issues.

By following these practices, you can effectively use the context management system to enhance AI agent interactions while maintaining performance and security.

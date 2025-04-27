# AI Agent Integration Guide

This comprehensive guide explains how to integrate AI agents with the MCP Server for DevOps tool integration, context management, and vector search capabilities.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Authentication](#authentication)
- [Client Libraries](#client-libraries)
- [Core Integration Workflow](#core-integration-workflow)
  - [Context Management](#context-management)
  - [Tool Discovery and Integration](#tool-discovery-and-integration)
  - [Executing Tool Operations](#executing-tool-operations)
  - [Vector Operations for Semantic Search](#vector-operations-for-semantic-search)
  - [Webhooks and Event Processing](#webhooks-and-event-processing)
- [Complete Integration Example](#complete-integration-example)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [API Reference](#api-reference)

## Overview

MCP Server provides a RESTful API that AI agents can use to:

1. **Manage conversation contexts** - Create, update, retrieve, and search conversation histories
2. **Execute DevOps tool operations** - Perform actions on GitHub and other supported tools
3. **Store and search vector embeddings** - Enable semantic search within conversations
4. **Process events from external systems** - Handle webhooks from integrated services

This guide covers everything you need to know to successfully integrate your AI agent with the MCP Server.

## Prerequisites

Before integrating with the MCP Server, ensure you have:

- MCP Server endpoint URL (e.g., `http://localhost:8080` or your production URL)
- Authentication credentials (API key or JWT token)
- Basic understanding of the Model Context Protocol
- An AI agent that can make HTTP requests

## Authentication

MCP Server supports two authentication methods:

### API Key Authentication

For most use cases, API key authentication is the simplest approach:

```http
Authorization: Bearer YOUR_API_KEY
```

API keys are defined in the MCP Server configuration and can be assigned different permission levels.

### JWT Authentication

For more advanced use cases, JWT authentication provides finer-grained control:

```http
Authorization: Bearer YOUR_JWT_TOKEN
```

JWT tokens include claims that specify permissions and expiration times.

## Client Libraries

### Official Client Libraries

MCP Server provides official client libraries for Go and Python:

#### Go Client

```go
import (
    "github.com/S-Corkum/mcp-server/pkg/client"
)

// Create client with API key authentication
mcpClient := client.NewClient(
    "https://your-mcp-server.example.com",
    client.WithAPIKey("your-api-key"),
)

// Or with JWT authentication
mcpClient := client.NewClient(
    "https://your-mcp-server.example.com",
    client.WithJWT("your-jwt-token"),
)
```

#### Python Client

```python
from mcp_client import MCPClient

# Create client with API key authentication
mcp_client = MCPClient(
    base_url="https://your-mcp-server.example.com",
    api_key="your-api-key"
)

# Or with JWT authentication
mcp_client = MCPClient(
    base_url="https://your-mcp-server.example.com",
    jwt_token="your-jwt-token"
)
```

### Custom HTTP Client

For languages without an official client, you can make HTTP requests directly:

```python
import requests

class CustomMCPClient:
    def __init__(self, base_url, api_key=None, jwt_token=None):
        self.base_url = base_url
        self.headers = {
            "Content-Type": "application/json"
        }
        
        if api_key:
            self.headers["Authorization"] = f"Bearer {api_key}"
        elif jwt_token:
            self.headers["Authorization"] = f"Bearer {jwt_token}"
    
    def create_context(self, context_data):
        response = requests.post(
            f"{self.base_url}/api/v1/contexts",
            headers=self.headers,
            json=context_data
        )
        return response.json()
        
    # Implement other methods as needed
```

## Core Integration Workflow

A typical AI agent integration follows these main steps:

1. **Create or retrieve a context** - Establish a conversation context to track interactions
2. **Discover available tools** - Understand what tools and actions are available
3. **Execute tool operations** - Perform actions based on user requests
4. **Update the context** - Record interactions in the conversation history
5. **Search within contexts** - Find relevant information using text or vector search

Let's explore each step in detail.

### Context Management

#### Creating a Context

A context represents a conversation between the AI agent and a user. It stores conversation history and metadata.

##### Go Example

```go
import (
    "context"
    "github.com/S-Corkum/mcp-server/pkg/mcp"
)

// Create a context for the conversation
contextData := &mcp.Context{
    AgentID:   "my-agent-id",     // Unique identifier for your agent
    ModelID:   "gpt-4",           // The AI model used
    SessionID: "user-session-123", // Optional session identifier
    MaxTokens: 4000,               // Maximum context window size
    Content: []mcp.ContextItem{
        {
            Role:    "system",
            Content: "You are a DevOps assistant that helps with GitHub operations.",
            Tokens:  12,  // Token count for the content
        },
    },
}

ctx := context.Background()
createdContext, err := mcpClient.CreateContext(ctx, contextData)
if err != nil {
    log.Fatalf("Failed to create context: %v", err)
}

contextID := createdContext.ID
```

##### Python Example

```python
# Create a context for the conversation
context_data = {
    "agent_id": "my-agent-id",
    "model_id": "gpt-4",
    "session_id": "user-session-123",
    "max_tokens": 4000,
    "content": [
        {
            "role": "system",
            "content": "You are a DevOps assistant that helps with GitHub operations.",
            "tokens": 12
        }
    ]
}

created_context = mcp_client.create_context(context_data)
context_id = created_context["id"]
```

#### Retrieving a Context

You can retrieve an existing context by its ID:

##### Go Example

```go
context, err := mcpClient.GetContext(ctx, contextID)
if err != nil {
    log.Fatalf("Failed to get context: %v", err)
}

// Access context properties
log.Printf("Context: %v", context)
```

##### Python Example

```python
context = mcp_client.get_context(context_id)
print(f"Context: {context}")
```

#### Updating a Context

As the conversation progresses, update the context with new messages:

##### Go Example

```go
// Update context with new message
updateData := &mcp.Context{
    Content: []mcp.ContextItem{
        {
            Role:    "user",
            Content: "Can you create a GitHub issue for the login bug?",
            Tokens:  10,
        },
    },
}

options := &mcp.ContextUpdateOptions{
    Truncate:         true,
    TruncateStrategy: "oldest_first",
}

updatedContext, err := mcpClient.UpdateContext(ctx, contextID, updateData, options)
if err != nil {
    log.Fatalf("Failed to update context: %v", err)
}
```

##### Python Example

```python
# Update context with new message
update_data = {
    "content": [
        {
            "role": "user",
            "content": "Can you create a GitHub issue for the login bug?",
            "tokens": 10
        }
    ]
}

options = {
    "truncate": True,
    "truncate_strategy": "oldest_first"
}

updated_context = mcp_client.update_context(context_id, update_data, options)
```

#### Listing Contexts

You can list all contexts for an agent:

##### Go Example

```go
contexts, err := mcpClient.ListContexts(ctx, "my-agent-id", "", map[string]interface{}{
    "limit": 10,
})
if err != nil {
    log.Fatalf("Failed to list contexts: %v", err)
}

for _, context := range contexts {
    log.Printf("Context ID: %s, Updated At: %s", context.ID, context.UpdatedAt)
}
```

##### Python Example

```python
contexts = mcp_client.list_contexts("my-agent-id", session_id="", options={"limit": 10})
for context in contexts:
    print(f"Context ID: {context['id']}, Updated At: {context['updated_at']}")
```

### Tool Discovery and Integration

#### Discovering Available Tools

Before executing tool operations, discover what tools are available:

##### Go Example

```go
// List available tools
tools, err := mcpClient.ListTools(ctx)
if err != nil {
    log.Fatalf("Failed to list tools: %v", err)
}

// Print available tools
for _, tool := range tools {
    log.Printf("Tool: %s - %s", tool.Name, tool.Description)
    log.Printf("Available actions: %v", tool.Actions)
}
```

##### Python Example

```python
# List available tools
tools = mcp_client.list_tools()
for tool in tools:
    print(f"Tool: {tool['name']} - {tool['description']}")
    print(f"Available actions: {tool['actions']}")
```

#### Getting Tool Actions

Get detailed information about available actions for a specific tool:

##### Go Example

```go
// Get GitHub actions
actions, err := mcpClient.ListToolActions(ctx, "github")
if err != nil {
    log.Fatalf("Failed to list GitHub actions: %v", err)
}

for _, action := range actions {
    log.Printf("Action: %s - %s", action.Name, action.Description)
    log.Printf("Required Parameters: %v", action.RequiredParameters)
}
```

##### Python Example

```python
# Get GitHub actions
actions = mcp_client.list_tool_actions("github")
for action in actions:
    print(f"Action: {action['name']} - {action['description']}")
    print(f"Required Parameters: {action['required_parameters']}")
```

### Executing Tool Operations

#### Executing an Action

Once you know what tools and actions are available, you can execute them:

##### Go Example

```go
// Execute GitHub action to create an issue
issueParams := map[string]interface{}{
    "owner": "your-organization",
    "repo":  "your-repo",
    "title": "Bug in login form",
    "body":  "The login button is not working on Safari browsers.",
    "labels": []string{"bug", "frontend", "priority-high"},
}

// Execute the action with context
issueResult, err := mcpClient.ExecuteToolAction(
    ctx, 
    contextID, 
    "github", 
    "create_issue", 
    issueParams,
)
if err != nil {
    log.Fatalf("Failed to execute GitHub action: %v", err)
}

log.Printf("Created GitHub issue: %v", issueResult)
```

##### Python Example

```python
# Execute GitHub action to create an issue
issue_params = {
    "owner": "your-organization",
    "repo": "your-repo",
    "title": "Bug in login form",
    "body": "The login button is not working on Safari browsers.",
    "labels": ["bug", "frontend", "priority-high"]
}

# Execute the action with context
issue_result = mcp_client.execute_tool_action(
    context_id,
    "github",
    "create_issue",
    issue_params
)

print(f"Created GitHub issue: {issue_result}")
```

#### Querying Tool Data

You can also query data from tools:

##### Go Example

```go
// Query GitHub repositories
queryParams := map[string]interface{}{
    "owner": "your-organization",
    "type": "public",
}

queryResult, err := mcpClient.QueryToolData(ctx, "github", queryParams)
if err != nil {
    log.Fatalf("Failed to query GitHub data: %v", err)
}

log.Printf("Query result: %v", queryResult)
```

##### Python Example

```python
# Query GitHub repositories
query_params = {
    "owner": "your-organization",
    "type": "public"
}

query_result = mcp_client.query_tool_data("github", query_params)
print(f"Query result: {query_result}")
```

### Vector Operations for Semantic Search

MCP Server supports storing and searching vector embeddings for semantic search:

#### Storing an Embedding

##### Go Example

```go
// Store an embedding
embeddingRequest := map[string]interface{}{
    "context_id": contextID,
    "content_index": 0,
    "text": "Help me create a GitHub issue for the login bug",
    "embedding": []float32{0.1, 0.2, 0.3}, // Your actual embedding vector here
    "model_id": "text-embedding-ada-002",
}

_, err = mcpClient.StoreEmbedding(ctx, embeddingRequest)
if err != nil {
    log.Fatalf("Failed to store embedding: %v", err)
}
```

##### Python Example

```python
# Store an embedding
embedding_request = {
    "context_id": context_id,
    "content_index": 0,
    "text": "Help me create a GitHub issue for the login bug",
    "embedding": [0.1, 0.2, 0.3],  # Your actual embedding vector here
    "model_id": "text-embedding-ada-002"
}

mcp_client.store_embedding(embedding_request)
```

#### Searching for Similar Content

##### Go Example

```go
// Search for similar content
searchRequest := map[string]interface{}{
    "context_id": contextID,
    "query_embedding": []float32{0.1, 0.2, 0.3}, // Your query embedding vector
    "limit": 5,
    "model_id": "text-embedding-ada-002",
    "similarity_threshold": 0.7,
}

searchResults, err := mcpClient.SearchEmbeddings(ctx, searchRequest)
if err != nil {
    log.Fatalf("Failed to search embeddings: %v", err)
}

log.Printf("Search results: %v", searchResults)
```

##### Python Example

```python
# Search for similar content
search_request = {
    "context_id": context_id,
    "query_embedding": [0.1, 0.2, 0.3],  # Your query embedding vector
    "limit": 5,
    "model_id": "text-embedding-ada-002",
    "similarity_threshold": 0.7
}

search_results = mcp_client.search_embeddings(search_request)
print(f"Search results: {search_results}")
```

### Webhooks and Event Processing

MCP Server can process webhook events from external systems like GitHub:

#### Setting Up a Webhook Handler

```go
// Setup a webhook handler in your application
func githubWebhookHandler(w http.ResponseWriter, r *http.Request) {
    // Read request body
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read request body", http.StatusBadRequest)
        return
    }
    
    // Get event type from header
    eventType := r.Header.Get("X-GitHub-Event")
    
    // Forward the webhook to MCP Server
    webhookURL := "http://mcp-server:8080/webhook/github"
    req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(body))
    if err != nil {
        http.Error(w, "Failed to create forwarding request", http.StatusInternalServerError)
        return
    }
    
    // Forward all relevant headers
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-GitHub-Event", eventType)
    req.Header.Set("X-Hub-Signature-256", r.Header.Get("X-Hub-Signature-256"))
    
    // Send the request
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        http.Error(w, "Failed to forward webhook", http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()
    
    // Return success
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}
```

## Complete Integration Example

See the [Complete AI Agent Example](../examples/complete-ai-agent-example.md) for a full end-to-end integration example.

## Best Practices

### Context Management

1. **Create separate contexts for different conversations** - Each user session should have its own context
2. **Use appropriate context IDs** - Consider using UUIDs or other unique identifiers that include the agent ID and session ID
3. **Apply proper truncation strategies** - Choose the right strategy to manage context size (`oldest_first`, `preserving_user`, `relevance_based`)
4. **Include system prompts** - Add system prompts to guide the AI agent's behavior
5. **Track token usage** - Monitor token counts to optimize context management
6. **Consider context expiration** - Set appropriate expiration times for contexts that aren't actively used

### Error Handling

1. **Implement proper error handling** - Check for and handle all API errors appropriately
2. **Add retries for transient failures** - Implement retry logic with exponential backoff for network issues
3. **Gracefully handle rate limits** - Respect rate limits and implement appropriate backoff strategies
4. **Provide meaningful error messages** - Give users clear error messages when operations fail
5. **Log errors for debugging** - Maintain logs of API interactions for troubleshooting

### Security

1. **Store secrets securely** - Never hardcode API keys or tokens in your code
2. **Use environment variables or secret management** - Store sensitive credentials in environment variables or a secret management system
3. **Implement proper authentication** - Always authenticate API requests
4. **Validate webhook signatures** - Verify the authenticity of webhook events
5. **Use HTTPS** - Always use TLS/SSL for production deployments
6. **Implement proper authorization** - Ensure that API keys have the appropriate permissions

### Performance

1. **Reuse context IDs** - Don't create a new context for every interaction in the same conversation
2. **Use caching** - Cache frequently accessed data like available tools and actions
3. **Optimize embedding operations** - Store and retrieve embeddings efficiently
4. **Minimize unnecessary API calls** - Batch operations when possible
5. **Track performance metrics** - Monitor response times and resource usage

## Troubleshooting

See the [Troubleshooting Guide](../troubleshooting-guide.md) for detailed information on common issues and their solutions.

## API Reference

For complete details on all available API endpoints, request/response formats, and authentication methods, see the [API Reference](../api-reference.md).

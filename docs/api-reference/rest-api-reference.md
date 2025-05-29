# REST API Reference

Complete API reference for the DevOps MCP REST API service.

## Overview

The REST API service provides comprehensive data management and search capabilities for the DevOps MCP platform.

### Base URL
```
Production: https://api.devops-mcp.com/api/v1
Staging:    https://staging-api.devops-mcp.com/api/v1
Local:      http://localhost:8081/api/v1
```

### Authentication

All API endpoints (except health checks) require authentication:

```bash
# API Key Authentication
curl -H "X-API-Key: your-api-key" https://api.devops-mcp.com/api/v1/contexts

# JWT Bearer Token
curl -H "Authorization: Bearer eyJhbGc..." https://api.devops-mcp.com/api/v1/contexts
```

### Rate Limiting

| Tier | Requests/Minute | Requests/Day |
|------|----------------|--------------|
| Free | 60 | 10,000 |
| Pro | 300 | 100,000 |
| Enterprise | Custom | Unlimited |

Rate limit headers:
```http
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 299
X-RateLimit-Reset: 1642521600
Retry-After: 60
```

## Health & Monitoring Endpoints

### Health Check
Check the health status of all components.

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "redis": "healthy",
    "vector_db": "healthy"
  },
  "timestamp": "2024-01-20T10:30:00Z"
}
```

### Metrics
Prometheus-compatible metrics endpoint.

```http
GET /metrics
```

## Context Management API

### Create Context
Create a new context for storing conversation or document data.

```http
POST /api/v1/contexts
```

**Request Body:**
```json
{
  "agent_id": "agent-123",
  "session_id": "session-456",
  "content": [
    {
      "type": "message",
      "role": "user",
      "content": "Hello, I need help with OAuth 2.0"
    }
  ],
  "metadata": {
    "source": "chat",
    "language": "en"
  }
}
```

**Response (201 Created):**
```json
{
  "id": "ctx-789",
  "agent_id": "agent-123",
  "session_id": "session-456",
  "created_at": "2024-01-20T10:30:00Z",
  "updated_at": "2024-01-20T10:30:00Z",
  "_links": {
    "self": "/api/v1/contexts/ctx-789",
    "search": "/api/v1/contexts/ctx-789/search",
    "summary": "/api/v1/contexts/ctx-789/summary"
  }
}
```

### List Contexts
Retrieve all contexts with optional filtering.

```http
GET /api/v1/contexts?agent_id=agent-123&limit=20&offset=0
```

**Query Parameters:**
- `agent_id` (optional): Filter by agent ID
- `session_id` (optional): Filter by session ID
- `limit` (optional): Number of results (default: 50, max: 100)
- `offset` (optional): Pagination offset
- `sort` (optional): Sort field (created_at, updated_at)
- `order` (optional): Sort order (asc, desc)

**Response:**
```json
{
  "contexts": [
    {
      "id": "ctx-789",
      "agent_id": "agent-123",
      "session_id": "session-456",
      "created_at": "2024-01-20T10:30:00Z",
      "updated_at": "2024-01-20T10:30:00Z"
    }
  ],
  "total": 156,
  "limit": 20,
  "offset": 0,
  "_links": {
    "self": "/api/v1/contexts?agent_id=agent-123&limit=20&offset=0",
    "next": "/api/v1/contexts?agent_id=agent-123&limit=20&offset=20",
    "first": "/api/v1/contexts?agent_id=agent-123&limit=20&offset=0",
    "last": "/api/v1/contexts?agent_id=agent-123&limit=20&offset=140"
  }
}
```

### Get Context
Retrieve a specific context by ID.

```http
GET /api/v1/contexts/:contextID
```

**Response:**
```json
{
  "id": "ctx-789",
  "agent_id": "agent-123",
  "session_id": "session-456",
  "content": [
    {
      "type": "message",
      "role": "user",
      "content": "Hello, I need help with OAuth 2.0"
    }
  ],
  "metadata": {
    "source": "chat",
    "language": "en"
  },
  "created_at": "2024-01-20T10:30:00Z",
  "updated_at": "2024-01-20T10:30:00Z"
}
```

### Update Context
Update an existing context.

```http
PUT /api/v1/contexts/:contextID
```

**Request Body:**
```json
{
  "content": [
    {
      "type": "message",
      "role": "assistant",
      "content": "I'll help you understand OAuth 2.0..."
    }
  ],
  "metadata": {
    "updated_by": "assistant-1"
  }
}
```

### Delete Context
Delete a context and all associated data.

```http
DELETE /api/v1/contexts/:contextID
```

**Response (204 No Content)**

### Get Context Summary
Generate an AI-powered summary of a context.

```http
GET /api/v1/contexts/:contextID/summary
```

**Response:**
```json
{
  "summary": "Discussion about OAuth 2.0 implementation, covering authorization flows, token management, and security best practices.",
  "key_topics": ["OAuth 2.0", "Authorization", "Security"],
  "word_count": 1250,
  "message_count": 15
}
```

### Search Within Context
Search for specific content within a context.

```http
POST /api/v1/contexts/:contextID/search
```

**Request Body:**
```json
{
  "query": "authorization flow",
  "limit": 10
}
```

## Tool Integration API

### List Available Tools
Get all available tool integrations.

```http
GET /api/v1/tools
```

**Response:**
```json
{
  "tools": [
    {
      "id": "github",
      "name": "GitHub",
      "description": "GitHub repository management",
      "version": "1.0",
      "_links": {
        "self": "/api/v1/tools/github",
        "actions": "/api/v1/tools/github/actions"
      }
    }
  ],
  "count": 5
}
```

### Get Tool Details
Get detailed information about a specific tool.

```http
GET /api/v1/tools/:tool
```

**Response:**
```json
{
  "id": "github",
  "name": "GitHub",
  "description": "GitHub integration for repository management",
  "version": "1.0",
  "vendor": "GitHub",
  "auth_methods": ["API Key", "OAuth"],
  "capabilities": ["issues", "pull_requests", "webhooks"],
  "_links": {
    "self": "/api/v1/tools/github",
    "actions": "/api/v1/tools/github/actions"
  }
}
```

### List Tool Actions
Get available actions for a tool.

```http
GET /api/v1/tools/:tool/actions
```

**Response:**
```json
{
  "tool": "github",
  "actions": [
    {
      "name": "create_issue",
      "display_name": "Create Issue",
      "description": "Create a new GitHub issue",
      "_links": {
        "self": "/api/v1/tools/github/actions/create_issue"
      }
    }
  ],
  "count": 12
}
```

### Execute Tool Action
Execute a specific action on a tool.

```http
POST /api/v1/tools/:tool/actions/:action
```

**Request Body:**
```json
{
  "repository": "owner/repo",
  "title": "Bug: Login fails with special characters",
  "body": "Users cannot login when password contains...",
  "labels": ["bug", "high-priority"]
}
```

**Response:**
```json
{
  "status": "success",
  "result": {
    "issue_number": 42,
    "html_url": "https://github.com/owner/repo/issues/42"
  },
  "_links": {
    "self": "/api/v1/tools/github/actions/create_issue",
    "result": "https://github.com/owner/repo/issues/42"
  }
}
```

### Query Tool Data
Execute queries against tool data.

```http
POST /api/v1/tools/:tool/queries
```

**Request Body:**
```json
{
  "query_type": "search_issues",
  "parameters": {
    "repository": "owner/repo",
    "state": "open",
    "labels": ["bug"]
  }
}
```

## Agent Management API

### Create Agent
Register a new AI agent.

```http
POST /api/v1/agents
```

**Request Body:**
```json
{
  "name": "DevOps Assistant",
  "type": "assistant",
  "capabilities": ["github", "kubernetes", "aws"],
  "metadata": {
    "model": "gpt-4",
    "version": "1.0"
  }
}
```

### List Agents
Get all registered agents.

```http
GET /api/v1/agents
```

**Response:**
```json
{
  "agents": [
    {
      "id": "agent-123",
      "name": "DevOps Assistant",
      "type": "assistant",
      "created_at": "2024-01-20T10:30:00Z"
    }
  ]
}
```

### Update Agent
Update agent metadata.

```http
PUT /api/v1/agents/:id
```

## Model Management API

### Create Model
Register a new embedding model.

```http
POST /api/v1/models
```

**Request Body:**
```json
{
  "name": "text-embedding-3-small",
  "provider": "openai",
  "dimensions": 1536,
  "capabilities": ["text", "code"]
}
```

### List Models
Get all registered models.

```http
GET /api/v1/models
```

## Vector Operations API

### Store Embedding
Store a vector embedding.

```http
POST /api/v1/vectors/store
```

**Request Body:**
```json
{
  "context_id": "ctx-123",
  "text": "OAuth 2.0 is an authorization framework",
  "embedding": [0.1, 0.2, 0.3, ...],
  "model_id": "text-embedding-3-small",
  "metadata": {
    "source": "documentation",
    "chunk_index": 1
  }
}
```

### Search Embeddings
Search for similar embeddings.

```http
POST /api/v1/vectors/search
```

**Request Body:**
```json
{
  "query": "How does OAuth work?",
  "context_ids": ["ctx-123", "ctx-456"],
  "limit": 10,
  "similarity_threshold": 0.75,
  "filters": {
    "source": "documentation"
  }
}
```

### Get Supported Models
List models available for embeddings.

```http
GET /api/v1/vectors/models
```

**Response:**
```json
{
  "models": [
    {
      "id": "text-embedding-3-small",
      "name": "OpenAI Text Embedding Small",
      "dimensions": 1536,
      "max_tokens": 8191
    }
  ]
}
```

## Search API

### Text Search
Search using natural language query.

```http
POST /api/v1/search/query
```

**Request Body:**
```json
{
  "query": "How to implement OAuth 2.0?",
  "contexts": ["ctx-123"],
  "limit": 10,
  "search_type": "hybrid"
}
```

### Vector Search
Search using pre-computed vector.

```http
POST /api/v1/search/vector
```

**Request Body:**
```json
{
  "vector": [0.1, 0.2, 0.3, ...],
  "model_id": "text-embedding-3-small",
  "limit": 10
}
```

### Find Similar Content
Find content similar to a reference.

```http
POST /api/v1/search/similar
```

**Request Body:**
```json
{
  "reference_id": "emb-123",
  "limit": 5
}
```

## MCP Protocol API

The MCP protocol endpoints follow the Model Context Protocol specification.

### Create MCP Context
```http
POST /api/v1/mcp/context
```

### Get MCP Context
```http
GET /api/v1/mcp/context/:id
```

### Update MCP Context
```http
PUT /api/v1/mcp/context/:id
```

### List MCP Contexts
```http
GET /api/v1/mcp/contexts
```

### Search MCP Context
```http
POST /api/v1/mcp/context/:id/search
```

## Webhook API

### GitHub Webhook
Receive GitHub webhook events.

```http
POST /api/webhooks/github
```

**Headers:**
```http
X-GitHub-Event: issues
X-Hub-Signature-256: sha256=...
```

## Relationship API

### Create Relationship
Create a relationship between entities.

```http
POST /api/v1/relationships
```

**Request Body:**
```json
{
  "source": {
    "type": "issue",
    "id": "issue-123"
  },
  "target": {
    "type": "pull_request",
    "id": "pr-456"
  },
  "relationship_type": "fixes"
}
```

### Get Entity Relationships
Get all relationships for an entity.

```http
GET /api/v1/entities/:type/:owner/:repo/:id/relationships
```

### Get Relationship Graph
Get the relationship graph for an entity.

```http
GET /api/v1/entities/:type/:owner/:repo/:id/graph?depth=2
```

## Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "Context ctx-999 not found",
    "details": {
      "resource_type": "context",
      "resource_id": "ctx-999"
    }
  },
  "request_id": "req-abc123",
  "timestamp": "2024-01-20T10:30:00Z"
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid authentication |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `RESOURCE_NOT_FOUND` | 404 | Resource does not exist |
| `VALIDATION_ERROR` | 400 | Invalid request data |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Server error |

## SDK Examples

### JavaScript/TypeScript
```typescript
import { DevOpsMCPClient } from '@devops-mcp/rest-client';

const client = new DevOpsMCPClient({
  apiKey: 'your-api-key',
  baseURL: 'https://api.devops-mcp.com/api/v1'
});

// Create context
const context = await client.contexts.create({
  agent_id: 'agent-123',
  content: [{ type: 'message', role: 'user', content: 'Hello' }]
});

// Search
const results = await client.search.query({
  query: 'OAuth 2.0',
  contexts: [context.id]
});
```

### Python
```python
from devops_mcp import RestClient

client = RestClient(
    api_key="your-api-key",
    base_url="https://api.devops-mcp.com/api/v1"
)

# Create context
context = client.contexts.create(
    agent_id="agent-123",
    content=[{"type": "message", "role": "user", "content": "Hello"}]
)

# Execute tool action
result = client.tools.execute_action(
    tool="github",
    action="create_issue",
    params={
        "repository": "owner/repo",
        "title": "Bug report",
        "body": "Description"
    }
)
```

### Go
```go
import "github.com/S-Corkum/devops-mcp/pkg/client/rest"

client := rest.NewClient(
    rest.WithAPIKey("your-api-key"),
    rest.WithBaseURL("https://api.devops-mcp.com/api/v1"),
)

// Create context
ctx, err := client.Contexts.Create(context.Background(), &CreateContextRequest{
    AgentID: "agent-123",
    Content: []ContextItem{{Type: "message", Role: "user", Content: "Hello"}},
})

// Search
results, err := client.Search.Query(context.Background(), &SearchRequest{
    Query: "OAuth 2.0",
    ContextIDs: []string{ctx.ID},
})
```

---

*For more information, visit [docs.devops-mcp.com](https://docs.devops-mcp.com)*
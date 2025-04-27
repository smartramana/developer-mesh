# MCP Server API Reference

This document provides a comprehensive reference for the MCP Server API. The API is organized into four main sections:

1. **Context API** - Manage conversation contexts and their content
2. **Tools API** - Integrate with DevOps tools and execute actions
3. **Vector API** - Store and search vector embeddings
4. **Metrics API** - Expose Prometheus-compatible metrics

## Base URL

All API endpoints are relative to the base URL:

```
/api/v1
```

## Authentication

The API supports two authentication methods for most endpoints:

1. **Bearer Authentication** - Using JWT tokens
   ```
   Authorization: Bearer <token>
   ```

2. **API Key Authentication** - Using an API key in the header
   ```
   X-API-Key: <api_key>
   ```

**Note:** The `/metrics` endpoint is public and does **not** require authentication for GET requests. All other API endpoints require authentication as described above.

## Context API

The Context API allows you to manage conversation contexts for AI agents.

### Endpoints

| Method | Endpoint                        | Description                                |
|--------|----------------------------------|--------------------------------------------|
| GET    | `/contexts`                     | List contexts for an agent                 |
| POST   | `/contexts`                     | Create a new context                       |
| GET    | `/contexts/{contextID}`         | Get a context by ID                        |
| PUT    | `/contexts/{contextID}`         | Update a context                           |
| DELETE | `/contexts/{contextID}`         | Delete a context                           |
| GET    | `/contexts/{contextID}/summary` | Get a summary of a context                 |
| POST   | `/contexts/{contextID}/search`  | Search within a context                    |

### List Contexts

```http
GET /api/v1/contexts?agent_id={agentID}&session_id={sessionID}&limit={limit}
```

Query Parameters:
- `agent_id` (required) - The agent ID
- `session_id` (optional) - Session ID filter
- `limit` (optional) - Maximum number of contexts to return (default: 20)

Response:
```json
{
  "contexts": [
    {
      "id": "ctx_123",
      "agent_id": "agent_456",
      "model_id": "gpt-4",
      "session_id": "session_789",
      "current_tokens": 150,
      "max_tokens": 2000,
      "created_at": "2025-04-22T12:00:00Z",
      "updated_at": "2025-04-22T12:30:00Z"
    }
  ],
  "_links": {
    "self": "/api/v1/contexts?agent_id=agent_456"
  }
}
```

### Create Context

```http
POST /api/v1/contexts
```

Request Body:
```json
{
  "agent_id": "agent_456",
  "model_id": "gpt-4",
  "session_id": "session_789",
  "max_tokens": 2000,
  "content": []
}
```

Response:
```json
{
  "message": "context created",
  "id": "ctx_123"
}
```

### Get Context

```http
GET /api/v1/contexts/{contextID}?include_content={boolean}
```

Query Parameters:
- `include_content` (optional) - Whether to include content items (default: true)

Response:
```json
{
  "id": "ctx_123",
  "message": "context retrieved"
}
```

### Update Context

```http
PUT /api/v1/contexts/{contextID}
```

Request Body:
```json
{
  "content": [
    {
      "role": "user",
      "content": "Hello AI assistant!"
    }
  ],
  "options": {
    "truncate": false,
    "replace_content": true
  }
}
```

Response: Returns the updated context object.

### Delete Context

```http
DELETE /api/v1/contexts/{contextID}
```

Response:
```json
{
  "status": "deleted"
}
```

### Get Context Summary

```http
GET /api/v1/contexts/{contextID}/summary
```

Response:
```json
{
  "summary": "Conversation about AI capabilities and limitations."
}
```

### Search Context

```http
POST /api/v1/contexts/{contextID}/search
```

Request Body:
```json
{
  "query": "machine learning"
}
```

Response:
```json
{
  "results": [
    {
      "id": "item_123",
      "role": "assistant",
      "content": "Machine learning is a subset of artificial intelligence.",
      "tokens": 12,
      "timestamp": "2025-04-22T12:05:00Z"
    }
  ]
}
```

## Tools API

The Tools API allows you to integrate with various DevOps tools and execute actions.

### Endpoints

| Method | Endpoint                            | Description                              |
|--------|-------------------------------------|------------------------------------------|
| GET    | `/tools`                           | List all available tools                 |
| GET    | `/tools/{tool}`                    | Get tool details                         |
| GET    | `/tools/{tool}/actions`            | List allowed actions for a tool          |
| GET    | `/tools/{tool}/actions/{action}`   | Get action details                       |
| POST   | `/tools/{tool}/actions/{action}`   | Execute tool action                      |
| POST   | `/tools/{tool}/queries`            | Query tool data                          |

### List Tools

```http
GET /api/v1/tools
```

Response:
```json
{
  "tools": [
    {
      "name": "github",
      "description": "GitHub integration for repository, PR, and code management",
      "actions": ["create_issue", "close_issue", "create_pull_request"],
      "safety_notes": "Cannot delete repositories for safety reasons"
    }
  ],
  "_links": {
    "self": "/api/v1/tools"
  }
}
```

### Get Tool Details

```http
GET /api/v1/tools/{tool}
```

Response:
```json
{
  "name": "github",
  "description": "GitHub integration for repository, PR, and code management",
  "actions": ["create_issue", "close_issue", "create_pull_request"],
  "safety_notes": "Cannot delete repositories for safety reasons",
  "_links": {
    "self": "/api/v1/tools/github",
    "actions": "/api/v1/tools/github/actions"
  }
}
```

### List Tool Actions

```http
GET /api/v1/tools/{tool}/actions
```

Response:
```json
{
  "tool": "github",
  "allowed_actions": [
    "create_issue",
    "close_issue",
    "create_pull_request"
  ],
  "disallowed_actions": [
    "delete_repository",
    "delete_branch"
  ],
  "safety_notes": "Repository deletion is restricted for safety reasons"
}
```

### Get Action Details

```http
GET /api/v1/tools/{tool}/actions/{action}
```

Response:
```json
{
  "name": "create_issue",
  "description": "Creates a new issue in a GitHub repository",
  "parameters": {
    "owner": "Repository owner (organization or user)",
    "repo": "Repository name",
    "title": "Issue title",
    "body": "Issue description"
  },
  "required_parameters": ["owner", "repo", "title"],
  "_links": {
    "self": "/api/v1/tools/github/actions/create_issue",
    "tool": "/api/v1/tools/github"
  }
}
```

### Execute Tool Action

```http
POST /api/v1/tools/{tool}/actions/{action}?context_id={contextID}
```

Query Parameters:
- `context_id` (required) - Context ID for tracking the operation

Request Body:
```json
{
  "owner": "octocat",
  "repo": "hello-world",
  "title": "Bug in login form",
  "body": "The login form doesn't submit when using Safari",
  "labels": ["bug", "frontend"]
}
```

Response:
```json
{
  "status": "success",
  "message": "Executed create_issue action on github tool",
  "tool": "github",
  "action": "create_issue",
  "params": {
    "owner": "octocat",
    "repo": "hello-world",
    "title": "Bug in login form"
  },
  "_links": {
    "self": "/api/v1/tools/github/actions/create_issue",
    "tool": "/api/v1/tools/github"
  }
}
```

### Query Tool Data

```http
POST /api/v1/tools/{tool}/queries?context_id={contextID}
```

Query Parameters:
- `context_id` (required) - Context ID for tracking the operation

Request Body:
```json
{
  "repo": "octocat/hello-world",
  "state": "open",
  "sort": "created",
  "direction": "desc"
}
```

Response:
```json
{
  "status": "success",
  "message": "Queried data from github tool",
  "tool": "github",
  "query_params": {
    "repo": "octocat/hello-world",
    "state": "open"
  },
  "data": [
    {
      "id": "1",
      "name": "Example data item 1"
    },
    {
      "id": "2",
      "name": "Example data item 2"
    }
  ]
}
```

## Vector API

The Vector API allows you to store and search vector embeddings for AI models.

### Endpoints

| Method | Endpoint                                       | Description                                 |
|--------|------------------------------------------------|---------------------------------------------|
| POST   | `/vectors/store`                              | Store an embedding                          |
| POST   | `/vectors/search`                             | Search embeddings                           |
| GET    | `/vectors/context/{context_id}`               | Get all embeddings for a context            |
| DELETE | `/vectors/context/{context_id}`               | Delete all embeddings for a context         |
| GET    | `/vectors/models`                             | Get supported models                        |
| GET    | `/vectors/context/{context_id}/model/{model_id}` | Get embeddings for a specific model      |
| DELETE | `/vectors/context/{context_id}/model/{model_id}` | Delete embeddings for a specific model   |

### Store Embedding

```http
POST /api/v1/vectors/store
```

Request Body:
```json
{
  "context_id": "ctx_123",
  "content_index": 0,
  "text": "Hello AI assistant!",
  "embedding": [0.1, 0.2, 0.3],
  "model_id": "text-embedding-ada-002"
}
```

Response: Returns the stored embedding object.

### Search Embeddings

```http
POST /api/v1/vectors/search
```

Request Body:
```json
{
  "context_id": "ctx_123",
  "query_embedding": [0.1, 0.2, 0.3],
  "limit": 5,
  "model_id": "text-embedding-ada-002",
  "similarity_threshold": 0.7
}
```

Response:
```json
{
  "embeddings": [
    {
      "context_id": "ctx_123",
      "content_index": 0,
      "text": "Hello AI assistant!",
      "embedding": [0.1, 0.2, 0.3],
      "model_id": "text-embedding-ada-002",
      "created_at": "2025-04-22T12:00:00Z"
    }
  ]
}
```

### Get Context Embeddings

```http
GET /api/v1/vectors/context/{context_id}
```

Response:
```json
{
  "embeddings": [
    {
      "context_id": "ctx_123",
      "content_index": 0,
      "text": "Hello AI assistant!",
      "embedding": [0.1, 0.2, 0.3],
      "model_id": "text-embedding-ada-002",
      "created_at": "2025-04-22T12:00:00Z"
    }
  ]
}
```

### Delete Context Embeddings

```http
DELETE /api/v1/vectors/context/{context_id}
```

Response:
```json
{
  "status": "deleted"
}
```

### Get Supported Models

```http
GET /api/v1/vectors/models
```

Response:
```json
{
  "models": [
    "text-embedding-ada-002",
    "text-embedding-3-small"
  ]
}
```

### Get Model Embeddings

```http
GET /api/v1/vectors/context/{context_id}/model/{model_id}
```

Response:
```json
{
  "embeddings": [
    {
      "context_id": "ctx_123",
      "content_index": 0,
      "text": "Hello AI assistant!",
      "embedding": [0.1, 0.2, 0.3],
      "model_id": "text-embedding-ada-002",
      "created_at": "2025-04-22T12:00:00Z"
    }
  ]
}
```

### Delete Model Embeddings

```http
DELETE /api/v1/vectors/context/{context_id}/model/{model_id}
```

Response:
```json
{
  "status": "deleted"
}
```

## Error Handling

All API endpoints follow a consistent error response format:

```json
{
  "error": "Error message"
}
```

Common HTTP status codes:

| Status Code | Description                                            |
|-------------|--------------------------------------------------------|
| 200         | Request succeeded                                     |
| 201         | Resource created successfully                         |
| 400         | Bad request (invalid parameters or request body)      |
| 401         | Unauthorized (authentication required)                |
| 404         | Resource not found                                    |
| 500         | Internal server error                                 |

## Metrics API

### GET /metrics

Exposes Prometheus-compatible metrics for monitoring the MCP server.

- **Endpoint:** `/metrics`
- **Method:** `GET`
- **Authentication:** _Not required_
- **Description:** Returns metrics in a format suitable for Prometheus scraping. This endpoint is public and does not require authentication.

Example response:
```
mcp_server_requests_total{method="GET",endpoint="/api/v1/contexts"} 42
mcp_server_requests_total{method="POST",endpoint="/api/v1/contexts"} 17
```

## Pagination

For endpoints that return multiple items, pagination is supported through the following query parameters:

- `limit` - Maximum number of items to return (default: 20, max: 100)
- `offset` - Number of items to skip (default: 0)

## HATEOAS Links

API responses include hypermedia links (_links) that provide URLs to related resources and actions. This follows the HATEOAS (Hypermedia as the Engine of Application State) pattern, allowing clients to discover available actions dynamically.

Example:
```json
"_links": {
  "self": "/api/v1/contexts/ctx_123",
  "update": "/api/v1/contexts/ctx_123",
  "delete": "/api/v1/contexts/ctx_123"
}
```
# MCP Server API Reference

This document provides a comprehensive reference for the MCP Server API endpoints, including request and response formats, authentication requirements, and examples.

## API Overview

The MCP Server provides a REST API for interacting with the platform and integrated systems. All API endpoints are prefixed with `/api/v1`.

### Authentication

The API supports two authentication methods:

1. **JWT Authentication** (preferred for user interactions)
2. **API Key Authentication** (preferred for system integrations)

#### JWT Authentication

To authenticate with JWT:

1. Obtain a JWT token by calling the authentication endpoint
2. Include the token in all API requests in the `Authorization` header:

```
Authorization: Bearer your-jwt-token
```

#### API Key Authentication

To authenticate with an API key:

1. Configure API keys in the server configuration
2. Include the API key in all API requests in the `Authorization` header:

```
Authorization: ApiKey your-api-key
```

### Response Format

API responses follow a consistent JSON format:

```json
{
  "status": "success",           // "success" or "error"
  "data": { ... },               // Response data (if successful)
  "error": "Error message",      // Error message (if failed)
  "code": "ERROR_CODE"           // Error code (if failed)
}
```

### Rate Limiting

API endpoints are rate-limited based on the server configuration. When a rate limit is exceeded, the server returns a `429 Too Many Requests` response with headers indicating the limit and reset time:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1619712000
```

## API Endpoints

### Health and Metrics Endpoints

#### Check API Health

```
GET /health
```

Returns the health status of the API server and all components.

**Example Response:**

```json
{
  "status": "healthy",
  "components": {
    "engine": "healthy",
    "github": "healthy"
  }
}
```

#### Get Metrics

```
GET /metrics
```

Returns metrics data in Prometheus format. Requires API key authentication.

### Webhook Endpoints

The following webhook endpoints are available for receiving events from external systems:

#### GitHub Webhook

```
POST /webhook/github
```

Processes webhooks from GitHub. The webhook must include the appropriate headers and a valid signature.

**Required Headers:**
- `X-GitHub-Event`: Type of GitHub event
- `X-Hub-Signature-256`: HMAC SHA-256 signature for payload verification

> **Note:** Support for Harness, SonarQube, Artifactory, and JFrog Xray webhooks has been removed.

### MCP Protocol Endpoints

#### Create MCP Context

```
POST /api/v1/mcp/context
```

Creates a new MCP context for tracking related events and operations.

**Request Body:**

```json
{
  "name": "my-context",
  "description": "My MCP context",
  "tags": ["tag1", "tag2"],
  "metadata": {
    "key1": "value1",
    "key2": "value2"
  }
}
```

**Response:**

```json
{
  "status": "success",
  "data": {
    "id": "ctx-123456",
    "name": "my-context",
    "description": "My MCP context",
    "created_at": "2023-04-29T12:34:56Z",
    "tags": ["tag1", "tag2"],
    "metadata": {
      "key1": "value1",
      "key2": "value2"
    }
  }
}
```

#### Get MCP Context

```
GET /api/v1/mcp/context/:id
```

Retrieves an MCP context by ID.

**Parameters:**
- `id`: ID of the MCP context

**Response:**

```json
{
  "status": "success",
  "data": {
    "id": "ctx-123456",
    "name": "my-context",
    "description": "My MCP context",
    "created_at": "2023-04-29T12:34:56Z",
    "updated_at": "2023-04-29T12:34:56Z",
    "tags": ["tag1", "tag2"],
    "metadata": {
      "key1": "value1",
      "key2": "value2"
    },
    "events": [
      {
        "id": "evt-123456",
        "source": "github",
        "type": "pull_request",
        "timestamp": "2023-04-29T12:35:00Z"
      }
    ]
  }
}
```

#### Update MCP Context

```
PUT /api/v1/mcp/context/:id
```

Updates an MCP context.

**Parameters:**
- `id`: ID of the MCP context

**Request Body:**

```json
{
  "name": "updated-context",
  "description": "Updated MCP context",
  "tags": ["tag1", "tag3"],
  "metadata": {
    "key1": "new-value",
    "key3": "value3"
  }
}
```

**Response:**

```json
{
  "status": "success",
  "data": {
    "id": "ctx-123456",
    "name": "updated-context",
    "description": "Updated MCP context",
    "created_at": "2023-04-29T12:34:56Z",
    "updated_at": "2023-04-29T12:36:00Z",
    "tags": ["tag1", "tag3"],
    "metadata": {
      "key1": "new-value",
      "key3": "value3"
    }
  }
}
```

#### Delete MCP Context

```
DELETE /api/v1/mcp/context/:id
```

Deletes an MCP context.

**Parameters:**
- `id`: ID of the MCP context

**Response:**

```json
{
  "status": "success",
  "data": {
    "message": "Context deleted successfully"
  }
}
```

### GitHub Integration Endpoints

#### List GitHub Repositories

```
GET /api/v1/github/repos
```

Lists GitHub repositories accessible to the configured GitHub token.

**Query Parameters:**
- `owner`: Filter by repository owner (optional)
- `type`: Repository type (public, private, all) (optional)
- `sort`: Sort field (created, updated, pushed, full_name) (optional)
- `direction`: Sort direction (asc, desc) (optional)
- `page`: Page number (optional)
- `per_page`: Items per page (optional)

**Response:**

```json
{
  "status": "success",
  "data": {
    "repositories": [
      {
        "id": 12345678,
        "name": "repo-name",
        "full_name": "owner/repo-name",
        "html_url": "https://github.com/owner/repo-name",
        "description": "Repository description",
        "private": false,
        "owner": {
          "login": "owner",
          "id": 1234567,
          "avatar_url": "https://avatars.githubusercontent.com/u/1234567"
        },
        "created_at": "2023-01-01T00:00:00Z",
        "updated_at": "2023-04-01T00:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "per_page": 30,
      "total": 45
    }
  }
}
```

#### Get GitHub Repository

```
GET /api/v1/github/repos/:owner/:repo
```

Retrieves information about a specific GitHub repository.

**Parameters:**
- `owner`: Repository owner
- `repo`: Repository name

**Response:**

```json
{
  "status": "success",
  "data": {
    "id": 12345678,
    "name": "repo-name",
    "full_name": "owner/repo-name",
    "html_url": "https://github.com/owner/repo-name",
    "description": "Repository description",
    "private": false,
    "owner": {
      "login": "owner",
      "id": 1234567,
      "avatar_url": "https://avatars.githubusercontent.com/u/1234567"
    },
    "default_branch": "main",
    "created_at": "2023-01-01T00:00:00Z",
    "updated_at": "2023-04-01T00:00:00Z",
    "pushed_at": "2023-04-15T00:00:00Z",
    "size": 1024,
    "language": "Go",
    "license": {
      "key": "mit",
      "name": "MIT License",
      "spdx_id": "MIT"
    }
  }
}
```

#### List GitHub Pull Requests

```
GET /api/v1/github/repos/:owner/:repo/pulls
```

Lists pull requests for a specific GitHub repository.

**Parameters:**
- `owner`: Repository owner
- `repo`: Repository name

**Query Parameters:**
- `state`: Pull request state (open, closed, all) (optional)
- `sort`: Sort field (created, updated, popularity, long-running) (optional)
- `direction`: Sort direction (asc, desc) (optional)
- `page`: Page number (optional)
- `per_page`: Items per page (optional)

**Response:**

```json
{
  "status": "success",
  "data": {
    "pull_requests": [
      {
        "id": 1234567890,
        "number": 123,
        "title": "Pull request title",
        "state": "open",
        "html_url": "https://github.com/owner/repo-name/pull/123",
        "user": {
          "login": "username",
          "id": 1234567,
          "avatar_url": "https://avatars.githubusercontent.com/u/1234567"
        },
        "created_at": "2023-04-01T00:00:00Z",
        "updated_at": "2023-04-15T00:00:00Z",
        "closed_at": null,
        "merged_at": null,
        "base": {
          "ref": "main",
          "label": "owner:main"
        },
        "head": {
          "ref": "feature-branch",
          "label": "contributor:feature-branch"
        }
      }
    ],
    "pagination": {
      "page": 1,
      "per_page": 30,
      "total": 45
    }
  }
}
```

> **Note:** Integration endpoints for Harness, SonarQube, Artifactory, and JFrog Xray have been removed.

## Error Codes

The API uses the following error codes:

| Code | Description |
|------|-------------|
| `AUTHENTICATION_ERROR` | Authentication failed |
| `AUTHORIZATION_ERROR` | User is not authorized to perform the action |
| `VALIDATION_ERROR` | Request validation failed |
| `RESOURCE_NOT_FOUND` | Requested resource not found |
| `RATE_LIMIT_EXCEEDED` | Rate limit exceeded |
| `INTERNAL_ERROR` | Internal server error |
| `SERVICE_UNAVAILABLE` | External service is unavailable |
| `ADAPTER_ERROR` | Error in adapter communication |
| `WEBHOOK_VALIDATION_ERROR` | Webhook signature validation failed |

## API Versioning

The MCP Server API is versioned in the URL path. The current version is `v1`:

```
/api/v1/...
```

Future versions will use different version prefixes:

```
/api/v2/...
```

## API Pagination

All list endpoints support pagination using the following query parameters:

- `page`: Page number (default: 1)
- `per_page`: Items per page (default: 30, max: 100)

Pagination information is included in the response:

```json
"pagination": {
  "page": 1,
  "per_page": 30,
  "total": 45
}
```

## Cross-Origin Resource Sharing (CORS)

The API supports CORS for JavaScript clients. The CORS configuration can be customized in the server configuration.
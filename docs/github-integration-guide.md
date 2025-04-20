# GitHub Integration Guide

This document provides details about the enhanced GitHub integration in the MCP Server project. It covers the new features, implementation details, and usage examples.

## New Features

The updated GitHub integration provides the following major enhancements:

### Expanded API Coverage
- Comprehensive REST API support for most GitHub operations
- Advanced GraphQL API with a query builder
- Multiple authentication methods (PAT, GitHub App, OAuth)
- Proper pagination handling for all API responses

### Robust Webhook System
- Flexible webhook handler registration
- Structured event parsing for all GitHub event types
- Webhook validation with JSON schema support
- Reliable webhook delivery retry mechanism
- Event filtering based on repositories, branches, and actions

### Performance & Reliability
- Smart rate limiting with backoff strategies
- Enhanced error handling and recovery
- Schema validation for webhook payloads
- Comprehensive metrics and logging

## Architecture

The GitHub integration is composed of several components:

### Core Components

1. **GitHubAdapter**: The main entry point that provides a unified interface for all GitHub operations.
2. **RESTClient**: Handles REST API operations with proper pagination support.
3. **GraphQLClient**: Provides access to GitHub's GraphQL API.
4. **GraphQLBuilder**: Helper for building complex GraphQL queries.
5. **AuthProvider**: Handles authentication with GitHub using different methods.
6. **WebhookManager**: Manages webhook event handlers with filtering capabilities.
7. **WebhookValidator**: Validates webhook payloads against JSON schemas.
8. **WebhookRetryManager**: Handles retrying failed webhook deliveries.

### Authentication

The system supports multiple authentication methods:

- **Personal Access Token (PAT)**: Simple token-based authentication
- **GitHub App**: JWT-based authentication for GitHub Apps
- **OAuth**: Token-based authentication with refresh capabilities

### Webhook Handling

The webhook system provides:

1. **Event Parsing**: Extracting structured data from webhook payloads
2. **Handler Registration**: Register callbacks for specific event types
3. **Event Filtering**: Filter events by repository, branch, or action
4. **Retry Mechanism**: Retry failed webhook deliveries with exponential backoff
5. **Schema Validation**: Validate webhook payloads against JSON schemas

## Usage Examples

### Basic Operations

#### Getting a Repository

```go
params := map[string]interface{}{
    "owner": "octocat",
    "repo":  "hello-world",
}
result, err := adapter.ExecuteAction(ctx, "context-123", "getRepository", params)
```

#### Creating an Issue

```go
params := map[string]interface{}{
    "owner": "octocat",
    "repo":  "hello-world",
    "title": "Found a bug",
    "body":  "I'm having a problem with this.",
    "labels": []string{"bug", "help wanted"},
}
result, err := adapter.ExecuteAction(ctx, "context-123", "createIssue", params)
```

#### Listing Pull Requests

```go
params := map[string]interface{}{
    "owner": "octocat",
    "repo":  "hello-world",
    "state": "open",
}
result, err := adapter.ExecuteAction(ctx, "context-123", "listPullRequests", params)
```

### GraphQL Operations

#### Using the GraphQL Builder

```go
params := map[string]interface{}{
    "operation_type": "query",
    "operation_name": "GetUserInfo",
    "variable_types": map[string]interface{}{
        "login": "String!",
    },
    "variables": map[string]interface{}{
        "login": "octocat",
    },
    "fields": []interface{}{
        "user(login: $login) { id, login, name, bio }",
    },
}
result, err := adapter.ExecuteAction(ctx, "context-123", "buildAndExecuteGraphQL", params)
```

### Webhook Handling

#### Registering a Webhook Handler

```go
params := map[string]interface{}{
    "handler_id":   "my-issue-handler",
    "event_types":  []interface{}{"issues"},
    "repositories": []interface{}{"octocat/hello-world"},
    "actions":      []interface{}{"opened", "edited"},
}
result, err := adapter.ExecuteAction(ctx, "context-123", "registerWebhookHandler", params)
```

## Configuration Options

The GitHub integration can be configured with the following options:

### Authentication

- `token`: Personal Access Token (PAT)
- `app_id`, `app_private_key`, `app_installation_id`: GitHub App credentials
- `oauth_token`, `oauth_client_id`, `oauth_client_secret`: OAuth credentials

### Connection

- `base_url`, `upload_url`, `graphql_url`: API endpoints (defaults to GitHub.com)
- `request_timeout`: Timeout for API requests
- Various connection pool settings (`max_idle_conns`, etc.)

### Rate Limiting

- `rate_limit`: Enable/disable rate limiting
- `rate_limit_per_hour`: Maximum requests per hour
- `rate_limit_backoff_factor`: Backoff factor for rate limit retries

### Webhook Settings

- `webhook_secret`: Secret for webhook validation
- `webhook_workers`: Number of concurrent webhook handlers
- `webhook_queue_size`: Size of the webhook processing queue
- `webhook_max_retries`: Maximum number of retries for failed webhooks
- `webhook_validate_payload`: Enable/disable JSON schema validation

## Error Handling

The system provides specific error types for common scenarios:

- `ErrInvalidSignature`: Invalid webhook signature
- `ErrReplayAttack`: Webhook replay attack detected
- `ErrRateLimitExceeded`: GitHub API rate limit exceeded
- `ErrUnauthorized`: Unauthorized GitHub API request
- `ErrForbidden`: Forbidden GitHub API request
- `ErrNotFound`: GitHub resource not found
- `ErrOperationNotSupported`: Operation not supported
- `ErrInvalidParameters`: Invalid parameters
- `ErrWebhookDisabled`: Webhooks are disabled

## Best Practices

1. **Rate Limit Awareness**: Be mindful of GitHub's rate limits (5000 requests/hour for authenticated requests)
2. **Pagination**: Use pagination for listing operations to avoid memory issues with large datasets
3. **GraphQL for Complex Queries**: Use GraphQL for complex data requirements to reduce the number of API calls
4. **Webhook Security**: Always set a webhook secret and validate all incoming webhook requests
5. **Error Handling**: Implement proper error handling, especially for rate limits and retries
6. **Caching**: Implement caching for frequently accessed data to reduce API calls
7. **Authentication**: Use GitHub Apps for higher rate limits and better security

## Future Improvements

Potential areas for future enhancement:

1. Distributed webhook delivery tracking for high availability
2. Caching layer for API responses
3. Enhanced metrics and observability
4. Support for GitHub Enterprise Server
5. Background jobs for long-running operations
6. Intelligent rate limit management across multiple tokens/apps

## Troubleshooting

Common issues and solutions:

- **Rate Limit Exceeded**: Check your rate limit usage and consider using a GitHub App or implementing caching
- **Invalid Webhook Signature**: Verify your webhook secret matches what's configured in GitHub
- **Webhook Delivery Issues**: Check webhook logs for delivery errors and implement retry logic
- **Authentication Failures**: Verify your token, app, or OAuth credentials are valid and have the required scopes

# MCP Server Integration Points

This document describes the integration points between the MCP Server and external systems, detailing how data flows between systems and how to configure and manage these integrations.

## Integration Architecture

The MCP Server uses a modular adapter-based architecture for integrations:

1. Each external system has a dedicated adapter implementation
2. Adapters implement a common interface defined in `internal/adapters/adapter.go`
3. The Core Engine manages adapter lifecycle and event routing
4. Webhook endpoints receive events from external systems
5. API endpoints allow querying and manipulating data in external systems

> **Note:** Support for Harness, SonarQube, Artifactory, and JFrog Xray integrations has been removed from this version. Only GitHub integration is currently supported.

## Common Integration Patterns

### Webhook Processing

Most integrations use webhooks for event notification:

1. External system sends a webhook to the MCP Server
2. MCP Server validates the webhook signature
3. Webhook payload is parsed and converted to an MCP event
4. Event is processed by the Core Engine
5. Actions may be triggered in response to the event

### API Polling

For systems without webhook support or for data that isn't pushed via webhooks:

1. MCP Server periodically polls the external API
2. Changes are detected by comparing with previous state
3. Detected changes are converted to MCP events
4. Events are processed by the Core Engine

### API Query Proxying

For client applications that need access to external system data:

1. Client makes a request to the MCP Server API
2. MCP Server validates and potentially transforms the request
3. Request is forwarded to the external system API
4. Response is cached, transformed, and returned to the client

## GitHub Integration

### Configuration

GitHub integration requires the following configuration:

```yaml
engine:
  github:
    api_token: "your-github-token"           # GitHub API token with appropriate permissions
    webhook_secret: "your-webhook-secret"    # Secret for verifying webhook signatures
    request_timeout: 10s                     # Timeout for API requests
    rate_limit_per_hour: 5000                # API rate limit per hour
    max_retries: 3                           # Maximum number of retries for API requests
    retry_delay: 1s                          # Initial delay between retries (increases exponentially)
    mock_responses: false                    # Use mock responses instead of real API
```

### Supported Events

The GitHub adapter supports the following webhook events:

- `pull_request`: Pull request opened, closed, edited, etc.
- `push`: Code pushed to a repository
- Additional events can be added in the GitHub adapter implementation

### API Capabilities

The GitHub adapter supports the following API operations:

- Query repository information
- List and filter pull requests
- Get issue information
- List and filter commits
- Additional operations can be added in the GitHub adapter implementation

### Webhook Setup

To configure GitHub webhooks:

1. Go to your GitHub repository settings
2. Navigate to "Webhooks" and click "Add webhook"
3. Set the Payload URL to `https://your-mcp-server/webhook/github`
4. Set the Content type to `application/json`
5. Enter your secret in the "Secret" field
6. Select the events you want to trigger the webhook
7. Click "Add webhook"



## Adding New Integrations

The MCP Server is designed to be extensible, allowing new integrations to be added easily.

### Integration Requirements

To add a new integration:

1. Create a new adapter package in `internal/adapters/yourservice`
2. Implement the Adapter interface
3. Add configuration options in the config package
4. Update the Core Engine to initialize the new adapter
5. Add webhook handling in the API Server if needed

See the [Adding New Integrations](adding-new-integrations.md) guide for detailed instructions.

## Testing Integrations

The MCP Server includes a mock server for testing integrations without connecting to real external services.

### Using the Mock Server

To use the mock server:

1. Build the mock server:
   ```bash
   make mockserver-build
   ```

2. Run the mock server:
   ```bash
   ./mockserver
   ```

3. Configure the MCP Server to use mock mode:
   ```yaml
   engine:
     github:
       mock_responses: true
       mock_url: "http://localhost:8081/mock-github"
     # Similarly for other adapters
   ```

### Manual Webhook Testing

To test webhooks manually:

1. Use a tool like `curl` to send webhook requests:
   ```bash
   curl -X POST -H "Content-Type: application/json" -H "X-GitHub-Event: push" -H "X-Hub-Signature-256: sha256=..." -d '{"event":"data"}' http://localhost:8080/webhook/github
   ```

2. Check the MCP Server logs for event processing
3. Verify that the appropriate actions were taken

### Integration Environment

For more comprehensive testing, set up a dedicated integration environment with:

1. MCP Server instance
2. Mock instances of external services
3. Test data and scenarios
4. Automated integration tests

## Troubleshooting Integrations

Common integration issues and solutions:

### Authentication Issues

- Verify API tokens and credentials
- Check for token expiration
- Ensure correct permissions are set
- Check for API rate limiting

### Webhook Issues

- Verify webhook URL is accessible
- Check webhook secret configuration
- Inspect webhook payload format
- Verify signature calculation

### Connection Issues

- Check network connectivity
- Verify SSL/TLS configuration
- Check for firewalls or proxy settings
- Verify API endpoint URLs

See the [Troubleshooting Guide](troubleshooting-guide.md) for more detailed information.
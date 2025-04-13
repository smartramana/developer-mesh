# MCP Server Integration Points

This document describes the integration points between the MCP Server and external systems, detailing how data flows between systems and how to configure and manage these integrations.

## Integration Architecture

The MCP Server uses a modular adapter-based architecture for integrations:

1. Each external system has a dedicated adapter implementation
2. Adapters implement a common interface defined in `internal/adapters/adapter.go`
3. The Core Engine manages adapter lifecycle and event routing
4. Webhook endpoints receive events from external systems
5. API endpoints allow querying and manipulating data in external systems

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

## Harness Integration

### Configuration

Harness integration requires the following configuration:

```yaml
engine:
  harness:
    api_token: "your-harness-token"           # Harness API token
    account_id: "your-harness-account-id"     # Harness account ID
    webhook_secret: "your-webhook-secret"     # Secret for verifying webhook signatures
    base_url: "https://app.harness.io/gateway" # Harness API base URL
    request_timeout: 10s                      # Timeout for API requests
    max_retries: 3                            # Maximum number of retries for API requests
    retry_delay: 1s                           # Initial delay between retries
    mock_responses: false                     # Use mock responses instead of real API
```

### Supported Events

The Harness adapter supports the following webhook events:

- `ci.build`: CI build started, completed, failed, etc.
- `cd.deployment`: CD deployment started, completed, failed, etc.
- `sto.experiment`: STO experiment started, completed, failed, etc.
- `ff.change`: Feature flag created, updated, toggled, etc.

### API Capabilities

The Harness adapter supports the following API operations:

- Query pipeline information
- Get build and deployment status
- Retrieve experiment results
- Manage feature flags
- Additional operations can be added in the Harness adapter implementation

### Webhook Setup

To configure Harness webhooks:

1. Log in to your Harness account
2. Navigate to the appropriate section (CI, CD, FF, etc.)
3. Find the webhook configuration section
4. Add a new webhook with the URL `https://your-mcp-server/webhook/harness`
5. Configure the webhook to include your secret
6. Select the events you want to trigger the webhook
7. Enable the webhook

## SonarQube Integration

### Configuration

SonarQube integration requires the following configuration:

```yaml
engine:
  sonarqube:
    base_url: "https://your-sonarqube-instance"  # SonarQube URL
    token: "your-sonarqube-token"              # SonarQube API token
    webhook_secret: "your-webhook-secret"      # Secret for verifying webhook signatures
    request_timeout: 10s                       # Timeout for API requests
    max_retries: 3                             # Maximum number of retries for API requests
    retry_delay: 1s                            # Initial delay between retries
    mock_responses: false                      # Use mock responses instead of real API
```

### Supported Events

The SonarQube adapter supports the following webhook events:

- `quality_gate`: Quality gate status changed
- `task_completed`: Analysis task completed

### API Capabilities

The SonarQube adapter supports the following API operations:

- Query project information
- Get quality gate details
- Retrieve code analysis results
- List issues and metrics
- Additional operations can be added in the SonarQube adapter implementation

### Webhook Setup

To configure SonarQube webhooks:

1. Log in to your SonarQube instance as an administrator
2. Go to Administration > Configuration > Webhooks
3. Click "Create"
4. Enter a name for the webhook
5. Set the URL to `https://your-mcp-server/webhook/sonarqube`
6. Enter your secret in the "Secret" field
7. Click "Create"

## JFrog Artifactory Integration

### Configuration

Artifactory integration requires the following configuration:

```yaml
engine:
  artifactory:
    base_url: "https://your-artifactory-instance"  # Artifactory URL
    username: "your-artifactory-username"        # Artifactory username
    password: "your-artifactory-password"        # Artifactory password
    api_key: "your-artifactory-api-key"          # Artifactory API key (alternative to username/password)
    webhook_secret: "your-webhook-secret"        # Secret for verifying webhook signatures
    request_timeout: 10s                         # Timeout for API requests
    max_retries: 3                               # Maximum number of retries for API requests
    retry_delay: 1s                              # Initial delay between retries
    mock_responses: false                        # Use mock responses instead of real API
```

### Supported Events

The Artifactory adapter supports the following webhook events:

- `artifact_created`: Artifact created or deployed
- `artifact_deleted`: Artifact deleted
- `artifact_property_changed`: Artifact property modified

### API Capabilities

The Artifactory adapter supports the following API operations:

- Query repository information
- Get artifact details
- Retrieve build information
- Check storage statistics
- Additional operations can be added in the Artifactory adapter implementation

### Webhook Setup

To configure Artifactory webhooks:

1. Log in to your Artifactory instance as an administrator
2. Go to Administration > Webhooks
3. Click "New Webhook"
4. Enter a name for the webhook
5. Set the URL to `https://your-mcp-server/webhook/artifactory`
6. Enter your secret in the "Secret" field
7. Select the events you want to trigger the webhook
8. Click "Create"

## JFrog Xray Integration

### Configuration

Xray integration requires the following configuration:

```yaml
engine:
  xray:
    base_url: "https://your-xray-instance"      # Xray URL
    username: "your-xray-username"            # Xray username
    password: "your-xray-password"            # Xray password
    api_key: "your-xray-api-key"              # Xray API key (alternative to username/password)
    webhook_secret: "your-webhook-secret"     # Secret for verifying webhook signatures
    request_timeout: 10s                      # Timeout for API requests
    max_retries: 3                            # Maximum number of retries for API requests
    retry_delay: 1s                           # Initial delay between retries
    mock_responses: false                     # Use mock responses instead of real API
```

### Supported Events

The Xray adapter supports the following webhook events:

- `security_violation`: Security vulnerability detected
- `license_violation`: License violation detected
- `scan_completed`: Scan completed

### API Capabilities

The Xray adapter supports the following API operations:

- Query vulnerability information
- Get license details
- Retrieve scan results
- Check security summary
- Additional operations can be added in the Xray adapter implementation

### Webhook Setup

To configure Xray webhooks:

1. Log in to your Xray instance as an administrator
2. Go to Administration > Webhooks
3. Click "New Webhook"
4. Enter a name for the webhook
5. Set the URL to `https://your-mcp-server/webhook/xray`
6. Enter your secret in the "Secret" field
7. Select the events you want to trigger the webhook
8. Click "Create"

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
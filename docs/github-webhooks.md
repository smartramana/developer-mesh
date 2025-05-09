# GitHub Webhook Receiver

This document describes how to configure and use the GitHub webhook receiver in the MCP server.

## Configuration

The GitHub webhook receiver is configured using environment variables:

| Environment Variable | Description | Default Value |
|---------------------|-------------|---------------|
| MCP_WEBHOOK_ENABLED | Enable webhook handling | false |
| MCP_GITHUB_WEBHOOK_SECRET | GitHub webhook secret | (none, required) |
| MCP_GITHUB_WEBHOOK_ENDPOINT | GitHub webhook endpoint path | /api/webhooks/github |
| MCP_GITHUB_IP_VALIDATION | Enable GitHub IP validation | true |
| MCP_GITHUB_ALLOWED_EVENTS | Comma-separated list of allowed events | push,pull_request,issues,issue_comment,release |

## Setting up a GitHub Webhook

To set up a GitHub webhook:

1. Go to your GitHub repository
2. Click on "Settings" > "Webhooks" > "Add webhook"
3. Set the Payload URL to your server's webhook endpoint (e.g., `https://your-server.com/api/webhooks/github`)
4. Set the Content type to `application/json`
5. Enter a secret key (must match the `MCP_GITHUB_WEBHOOK_SECRET` environment variable)
6. Select the events you want to receive (must match the allowed events in the configuration)
7. Ensure "Active" is checked
8. Click "Add webhook"

## Security Considerations

The webhook receiver implements the following security measures:

1. **HMAC Signature Verification**: Each webhook request is verified using the `X-Hub-Signature-256` header, which contains a HMAC-SHA256 hash of the request body using the webhook secret as the key.
2. **IP Validation**: Requests are validated to ensure they come from GitHub's IP ranges, which are fetched from the GitHub meta API.
3. **Event Type Validation**: Only configured event types are allowed to be processed.

## Handling Different Event Types

The current implementation logs received events and returns a 200 OK response. To handle specific event types differently, extend the `GitHubWebhookHandler` function in `internal/api/webhooks.go`.

Typical event types include:

- `push`: Triggered when commits are pushed to a repository branch or tag
- `pull_request`: Triggered when a pull request is opened, closed, reopened, etc.
- `issues`: Triggered when an issue is opened, closed, etc.
- `issue_comment`: Triggered when a comment is added to an issue or pull request
- `release`: Triggered when a release is published, edited, etc.

Refer to [GitHub's webhook documentation](https://docs.github.com/en/developers/webhooks-and-events/webhooks/webhook-events-and-payloads) for details on all available event types and their payloads.
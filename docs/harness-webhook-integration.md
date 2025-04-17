# Harness.io Webhook Integration

This document describes how to set up and use the Harness.io webhook integration with MCP.

## Configuration

### MCP Configuration

The Harness webhook integration is configured in the `config.yaml` file under the `api.webhooks.harness` section:

```yaml
api:
  webhooks:
    harness:
      enabled: ${HARNESS_WEBHOOK_ENABLED:-true}
      secret: "${HARNESS_WEBHOOK_SECRET:-}" # Secret for webhook authentication
      path: "/harness"                      # Endpoint path for webhook
      base_url: "${HARNESS_BASE_URL:-https://harness.io}"
      webhook_path: "${HARNESS_WEBHOOK_PATH:-ng/api/webhook}"
      account_id: "${HARNESS_ACCOUNT_ID:-}" # Default account ID
```

Environment variables can be used to customize the configuration:

- `HARNESS_WEBHOOK_ENABLED`: Enable/disable Harness webhook (default: true)
- `HARNESS_WEBHOOK_SECRET`: Secret for webhook authentication
- `HARNESS_BASE_URL`: Base URL for Harness (default: https://harness.io)
- `HARNESS_WEBHOOK_PATH`: Path for webhook endpoint (default: ng/api/webhook)
- `HARNESS_ACCOUNT_ID`: Default Harness account ID

### Harness.io Configuration

To configure a webhook in Harness.io:

1. Get the webhook URL from MCP using the following endpoint:

   ```
   GET /api/v1/webhooks/harness/url?accountIdentifier=YOUR_ACCOUNT_ID&webhookIdentifier=YOUR_WEBHOOK_ID
   ```

   Example:
   ```bash
   curl -H "X-Api-Key: YOUR_API_KEY" \
     "http://your-mcp-server/api/v1/webhooks/harness/url?accountIdentifier=12345abcd&webhookIdentifier=devopsmcp"
   ```

2. Configure a Generic Webhook in Harness.io:
   - Go to Account/Project settings in Harness
   - Click on Webhook under resources
   - Click on New Webhook
   - Enter a Name for the webhook
   - Select the type as "Generic"
   - For Auth type, select "No Auth" if no authentication is needed

3. Link the webhook to your pipeline:
   - Navigate to your pipeline in Harness
   - Select Triggers in the top right corner
   - Click on New Trigger and select Event Relay
   - Provide a Name for the webhook
   - Under Listen on New Webhook, select Event Relay for Payload Type
   - Select the Generic webhook you created earlier
   - Configure appropriate Header Conditions and Payload Conditions
   - Provide any Pipeline input variables as required
   - Click on Create Trigger

4. The webhook URL from step 1 must be used to configure the Harness.io webhook.

## Testing the Integration

You can test the integration using the provided test script:

```bash
./scripts/test-harness-webhook.sh
```

This script:
1. Gets the webhook URL configuration from MCP
2. Sends a test webhook event to MCP
3. Displays the response

You can customize the test by setting environment variables:

```bash
MCP_BASE_URL=http://localhost:8080 \
HARNESS_PATH=/webhook/harness \
API_KEY=your-api-key \
ACCOUNT_ID=your-account-id \
WEBHOOK_ID=your-webhook-id \
WEBHOOK_SECRET=your-webhook-secret \
./scripts/test-harness-webhook.sh
```

## Event Processing

When a webhook is received from Harness.io, MCP will:

1. Validate the webhook signature if a secret is configured
2. Parse the event payload
3. Forward the event to the Harness adapter for processing
4. Record the event in the context if an agent ID is provided

The Harness adapter will process the event based on its type and notify any subscribers.

## Subscribing to Events

To subscribe to Harness events in your custom code:

```go
import "github.com/S-Corkum/mcp-server/internal/adapters/harness"

// Get the Harness adapter
adapter, err := engine.GetAdapter("harness")
if err != nil {
    // Handle error
}

// Convert to Harness adapter
harnessAdapter, ok := adapter.(*harness.Adapter)
if !ok {
    // Handle error
}

// Subscribe to specific event type
err = harnessAdapter.Subscribe("generic_webhook", func(event interface{}) {
    // Process the event
    fmt.Printf("Received event: %+v\n", event)
})
```

## Event Types

Common event types from Harness.io:

- `generic_webhook`: Default event type for generic webhooks
- Other event types as defined in the Harness.io documentation

## Payload Format

The payload format depends on the webhook configuration in Harness.io. For generic webhooks, the payload typically includes:

- Headers from the request
- Payload data
- Event type
- Timestamp

Example payload:
```json
{
  "headers": {
    "Content-Type": "application/json",
    "X-Harness-Event": "test_event"
  },
  "payload": {
    "webhookName": "test-webhook",
    "trigger": {
      "type": "generic",
      "status": "success"
    },
    "application": {
      "name": "Test Application",
      "id": "test-app-123"
    }
  },
  "event_type": "generic_webhook",
  "timestamp": "2023-04-17T10:30:00Z"
}
```

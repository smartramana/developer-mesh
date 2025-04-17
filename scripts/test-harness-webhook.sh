#!/bin/bash

# Script to test Harness.io webhook integration with MCP
# This script simulates sending a webhook event from Harness.io to MCP

# Configuration parameters
MCP_BASE_URL=${MCP_BASE_URL:-http://localhost:8080}
HARNESS_PATH=${HARNESS_PATH:-/webhook/harness}
API_KEY=${API_KEY:-test-api-key}
ACCOUNT_ID=${ACCOUNT_ID:-12345abcd}
WEBHOOK_ID=${WEBHOOK_ID:-devopsmcp}

# Get the webhook URL configuration
echo "Getting webhook URL configuration..."
WEBHOOK_CONFIG_RESPONSE=$(curl -s -H "X-Api-Key: $API_KEY" "$MCP_BASE_URL/api/v1/webhooks/harness/url?accountIdentifier=$ACCOUNT_ID&webhookIdentifier=$WEBHOOK_ID")

# Display the webhook URL for Harness.io configuration
echo "Webhook URL for Harness.io configuration:"
echo "$WEBHOOK_CONFIG_RESPONSE" | jq -r '.webhook_url'

# Calculate HMAC signature for request
generate_signature() {
    local payload=$1
    local secret=$2
    
    # Using openssl for HMAC-SHA256
    signature=$(echo -n "$payload" | openssl dgst -sha256 -hmac "$secret" | cut -d' ' -f2)
    echo "sha256=$signature"
}

# Test webhook with a simple payload
echo -e "\nSending test webhook event to MCP..."

# Sample payload simulating a Harness.io webhook event
PAYLOAD='{
  "webhookName": "test-webhook",
  "trigger": {
    "type": "generic",
    "status": "success"
  },
  "application": {
    "name": "Test Application",
    "id": "test-app-123"
  },
  "pipeline": {
    "name": "Test Pipeline",
    "id": "test-pipeline-456"
  },
  "artifacts": [
    {
      "name": "test-artifact",
      "version": "1.0.0"
    }
  ],
  "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"
}'

SECRET=${WEBHOOK_SECRET:-mock-harness-secret}
SIGNATURE=$(generate_signature "$PAYLOAD" "$SECRET")

# Send the webhook to MCP
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "X-Harness-Signature: $SIGNATURE" \
  -H "X-Harness-Event: test_event" \
  -d "$PAYLOAD" \
  "$MCP_BASE_URL$HARNESS_PATH?accountIdentifier=$ACCOUNT_ID&webhookIdentifier=$WEBHOOK_ID")

echo "Response from MCP:"
echo "$RESPONSE" | jq .

echo -e "\nTest completed"

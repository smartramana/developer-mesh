#!/bin/bash

# Test script for dynamic webhook system
# This script demonstrates the webhook functionality for dynamically added tools

BASE_URL="http://localhost:8090"
API_KEY="sk-test-key-001"  # Update with your actual API key

echo "=== Testing Dynamic Webhook System ==="

# 1. Create a dynamic tool with webhook support
echo -e "\n1. Creating a dynamic tool with webhook discovery..."
TOOL_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/tools" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "GitHub Test Tool",
    "base_url": "https://api.github.com",
    "openapi_url": "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json",
    "provider": "github",
    "credential": {
      "type": "token",
      "token": "github_pat_test"
    }
  }')

TOOL_ID=$(echo $TOOL_RESPONSE | jq -r '.tool.id')
echo "Created tool with ID: $TOOL_ID"

# 2. Get webhook configuration
echo -e "\n2. Getting webhook configuration for the tool..."
WEBHOOK_CONFIG=$(curl -s -X GET "$BASE_URL/api/v1/tools/$TOOL_ID/webhook" \
  -H "X-API-Key: $API_KEY")

echo "Webhook Configuration:"
echo $WEBHOOK_CONFIG | jq '.'

WEBHOOK_URL=$(echo $WEBHOOK_CONFIG | jq -r '.webhook_url')
echo "Webhook URL: $WEBHOOK_URL"

# 3. Test sending a webhook
echo -e "\n3. Testing webhook delivery..."

# Example GitHub webhook payload
PAYLOAD='{
  "action": "opened",
  "pull_request": {
    "id": 1,
    "number": 1,
    "title": "Test PR",
    "state": "open"
  },
  "repository": {
    "name": "test-repo",
    "full_name": "user/test-repo"
  }
}'

# Calculate HMAC signature (if webhook uses HMAC auth)
# This is a simplified example - real implementation would use the actual secret
SIGNATURE="sha256=test-signature"

echo "Sending webhook to: $WEBHOOK_URL"
WEBHOOK_RESPONSE=$(curl -s -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: pull_request" \
  -H "X-Hub-Signature-256: $SIGNATURE" \
  -d "$PAYLOAD")

echo "Webhook Response:"
echo $WEBHOOK_RESPONSE | jq '.'

# 4. List webhook events (if implemented)
echo -e "\n4. Checking webhook events..."
# This endpoint would need to be implemented to list webhook events
# curl -s -X GET "$BASE_URL/api/v1/tools/$TOOL_ID/webhook-events" \
#   -H "X-API-Key: $API_KEY" | jq '.'

echo -e "\n=== Test Complete ===\n"

# Cleanup
echo "To cleanup, delete the tool with:"
echo "curl -X DELETE \"$BASE_URL/api/v1/tools/$TOOL_ID\" -H \"X-API-Key: $API_KEY\""
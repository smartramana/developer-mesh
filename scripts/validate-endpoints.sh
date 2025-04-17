#!/bin/bash

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Base URL
BASE_URL="http://localhost:8080"

# Test health endpoint
echo -e "\nTesting health endpoint..."
HEALTH_RESPONSE=$(curl -s $BASE_URL/health)
if [[ $HEALTH_RESPONSE == *"healthy"* ]]; then
  echo -e "${GREEN}Health check passed!${NC}"
else
  echo -e "${RED}Health check failed!${NC}"
  echo "Response: $HEALTH_RESPONSE"
fi

# Test GitHub webhook endpoint
echo -e "\nTesting GitHub webhook endpoint..."
WEBHOOK_SECRET="test-webhook-secret"
PAYLOAD='{"repository":{"full_name":"test/repo"}}'
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | sed 's/^.* //')

WEBHOOK_RESPONSE=$(curl -s -X POST "$BASE_URL/webhook/github" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: ping" \
  -H "X-Hub-Signature-256: sha256=$SIGNATURE" \
  -d "$PAYLOAD")

if [[ $WEBHOOK_RESPONSE == *"ok"* ]]; then
  echo -e "${GREEN}GitHub webhook test passed!${NC}"
else
  echo -e "${RED}GitHub webhook test failed!${NC}"
  echo "Response: $WEBHOOK_RESPONSE"
fi

# Test context creation (requires JWT auth, so this might fail if auth is enabled)
echo -e "\nTesting context creation (might fail if auth is enabled)..."
CONTEXT_PAYLOAD='{
  "agent_id": "test-agent",
  "model_id": "test-model",
  "content": [],
  "metadata": {
    "source": "validation-test"
  }
}'

CONTEXT_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/contexts" \
  -H "Content-Type: application/json" \
  -d "$CONTEXT_PAYLOAD")

if [[ $CONTEXT_RESPONSE == *"id"* ]]; then
  echo -e "${GREEN}Context creation test passed!${NC}"
  # Extract the context ID for further testing
  CONTEXT_ID=$(echo $CONTEXT_RESPONSE | grep -o '"id":"[^"]*' | cut -d'"' -f4)
  echo "Created context with ID: $CONTEXT_ID"
  
  # Test context retrieval
  echo -e "\nTesting context retrieval..."
  GET_CONTEXT_RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/contexts/$CONTEXT_ID")
  if [[ $GET_CONTEXT_RESPONSE == *"id"* ]]; then
    echo -e "${GREEN}Context retrieval test passed!${NC}"
  else
    echo -e "${RED}Context retrieval test failed!${NC}"
    echo "Response: $GET_CONTEXT_RESPONSE"
  fi
else
  echo -e "${RED}Context creation test failed!${NC}"
  echo "Response: $CONTEXT_RESPONSE"
fi

# Test API listing available tools
echo -e "\nTesting listing available tools..."
TOOLS_RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/tools")
if [[ $TOOLS_RESPONSE == *"tools"* ]]; then
  echo -e "${GREEN}Tools listing test passed!${NC}"
else
  echo -e "${RED}Tools listing test failed!${NC}"
  echo "Response: $TOOLS_RESPONSE"
fi

echo -e "\nValidation tests completed."

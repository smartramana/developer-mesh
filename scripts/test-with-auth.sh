#!/bin/bash

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Base URL
BASE_URL="http://localhost:8080"

# Get JWT token first
echo -e "\nGetting JWT token..."
JWT_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}')

if [[ $JWT_RESPONSE == *"token"* ]]; then
  JWT_TOKEN=$(echo $JWT_RESPONSE | grep -o '"token":"[^"]*' | cut -d'"' -f4)
  echo -e "${GREEN}JWT token obtained!${NC}"
else
  # If login endpoint is not available, use a hardcoded token
  JWT_TOKEN="test-jwt-token"
  echo -e "${RED}Login endpoint not available, using hardcoded token${NC}"
fi

# Test API listing available tools with JWT
echo -e "\nTesting listing available tools with JWT..."
TOOLS_RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/tools" \
  -H "Authorization: Bearer $JWT_TOKEN")

if [[ $TOOLS_RESPONSE == *"tools"* ]]; then
  echo -e "${GREEN}Tools listing test passed!${NC}"
else
  echo -e "${RED}Tools listing test failed!${NC}"
  echo "Response: $TOOLS_RESPONSE"
fi

# Test context creation with API key
echo -e "\nTesting context creation with API key..."
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
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d "$CONTEXT_PAYLOAD")

if [[ $CONTEXT_RESPONSE == *"id"* ]]; then
  echo -e "${GREEN}Context creation test passed!${NC}"
  # Extract the context ID for further testing
  CONTEXT_ID=$(echo $CONTEXT_RESPONSE | grep -o '"id":"[^"]*' | cut -d'"' -f4)
  echo "Created context with ID: $CONTEXT_ID"
  
  # Test context retrieval
  echo -e "\nTesting context retrieval..."
  GET_CONTEXT_RESPONSE=$(curl -s -X GET "$BASE_URL/api/v1/contexts/$CONTEXT_ID" \
    -H "Authorization: Bearer $JWT_TOKEN")
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

echo -e "\nValidation tests completed."

#!/bin/bash

# Simple test script to verify API endpoints 
echo "Testing all MCP API endpoints..."

# Base URL
BASE_URL="http://localhost:8080"

# Health check
echo "1. Testing health endpoint..."
curl -s "${BASE_URL}/health" | grep "healthy" > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ Health endpoint working"
else
    echo "❌ Health endpoint not working"
fi

# Create context
echo "2. Testing context creation..."
CONTEXT_CREATE_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/mcp/context" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer admin-api-key" \
    -d '{
        "agent_id": "test-agent",
        "model_id": "gpt-4",
        "session_id": "test-session"
    }')

echo "Response: $CONTEXT_CREATE_RESPONSE"
echo "$CONTEXT_CREATE_RESPONSE" | grep "context created" > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ Context creation working"
else
    echo "❌ Context creation not working"
fi

# Extract the context ID from the response (assuming it's returned)
CONTEXT_ID=$(echo "$CONTEXT_CREATE_RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
if [ -z "$CONTEXT_ID" ]; then
    # Use a default test ID if extraction fails
    CONTEXT_ID="test-id-1"
    echo "Using default context ID: $CONTEXT_ID"
fi

# Get context
echo "3. Testing get context..."
curl -s -X GET "${BASE_URL}/api/v1/mcp/context/${CONTEXT_ID}" \
    -H "Authorization: Bearer admin-api-key" | grep "context retrieved" > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ Get context working"
else
    echo "❌ Get context not working"
fi

# Update context
echo "4. Testing update context..."
UPDATE_RESPONSE=$(curl -s -X PUT "${BASE_URL}/api/v1/mcp/context/${CONTEXT_ID}" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer admin-api-key" \
    -d '{
        "content": [
            {"role": "system", "content": "You are a helpful assistant"},
            {"role": "user", "content": "Hello!"},
            {"role": "assistant", "content": "Hi there! How can I help you today?"}
        ]
    }')
echo "Update response: $UPDATE_RESPONSE"

# Vector API endpoints
echo "5. Testing vector store endpoint..."
STORE_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/vectors/store" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer admin-api-key" \
    -d '{
        "context_id": "'${CONTEXT_ID}'",
        "content_index": 1,
        "text": "Hello!",
        "embedding": [0.1, 0.2, 0.3, 0.4, 0.5],
        "model_id": "text-embedding-ada-002"
    }')
echo "Store response: $STORE_RESPONSE"

echo "6. Testing vector search endpoint..."
SEARCH_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/vectors/search" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer admin-api-key" \
    -d '{
        "context_id": "'${CONTEXT_ID}'",
        "query_embedding": [0.1, 0.2, 0.3, 0.4, 0.5],
        "limit": 5
    }')
echo "Search response: $SEARCH_RESPONSE"

echo "7. Testing get context embeddings endpoint..."
GET_EMBED_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/v1/vectors/context/${CONTEXT_ID}" \
    -H "Authorization: Bearer admin-api-key")
echo "Get embeddings response: $GET_EMBED_RESPONSE"

echo "8. Testing tool endpoints..."
TOOLS_RESPONSE=$(curl -s -X GET "${BASE_URL}/api/v1/tools" \
    -H "Authorization: Bearer admin-api-key")
echo "Tools response: $TOOLS_RESPONSE"

echo "All tests completed!"

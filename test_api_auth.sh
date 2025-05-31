#!/bin/bash

echo "Testing REST API authentication..."

# Test without auth
echo -e "\n1. Testing without authentication:"
curl -s http://localhost:8081/api/v1/contexts | jq

# Test with API key
echo -e "\n2. Testing with API key:"
curl -s -H "X-API-Key: dev-admin-key-1234567890" http://localhost:8081/api/v1/contexts | jq

# Test health endpoint (no auth required)
echo -e "\n3. Testing health endpoint:"
curl -s http://localhost:8081/health | jq
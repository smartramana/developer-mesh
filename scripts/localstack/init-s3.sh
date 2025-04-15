#!/bin/bash
# Initialize S3 bucket for local development

set -e

echo "Initializing LocalStack S3 for MCP Server"

# Create bucket
echo "Creating S3 bucket: mcp-contexts"
aws --endpoint-url=http://localhost:4566 s3 mb s3://mcp-contexts

# Set bucket policy
echo "Setting bucket policy to allow all operations"
aws --endpoint-url=http://localhost:4566 s3api put-bucket-policy \
  --bucket mcp-contexts \
  --policy '{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": "*",
            "Action": "s3:*",
            "Resource": [
                "arn:aws:s3:::mcp-contexts",
                "arn:aws:s3:::mcp-contexts/*"
            ]
        }
    ]
}'

# Create test context file
echo "Creating test context file"
echo '{
  "id": "test-context",
  "agentID": "test-agent",
  "modelID": "test-model",
  "content": [
    {
      "role": "system",
      "content": "You are a helpful assistant.",
      "tokens": 10
    }
  ],
  "currentTokens": 10,
  "maxTokens": 1000,
  "createdAt": "2025-04-14T10:00:00Z",
  "updatedAt": "2025-04-14T10:00:00Z"
}' > /tmp/test-context.json

# Upload test context file
echo "Uploading test context file"
aws --endpoint-url=http://localhost:4566 s3 cp /tmp/test-context.json s3://mcp-contexts/contexts/test-context.json

echo "S3 bucket initialization complete"

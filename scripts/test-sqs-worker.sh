#!/bin/bash
# Test script for SQS worker with security-configured queue

set -e

echo "Testing SQS Worker with IP-restricted queue..."

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Load environment variables
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs -0)
fi

# Check current IP
CURRENT_IP=$(curl -s https://api.ipify.org)
echo "Current IP: $CURRENT_IP"

# Send a test message
echo -e "\n${GREEN}Sending test message to SQS...${NC}"
MESSAGE_ID=$(aws sqs send-message \
    --queue-url "$SQS_QUEUE_URL" \
    --message-body '{
        "event_type": "integration_test",
        "delivery_id": "test-'$(date +%s)'",
        "repo_name": "devops-mcp",
        "sender_name": "test-script",
        "payload": {"test": true, "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}
    }' \
    --region "$AWS_REGION" \
    --query 'MessageId' \
    --output text)

echo "Message sent with ID: $MESSAGE_ID"

# Start the worker for a short time to process the message
echo -e "\n${GREEN}Starting worker to process message...${NC}"
timeout 15s ./apps/worker/worker || true

echo -e "\n${GREEN}Worker test completed!${NC}"

# Check queue attributes
echo -e "\n${GREEN}Queue Security Status:${NC}"
aws sqs get-queue-attributes \
    --queue-url "$SQS_QUEUE_URL" \
    --attribute-names ApproximateNumberOfMessages SqsManagedSseEnabled \
    --region "$AWS_REGION" \
    --query 'Attributes' \
    --output table

echo -e "\n${GREEN}Test completed successfully!${NC}"
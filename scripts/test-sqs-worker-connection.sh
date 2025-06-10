#!/bin/bash
# Test SQS Worker Connection

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

QUEUE_URL="https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test"
REGION="us-east-1"

echo "========================================="
echo "Testing SQS Worker Connection"
echo "========================================="
echo ""

# 1. Send test messages
echo "1. Sending test messages to queue..."
echo "-----------------------------------"

for i in {1..3}; do
    MESSAGE_BODY=$(cat <<EOF
{
  "id": "test-msg-$i",
  "type": "test",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "data": {
    "message": "Test message $i for worker processing"
  }
}
EOF
)
    
    RESULT=$(aws sqs send-message \
        --queue-url "$QUEUE_URL" \
        --message-body "$MESSAGE_BODY" \
        --region "$REGION" \
        --output json)
    
    MESSAGE_ID=$(echo "$RESULT" | jq -r '.MessageId')
    echo -e "${GREEN}✓ Sent message $i: $MESSAGE_ID${NC}"
done

echo ""

# 2. Check queue status
echo "2. Checking queue status..."
echo "--------------------------"

QUEUE_STATUS=$(aws sqs get-queue-attributes \
    --queue-url "$QUEUE_URL" \
    --attribute-names ApproximateNumberOfMessages ApproximateNumberOfMessagesNotVisible \
    --region "$REGION" \
    --output json)

MESSAGES=$(echo "$QUEUE_STATUS" | jq -r '.Attributes.ApproximateNumberOfMessages')
IN_FLIGHT=$(echo "$QUEUE_STATUS" | jq -r '.Attributes.ApproximateNumberOfMessagesNotVisible')

echo "Messages in queue: $MESSAGES"
echo "Messages in flight: $IN_FLIGHT"

echo ""

# 3. Simulate worker receiving messages
echo "3. Simulating worker message processing..."
echo "-----------------------------------------"

MESSAGES=$(aws sqs receive-message \
    --queue-url "$QUEUE_URL" \
    --region "$REGION" \
    --max-number-of-messages 10 \
    --wait-time-seconds 5 \
    --output json)

if [[ $(echo "$MESSAGES" | jq '.Messages | length') -gt 0 ]]; then
    echo "$MESSAGES" | jq -r '.Messages[] | "Message ID: \(.MessageId)"'
    
    # Process and delete messages
    echo ""
    echo "Processing and deleting messages..."
    
    echo "$MESSAGES" | jq -r '.Messages[] | @base64' | while read -r msg; do
        MESSAGE=$(echo "$msg" | base64 --decode)
        RECEIPT_HANDLE=$(echo "$MESSAGE" | jq -r '.ReceiptHandle')
        MESSAGE_ID=$(echo "$MESSAGE" | jq -r '.MessageId')
        BODY=$(echo "$MESSAGE" | jq -r '.Body')
        
        # Simulate processing
        echo -e "${GREEN}✓ Processing message: $MESSAGE_ID${NC}"
        echo "  Body: $(echo "$BODY" | jq -c '.')"
        
        # Delete message
        aws sqs delete-message \
            --queue-url "$QUEUE_URL" \
            --receipt-handle "$RECEIPT_HANDLE" \
            --region "$REGION"
        
        echo -e "${GREEN}✓ Deleted message: $MESSAGE_ID${NC}"
    done
else
    echo -e "${YELLOW}No messages received from queue${NC}"
fi

echo ""

# 4. Test worker configuration
echo "4. Testing worker configuration..."
echo "---------------------------------"

# Check if worker config exists
if [[ -f "apps/worker/configs/config.docker.yaml" ]]; then
    echo -e "${GREEN}✓ Worker config file exists${NC}"
    
    # Check SQS configuration in worker config
    echo ""
    echo "Worker SQS Configuration:"
    grep -A 5 "sqs:" apps/worker/configs/config.docker.yaml || echo "No SQS config found in worker config"
else
    echo -e "${RED}✗ Worker config file not found${NC}"
fi

echo ""

# 5. Environment validation
echo "5. Environment validation..."
echo "---------------------------"

# Check required environment variables
REQUIRED_VARS=("AWS_REGION" "SQS_QUEUE_URL")
MISSING_VARS=()

for var in "${REQUIRED_VARS[@]}"; do
    if [[ -z "${!var}" ]]; then
        MISSING_VARS+=("$var")
        echo -e "${YELLOW}⚠ $var is not set${NC}"
    else
        echo -e "${GREEN}✓ $var is set: ${!var}${NC}"
    fi
done

# Check AWS credentials
if aws sts get-caller-identity &>/dev/null; then
    echo -e "${GREEN}✓ AWS credentials are configured${NC}"
else
    echo -e "${RED}✗ AWS credentials not configured${NC}"
fi

echo ""

# 6. Connection test summary
echo "========================================="
echo "Connection Test Summary"
echo "========================================="

if [[ ${#MISSING_VARS[@]} -eq 0 ]]; then
    echo -e "${GREEN}✓ All environment variables are set${NC}"
else
    echo -e "${YELLOW}⚠ Missing environment variables: ${MISSING_VARS[*]}${NC}"
fi

echo ""
echo "To run the worker locally:"
echo "-------------------------"
echo "export AWS_REGION=us-east-1"
echo "export SQS_QUEUE_URL=$QUEUE_URL"
echo "cd apps/worker"
echo "go run cmd/worker/main.go"

echo ""
echo "To run with Docker:"
echo "------------------"
echo "docker-compose -f docker-compose.local.yml up worker"

echo ""
echo "========================================="
echo "Test Complete"
echo "========================================="
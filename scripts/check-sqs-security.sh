#!/bin/bash
# SQS Security Configuration Checker

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

QUEUE_URL="https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test"
REGION="us-east-1"

echo "======================================"
echo "SQS Queue Security Configuration Check"
echo "======================================"
echo ""

# 1. Check Queue Attributes
echo "1. Queue Configuration:"
echo "----------------------"
aws sqs get-queue-attributes \
  --queue-url "$QUEUE_URL" \
  --attribute-names All \
  --region "$REGION" \
  --output table

echo ""

# 2. Check Access Policy
echo "2. Access Policy Analysis:"
echo "-------------------------"
POLICY=$(aws sqs get-queue-attributes \
  --queue-url "$QUEUE_URL" \
  --attribute-names Policy \
  --region "$REGION" \
  --query 'Attributes.Policy' \
  --output text)

echo "Current Policy:"
echo "$POLICY" | jq '.'

# 3. Check Encryption
echo ""
echo "3. Encryption Settings:"
echo "----------------------"
SSE_ENABLED=$(aws sqs get-queue-attributes \
  --queue-url "$QUEUE_URL" \
  --attribute-names SqsManagedSseEnabled KmsMasterKeyId \
  --region "$REGION" \
  --query 'Attributes' \
  --output json)

echo "$SSE_ENABLED" | jq '.'

if [[ $(echo "$SSE_ENABLED" | jq -r '.SqsManagedSseEnabled // "false"') == "true" ]]; then
    echo -e "${GREEN}✓ Server-Side Encryption (SSE) is enabled${NC}"
else
    echo -e "${YELLOW}⚠ Server-Side Encryption (SSE) is not enabled${NC}"
fi

# 4. Check Dead Letter Queue
echo ""
echo "4. Dead Letter Queue Configuration:"
echo "----------------------------------"
DLQ=$(aws sqs get-queue-attributes \
  --queue-url "$QUEUE_URL" \
  --attribute-names RedrivePolicy \
  --region "$REGION" \
  --query 'Attributes.RedrivePolicy' \
  --output text 2>/dev/null || echo "")

if [[ -n "$DLQ" && "$DLQ" != "None" ]]; then
    echo "$DLQ" | jq '.' 2>/dev/null || echo "$DLQ"
    echo -e "${GREEN}✓ Dead Letter Queue is configured${NC}"
else
    echo -e "${YELLOW}⚠ No Dead Letter Queue configured${NC}"
fi

# 5. Check Current IAM Permissions
echo ""
echo "5. Current IAM Identity:"
echo "-----------------------"
aws sts get-caller-identity --output table

# 6. Test Message Operations
echo ""
echo "6. Permission Tests:"
echo "-------------------"

# Test SendMessage
if aws sqs send-message \
    --queue-url "$QUEUE_URL" \
    --message-body '{"test": "security-check"}' \
    --region "$REGION" \
    --output json > /dev/null 2>&1; then
    echo -e "${GREEN}✓ SendMessage permission: GRANTED${NC}"
else
    echo -e "${RED}✗ SendMessage permission: DENIED${NC}"
fi

# Test ReceiveMessage
if aws sqs receive-message \
    --queue-url "$QUEUE_URL" \
    --region "$REGION" \
    --max-number-of-messages 1 \
    --output json > /dev/null 2>&1; then
    echo -e "${GREEN}✓ ReceiveMessage permission: GRANTED${NC}"
else
    echo -e "${RED}✗ ReceiveMessage permission: DENIED${NC}"
fi

# Test DeleteMessage (without actual deletion)
if aws sqs get-queue-attributes \
    --queue-url "$QUEUE_URL" \
    --attribute-names QueueArn \
    --region "$REGION" \
    --output json > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Queue access permission: GRANTED${NC}"
else
    echo -e "${RED}✗ Queue access permission: DENIED${NC}"
fi

# 7. Security Recommendations
echo ""
echo "======================================"
echo "Security Recommendations:"
echo "======================================"

# Check if using root credentials
if [[ $(aws sts get-caller-identity --query 'Arn' --output text) == *":root" ]]; then
    echo -e "${RED}⚠ WARNING: Using root credentials!${NC}"
    echo "  Recommendation: Create an IAM user or role with minimal required permissions"
fi

# Check encryption
if [[ $(echo "$SSE_ENABLED" | jq -r '.SqsManagedSseEnabled // "false"') != "true" ]]; then
    echo -e "${YELLOW}⚠ Enable Server-Side Encryption:${NC}"
    echo "  aws sqs set-queue-attributes --queue-url $QUEUE_URL --attributes SqsManagedSseEnabled=true"
fi

# Check DLQ
if [[ "$DLQ" == "null" ]]; then
    echo -e "${YELLOW}⚠ Configure Dead Letter Queue:${NC}"
    echo "  - Create a DLQ: aws sqs create-queue --queue-name sean-mcp-test-dlq"
    echo "  - Configure redrive policy with maxReceiveCount"
fi

# IAM Best Practices
echo ""
echo "IAM Best Practices:"
echo "------------------"
echo "1. Create a dedicated IAM user/role for the application"
echo "2. Use the principle of least privilege"
echo "3. Required permissions for the application:"
echo "   - sqs:SendMessage"
echo "   - sqs:ReceiveMessage"
echo "   - sqs:DeleteMessage"
echo "   - sqs:GetQueueAttributes"
echo "   - sqs:ChangeMessageVisibility (for processing)"

# Example IAM Policy
echo ""
echo "Example Minimal IAM Policy:"
echo "--------------------------"
cat << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sqs:SendMessage",
        "sqs:ReceiveMessage",
        "sqs:DeleteMessage",
        "sqs:GetQueueAttributes",
        "sqs:ChangeMessageVisibility"
      ],
      "Resource": "arn:aws:sqs:us-east-1:594992249511:sean-mcp-test"
    }
  ]
}
EOF

echo ""
echo "======================================"
echo "Check Complete"
echo "======================================"
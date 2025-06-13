#!/bin/bash
# Test script to validate AWS services connectivity

set -e

echo "======================================"
echo "AWS Services Connectivity Test"
echo "======================================"

# Source environment
source .env

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test S3
echo -e "\n${YELLOW}Testing S3 Bucket Access...${NC}"
if aws s3 ls s3://$S3_BUCKET --region $AWS_REGION >/dev/null 2>&1; then
    echo -e "${GREEN}✓ S3 bucket accessible${NC}"
    # Try to write a test file
    echo "test" | aws s3 cp - s3://$S3_BUCKET/test.txt --region $AWS_REGION
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ S3 write successful${NC}"
        aws s3 rm s3://$S3_BUCKET/test.txt --region $AWS_REGION
    else
        echo -e "${RED}✗ S3 write failed${NC}"
    fi
else
    echo -e "${RED}✗ S3 bucket not accessible${NC}"
fi

# Test SQS
echo -e "\n${YELLOW}Testing SQS Queue Access...${NC}"
if aws sqs get-queue-attributes --queue-url $SQS_QUEUE_URL --attribute-names All --region $AWS_REGION >/dev/null 2>&1; then
    echo -e "${GREEN}✓ SQS queue accessible${NC}"
else
    echo -e "${RED}✗ SQS queue not accessible${NC}"
fi

# Test ElastiCache (requires SSH tunnel)
echo -e "\n${YELLOW}Testing ElastiCache Redis...${NC}"
if [ "$USE_SSH_TUNNEL_FOR_REDIS" = "true" ]; then
    echo "Checking for SSH tunnel on localhost:6379..."
    if nc -zv localhost 6379 2>&1 | grep -q succeeded; then
        echo -e "${GREEN}✓ Redis tunnel is active${NC}"
        if command -v redis-cli >/dev/null 2>&1; then
            if redis-cli -h localhost -p 6379 ping >/dev/null 2>&1; then
                echo -e "${GREEN}✓ Redis connection successful${NC}"
            else
                echo -e "${RED}✗ Redis connection failed${NC}"
            fi
        else
            echo -e "${YELLOW}! redis-cli not installed, skipping ping test${NC}"
        fi
    else
        echo -e "${RED}✗ SSH tunnel not active${NC}"
        echo "  Run ./scripts/aws/connect-elasticache.sh in another terminal"
    fi
else
    echo "Direct ElastiCache connection configured (VPC access required)"
fi

# Test Bedrock
echo -e "\n${YELLOW}Testing AWS Bedrock Access...${NC}"
if aws bedrock list-foundation-models --region $AWS_REGION --query 'modelSummaries[0].modelId' >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Bedrock accessible${NC}"
else
    echo -e "${RED}✗ Bedrock not accessible${NC}"
    echo "  Note: Bedrock requires specific IAM permissions"
fi

# Test Database
echo -e "\n${YELLOW}Testing PostgreSQL Database...${NC}"
if PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -U $DATABASE_USER -d $DATABASE_NAME -c "SELECT 1" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ PostgreSQL accessible${NC}"
else
    echo -e "${RED}✗ PostgreSQL not accessible${NC}"
fi

echo -e "\n======================================"
echo "Test Summary:"
echo "======================================"
echo "S3 Bucket: $S3_BUCKET"
echo "SQS Queue: $SQS_QUEUE_URL"
echo "Redis: ${REDIS_HOST}:${REDIS_PORT}"
echo "Region: $AWS_REGION"
echo ""
echo "To run functional tests:"
echo "1. Ensure SSH tunnel is active: ./scripts/aws/connect-elasticache.sh"
echo "2. Ensure PostgreSQL is running: docker ps | grep postgres"
echo "3. Run: make test-functional"
echo "======================================"
#!/bin/bash
# Update Security Groups with New IP Address
# This script updates all security groups when your home IP changes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}DevOps MCP - Security Group IP Update${NC}"
echo "========================================"

# Get current IP
CURRENT_IP=$(curl -s https://ipinfo.io/ip)
if [ -z "$CURRENT_IP" ]; then
    echo -e "${RED}Error: Could not determine current IP address${NC}"
    exit 1
fi

echo -e "Current IP: ${GREEN}${CURRENT_IP}${NC}"

# Configuration
REGION=${AWS_REGION:-us-east-1}
OLD_IP_FILE=".last_known_ip"

# Get last known IP
if [ -f "$OLD_IP_FILE" ]; then
    OLD_IP=$(cat "$OLD_IP_FILE")
    echo -e "Last known IP: ${YELLOW}${OLD_IP}${NC}"
else
    echo -e "${YELLOW}No previous IP found. Please enter your old IP address:${NC}"
    read -r OLD_IP
fi

if [ "$CURRENT_IP" == "$OLD_IP" ]; then
    echo -e "${GREEN}IP has not changed. No updates needed.${NC}"
    exit 0
fi

echo -e "\n${YELLOW}Updating security groups from ${OLD_IP} to ${CURRENT_IP}...${NC}\n"

# Function to update security group
update_security_group() {
    local sg_id=$1
    local sg_name=$2
    local port=$3
    local protocol=${4:-tcp}
    
    echo -e "Updating ${GREEN}${sg_name}${NC} (${sg_id})..."
    
    # Revoke old IP
    if [ ! -z "$OLD_IP" ]; then
        echo -e "  Revoking access for ${OLD_IP}/${port}..."
        aws ec2 revoke-security-group-ingress \
            --group-id "$sg_id" \
            --protocol "$protocol" \
            --port "$port" \
            --cidr "${OLD_IP}/32" \
            --region "$REGION" 2>/dev/null || echo -e "  ${YELLOW}Old rule not found or already removed${NC}"
    fi
    
    # Authorize new IP
    echo -e "  Authorizing access for ${CURRENT_IP}/${port}..."
    aws ec2 authorize-security-group-ingress \
        --group-id "$sg_id" \
        --protocol "$protocol" \
        --port "$port" \
        --cidr "${CURRENT_IP}/32" \
        --region "$REGION" \
        --group-rule-description "Home IP - Updated $(date +%Y-%m-%d)" 2>/dev/null || echo -e "  ${YELLOW}Rule already exists${NC}"
    
    echo -e "  ${GREEN}✓ Updated${NC}\n"
}

# Find security groups
echo -e "${YELLOW}Finding DevOps MCP security groups...${NC}"

# Get VPC ID
VPC_ID=$(aws ec2 describe-vpcs \
    --filters "Name=tag:Name,Values=devops-mcp-vpc" \
    --query 'Vpcs[0].VpcId' \
    --output text \
    --region "$REGION")

if [ "$VPC_ID" == "None" ] || [ -z "$VPC_ID" ]; then
    echo -e "${RED}Error: Could not find DevOps MCP VPC${NC}"
    echo "Make sure the AWS infrastructure is set up first."
    exit 1
fi

echo -e "Found VPC: ${GREEN}${VPC_ID}${NC}\n"

# Get Application Security Group
APP_SG_ID=$(aws ec2 describe-security-groups \
    --filters "Name=vpc-id,Values=${VPC_ID}" "Name=group-name,Values=devops-mcp-app-sg" \
    --query 'SecurityGroups[0].GroupId' \
    --output text \
    --region "$REGION")

if [ "$APP_SG_ID" != "None" ] && [ ! -z "$APP_SG_ID" ]; then
    update_security_group "$APP_SG_ID" "Application Security Group" 5432 tcp
    update_security_group "$APP_SG_ID" "Application Security Group" 6379 tcp
else
    echo -e "${YELLOW}Warning: Application security group not found${NC}"
fi

# Update S3 bucket policy
echo -e "${YELLOW}Updating S3 bucket policy...${NC}"
S3_BUCKET="sean-mcp-dev-contexts"

# Get current bucket policy
CURRENT_POLICY=$(aws s3api get-bucket-policy --bucket "$S3_BUCKET" --query Policy --output text 2>/dev/null || echo "")

if [ ! -z "$CURRENT_POLICY" ]; then
    # Update the IP in the policy
    NEW_POLICY=$(echo "$CURRENT_POLICY" | sed "s/${OLD_IP}/${CURRENT_IP}/g")
    
    # Apply the new policy
    echo "$NEW_POLICY" > /tmp/bucket-policy.json
    aws s3api put-bucket-policy --bucket "$S3_BUCKET" --policy file:///tmp/bucket-policy.json
    rm -f /tmp/bucket-policy.json
    echo -e "${GREEN}✓ S3 bucket policy updated${NC}\n"
else
    echo -e "${YELLOW}Creating new S3 bucket policy...${NC}"
    cat > /tmp/bucket-policy.json << EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "IPRestriction",
            "Effect": "Deny",
            "Principal": "*",
            "Action": "s3:*",
            "Resource": [
                "arn:aws:s3:::${S3_BUCKET}/*",
                "arn:aws:s3:::${S3_BUCKET}"
            ],
            "Condition": {
                "NotIpAddress": {
                    "aws:SourceIp": ["${CURRENT_IP}/32"]
                }
            }
        }
    ]
}
EOF
    aws s3api put-bucket-policy --bucket "$S3_BUCKET" --policy file:///tmp/bucket-policy.json
    rm -f /tmp/bucket-policy.json
    echo -e "${GREEN}✓ S3 bucket policy created${NC}\n"
fi

# Save current IP for next time
echo "$CURRENT_IP" > "$OLD_IP_FILE"

echo -e "${GREEN}✅ All security groups and policies have been updated!${NC}"
echo -e "\nYou can now connect to:"
echo -e "  - RDS PostgreSQL on port 5432"
echo -e "  - ElastiCache Redis on port 6379"
echo -e "  - S3 bucket: ${S3_BUCKET}"
echo -e "\nFrom IP: ${GREEN}${CURRENT_IP}${NC}"

# Update .env file if it exists
if [ -f ".env" ] || [ -f ".env.production" ]; then
    echo -e "\n${YELLOW}Note: Remember to update any IP addresses in your .env files${NC}"
fi
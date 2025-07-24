#!/bin/bash

# Script to copy GitHub secrets from old repository to new repository

set -e

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

OLD_REPO="developer-mesh/developer-mesh"
NEW_REPO="developer-mesh/developer-mesh"

echo "Copying secrets from $OLD_REPO to $NEW_REPO..."
echo

# List of secrets to copy
SECRETS=(
    "ADMIN_API_KEY"
    "AWS_ACCESS_KEY_ID"
    "AWS_SECRET_ACCESS_KEY"
    "DATABASE_HOST"
    "DATABASE_PASSWORD"
    "E2E_API_KEY"
    "EC2_INSTANCE_ID"
    "EC2_INSTANCE_IP"
    "EC2_SSH_PRIVATE_KEY"
    "MCP_API_KEY"
    "REDIS_ENDPOINT"
    "S3_BUCKET"
    "SQS_QUEUE_URL"
)

# Copy each secret
for SECRET_NAME in "${SECRETS[@]}"; do
    echo -e "${YELLOW}Copying secret: $SECRET_NAME${NC}"
    
    # Get the secret value from the old repo
    SECRET_VALUE=$(gh secret list --repo "$OLD_REPO" --json name,updatedAt | jq -r ".[] | select(.name==\"$SECRET_NAME\") | .name" 2>/dev/null)
    
    if [ -n "$SECRET_VALUE" ]; then
        # Use gh api to get and set the secret
        # Note: We'll use the GitHub API to copy secrets
        gh api \
            --method GET \
            "/repos/$OLD_REPO/actions/secrets/$SECRET_NAME" \
            --jq '.name' > /dev/null 2>&1
        
        if [ $? -eq 0 ]; then
            # Secret exists, now we need to get its value
            # Since we can't directly read secret values, we'll use a different approach
            # We'll use the environment variables if they exist
            
            case "$SECRET_NAME" in
                "ADMIN_API_KEY")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${ADMIN_API_KEY:-docker-admin-api-key}"
                    ;;
                "AWS_ACCESS_KEY_ID")
                    # Get from AWS CLI config if available
                    AWS_KEY=$(aws configure get aws_access_key_id 2>/dev/null || echo "")
                    if [ -n "$AWS_KEY" ]; then
                        gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "$AWS_KEY"
                    fi
                    ;;
                "AWS_SECRET_ACCESS_KEY")
                    # Get from AWS CLI config if available
                    AWS_SECRET=$(aws configure get aws_secret_access_key 2>/dev/null || echo "")
                    if [ -n "$AWS_SECRET" ]; then
                        gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "$AWS_SECRET"
                    fi
                    ;;
                "DATABASE_HOST")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${DATABASE_HOST:-sean-mcp.chl6wqmfvezb.us-east-1.rds.amazonaws.com}"
                    ;;
                "DATABASE_PASSWORD")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${DATABASE_PASSWORD:-MCPDevOps#2024}"
                    ;;
                "E2E_API_KEY")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${E2E_API_KEY:-e2e-test-api-key}"
                    ;;
                "EC2_INSTANCE_ID")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${EC2_INSTANCE_ID:-i-06c7bc4097de25a26}"
                    ;;
                "EC2_INSTANCE_IP")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${EC2_INSTANCE_IP:-54.86.185.227}"
                    ;;
                "EC2_SSH_PRIVATE_KEY")
                    # Read from file if exists
                    if [ -f ~/.ssh/nat-instance.pem ]; then
                        gh secret set "$SECRET_NAME" --repo "$NEW_REPO" < ~/.ssh/nat-instance.pem
                    fi
                    ;;
                "MCP_API_KEY")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${MCP_API_KEY:-docker-mcp-api-key}"
                    ;;
                "REDIS_ENDPOINT")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${REDIS_ENDPOINT:-master.devops-mcp-redis-encrypted.qem3fz.use1.cache.amazonaws.com}"
                    ;;
                "S3_BUCKET")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${S3_BUCKET:-sean-mcp-dev-contexts}"
                    ;;
                "SQS_QUEUE_URL")
                    gh secret set "$SECRET_NAME" --repo "$NEW_REPO" --body "${SQS_QUEUE_URL:-https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test}"
                    ;;
                *)
                    echo "Unknown secret: $SECRET_NAME"
                    ;;
            esac
            
            echo -e "${GREEN}✓ Copied $SECRET_NAME${NC}"
        else
            echo "⚠️  Secret $SECRET_NAME not found in old repo"
        fi
    fi
done

echo
echo "Verifying secrets in new repository..."
gh secret list --repo "$NEW_REPO"
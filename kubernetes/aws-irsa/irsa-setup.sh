#!/bin/bash
# This script sets up IAM Roles for Service Accounts (IRSA) for the MCP server
# It creates an IAM policy and role that the Kubernetes service account can assume

# Set variables (replace with your values)
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query "Account" --output text)
AWS_REGION=$(aws configure get region)
CLUSTER_NAME="mcp-cluster"
POLICY_NAME="mcp-server-policy"
ROLE_NAME="mcp-server-role"
NAMESPACE="default"
SERVICE_ACCOUNT_NAME="mcp-server"
S3_BUCKET_NAME="mcp-contexts"
DATABASE_USERNAME="mcp_app"

# Create S3 bucket if it doesn't exist
echo "Creating S3 bucket for contexts..."
aws s3api create-bucket \
    --bucket $S3_BUCKET_NAME \
    --region $AWS_REGION \
    --create-bucket-configuration LocationConstraint=$AWS_REGION || true

# Enable server-side encryption for S3 bucket
aws s3api put-bucket-encryption \
    --bucket $S3_BUCKET_NAME \
    --server-side-encryption-configuration '{"Rules": [{"ApplyServerSideEncryptionByDefault": {"SSEAlgorithm": "AES256"}}]}'

# Update the policy with the correct account ID, region, and username
cat iam-policy.json | \
    sed "s/<AWS_ACCOUNT_ID>/$AWS_ACCOUNT_ID/g" | \
    sed "s/<AWS_REGION>/$AWS_REGION/g" | \
    sed "s/<DATABASE_USERNAME>/$DATABASE_USERNAME/g" > /tmp/iam-policy.json

# Create IAM policy
echo "Creating IAM policy..."
aws iam create-policy \
    --policy-name $POLICY_NAME \
    --policy-document file:///tmp/iam-policy.json \
    --description "Policy for MCP server to access S3 and RDS"

# Get OIDC provider URL for the EKS cluster
OIDC_PROVIDER=$(aws eks describe-cluster --name $CLUSTER_NAME --query "cluster.identity.oidc.issuer" --output text | sed -e "s/^https:\/\///")

# Check if OIDC provider exists
if ! aws iam list-open-id-connect-providers | grep -q $(echo $OIDC_PROVIDER | sed -e "s/\//\\\\\//g"); then
    echo "Error: OIDC provider does not exist for cluster $CLUSTER_NAME"
    echo "Run the following command to create it:"
    echo "eksctl utils associate-iam-oidc-provider --cluster $CLUSTER_NAME --approve"
    exit 1
fi

# Create IAM role
echo "Creating IAM role..."
cat > /tmp/trust-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::${AWS_ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "${OIDC_PROVIDER}:sub": "system:serviceaccount:${NAMESPACE}:${SERVICE_ACCOUNT_NAME}",
          "${OIDC_PROVIDER}:aud": "sts.amazonaws.com"
        }
      }
    }
  ]
}
EOF

aws iam create-role \
    --role-name $ROLE_NAME \
    --assume-role-policy-document file:///tmp/trust-policy.json \
    --description "Role for MCP server service account"

# Attach policy to role
echo "Attaching policy to role..."
aws iam attach-role-policy \
    --role-name $ROLE_NAME \
    --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/${POLICY_NAME}

# Update the service account YAML with the correct role ARN
cat service-account.yaml | \
    sed "s/<AWS_ACCOUNT_ID>/$AWS_ACCOUNT_ID/g" > /tmp/service-account.yaml

echo "IRSA setup complete!"
echo "Remember to apply the service account: kubectl apply -f /tmp/service-account.yaml"
echo ""
echo "To verify setup, check that the pod can access AWS resources:"
echo "kubectl run aws-cli --image=amazon/aws-cli --serviceaccount=mcp-server -i --rm -- aws s3 ls s3://$S3_BUCKET_NAME"

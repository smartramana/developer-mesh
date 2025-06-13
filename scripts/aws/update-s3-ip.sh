#!/bin/bash
# Update S3 bucket policy with current IP address

# Source environment variables
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Use environment variables with defaults
BUCKET="${S3_BUCKET:-dev-contexts}"
BASTION_IP="${BASTION_HOST_IP:-}"
AWS_ACCOUNT_ID="${AWS_ACCOUNT_ID:-$(aws sts get-caller-identity --query Account --output text 2>/dev/null)}"

if [ -z "$AWS_ACCOUNT_ID" ]; then
    echo "Error: Unable to determine AWS account ID"
    echo "Please set AWS_ACCOUNT_ID in your .env file"
    exit 1
fi

# Get current public IP
CURRENT_IP=$(curl -s https://api.ipify.org)

echo "Updating S3 bucket policy with IP: $CURRENT_IP"

# Only include bastion policy if BASTION_IP is set
if [ -n "$BASTION_IP" ]; then
    BASTION_POLICY=",
    {
      \"Sid\": \"AllowBastionAccess\",
      \"Effect\": \"Allow\",
      \"Principal\": {
        \"AWS\": \"arn:aws:iam::${AWS_ACCOUNT_ID}:root\"
      },
      \"Action\": \"s3:*\",
      \"Resource\": [
        \"arn:aws:s3:::${BUCKET}/*\",
        \"arn:aws:s3:::${BUCKET}\"
      ],
      \"Condition\": {
        \"IpAddress\": {
          \"aws:SourceIp\": \"${BASTION_IP}/32\"
        }
      }
    }"
else
    BASTION_POLICY=""
fi

# Create updated policy
aws s3api put-bucket-policy --bucket "$BUCKET" --policy "{
  \"Version\": \"2012-10-17\",
  \"Statement\": [
    {
      \"Sid\": \"DenyInsecureConnections\",
      \"Effect\": \"Deny\",
      \"Principal\": \"*\",
      \"Action\": \"s3:*\",
      \"Resource\": [
        \"arn:aws:s3:::${BUCKET}/*\",
        \"arn:aws:s3:::${BUCKET}\"
      ],
      \"Condition\": {
        \"Bool\": {
          \"aws:SecureTransport\": \"false\"
        }
      }
    },
    {
      \"Sid\": \"AllowSpecificIPOnly\",
      \"Effect\": \"Allow\",
      \"Principal\": {
        \"AWS\": \"arn:aws:iam::${AWS_ACCOUNT_ID}:root\"
      },
      \"Action\": \"s3:*\",
      \"Resource\": [
        \"arn:aws:s3:::${BUCKET}/*\",
        \"arn:aws:s3:::${BUCKET}\"
      ],
      \"Condition\": {
        \"IpAddress\": {
          \"aws:SourceIp\": \"${CURRENT_IP}/32\"
        }
      }
    }${BASTION_POLICY}
  ]
}" --region us-east-1

echo "✓ S3 bucket policy updated successfully"
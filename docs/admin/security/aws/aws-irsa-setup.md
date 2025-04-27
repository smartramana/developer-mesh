# Setting up AWS IAM Roles for Service Accounts (IRSA)

This guide explains how to set up IRSA for MCP Server to securely interact with AWS services without managing static credentials.

## What is IRSA?

IAM Roles for Service Accounts (IRSA) allows Kubernetes pods to use AWS IAM roles to access AWS services. This approach is more secure than using static AWS credentials because:

1. No AWS access keys are stored in the container or configuration
2. Permissions are scoped to specific pods using Kubernetes service accounts
3. Temporary credentials are automatically rotated
4. All AWS API calls can be audited through CloudTrail

## Prerequisites

1. An EKS cluster with the EKS Pod Identity Agent addon enabled
2. AWS CLI configured with appropriate permissions
3. kubectl configured to access your EKS cluster
4. eksctl (optional, but recommended for easier setup)

## Step 1: Create IAM Policies

Create IAM policies for each AWS service that MCP Server needs to access. You can use the policy templates provided in this directory as starting points:

- [RDS IAM Policy](./rds-iam-policy.json)
- [ElastiCache IAM Policy](./elasticache-iam-policy.json)
- [S3 IAM Policy](./s3-iam-policy.json)
- [Combined IAM Policy](./combined-iam-policy.json)

Replace the placeholder values (e.g., `${AWS_REGION}`, `${ACCOUNT_ID}`, etc.) with your actual values.

Example using AWS CLI:

```bash
# Get your AWS account ID
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

# Create IAM policy for combined access
aws iam create-policy \
    --policy-name MCP-Server-AWS-Access \
    --policy-document file://combined-iam-policy.json
```

## Step 2: Create IAM Role for Service Account

Use eksctl or AWS CLI to create an IAM role and associate it with a Kubernetes service account.

Using eksctl:

```bash
eksctl create iamserviceaccount \
    --name mcp-server \
    --namespace mcp \
    --cluster your-eks-cluster \
    --attach-policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/MCP-Server-AWS-Access \
    --approve
```

Alternatively, using AWS CLI:

```bash
# Create IAM role trust policy
cat > trust-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::${AWS_ACCOUNT_ID}:oidc-provider/oidc.eks.${AWS_REGION}.amazonaws.com/id/${OIDC_ID}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "oidc.eks.${AWS_REGION}.amazonaws.com/id/${OIDC_ID}:sub": "system:serviceaccount:mcp:mcp-server"
        }
      }
    }
  ]
}
EOF

# Create IAM role
aws iam create-role \
    --role-name mcp-server-role \
    --assume-role-policy-document file://trust-policy.json

# Attach policy to role
aws iam attach-role-policy \
    --role-name mcp-server-role \
    --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/MCP-Server-AWS-Access
```

## Step 3: Update Kubernetes Service Account

Update the ServiceAccount in `kubernetes/serviceaccount.yaml` with the correct ARN:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mcp-server
  namespace: mcp
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::${AWS_ACCOUNT_ID}:role/mcp-server-role
    eks.amazonaws.com/sts-regional-endpoints: "true"
```

## Step 4: Configure AWS Services

### RDS Aurora PostgreSQL

1. Enable IAM authentication on your RDS Aurora PostgreSQL cluster:

```bash
aws rds modify-db-cluster \
    --db-cluster-identifier your-aurora-cluster \
    --enable-iam-database-authentication
```

2. Create a database user with IAM authentication:

```sql
CREATE USER your_db_username WITH LOGIN;
GRANT rds_iam TO your_db_username;
GRANT ALL PRIVILEGES ON DATABASE your_database TO your_db_username;
```

### ElastiCache Redis

1. Create an ElastiCache user with IAM authentication:

```bash
aws elasticache create-user \
    --user-id your-redis-username \
    --user-name your-redis-username \
    --engine redis \
    --authentication-mode Type=iam \
    --access-string "on ~* +@all"
```

2. Associate the user with your ElastiCache Redis cluster:

```bash
aws elasticache modify-user-group \
    --user-group-id default \
    --user-ids your-redis-username
```

3. Enable user group access on your Redis cluster:

```bash
aws elasticache modify-replication-group \
    --replication-group-id your-redis-cluster \
    --user-group-ids default
```

### S3 Bucket

Ensure your S3 bucket has the necessary permissions:

```bash
aws s3api put-bucket-policy \
    --bucket your-bucket-name \
    --policy file://bucket-policy.json
```

Where bucket-policy.json contains:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::${AWS_ACCOUNT_ID}:role/mcp-server-role"
            },
            "Action": [
                "s3:ListBucket"
            ],
            "Resource": "arn:aws:s3:::your-bucket-name"
        },
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::${AWS_ACCOUNT_ID}:role/mcp-server-role"
            },
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject"
            ],
            "Resource": "arn:aws:s3:::your-bucket-name/*"
        }
    ]
}
```

## Step 5: Deploy MCP Server

Deploy the MCP Server with the updated ServiceAccount and configuration:

```bash
kubectl apply -f kubernetes/namespace.yaml
kubectl apply -f kubernetes/serviceaccount.yaml
kubectl apply -f kubernetes/deployment.yaml
kubectl apply -f kubernetes/service.yaml
```

## Verification

To verify that IRSA is working correctly:

1. Check that the pod starts without authentication errors:

```bash
kubectl logs -n mcp deployment/mcp-server
```

2. You should see log messages indicating that IRSA authentication is being used:

```
Using IRSA authentication for AWS services
```

3. Test the connections to RDS, ElastiCache, and S3 using the MCP Server API.

## Troubleshooting

### Common Issues

1. **Pod fails to start with AWS authentication errors**

   Check that the EKS Pod Identity Agent is working correctly:

   ```bash
   kubectl describe pod -n mcp -l app=mcp-server
   ```

   Look for environment variables like `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE`.

2. **Database connection failures**

   Verify that the IAM user has been created in PostgreSQL and has the correct permissions:

   ```sql
   SELECT * FROM pg_user WHERE usename = 'your_db_username';
   SELECT * FROM pg_authid WHERE rolname = 'your_db_username';
   ```

3. **ElastiCache authentication issues**

   Confirm that IAM authentication is properly configured for the ElastiCache user and cluster:

   ```bash
   aws elasticache describe-users --user-id your-redis-username
   aws elasticache describe-user-groups --user-group-id default
   ```

4. **S3 access denied errors**

   Check the IAM policy and bucket policy to ensure the correct permissions are granted:

   ```bash
   aws iam get-policy-version --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/MCP-Server-AWS-Access --version-id v1
   aws s3api get-bucket-policy --bucket your-bucket-name
   ```

## Local Development

For local development and testing, you can use the following techniques:

1. **AWS credentials file**: Use a standard `~/.aws/credentials` file with a profile that has the necessary permissions.
2. **AWS_* environment variables**: Set the standard AWS environment variables in your development environment.
3. **AWS IAM authentication with Docker**: When using Docker Compose, you can mount your AWS credentials.

See the [Local Development Guide](../development/local-aws-auth.md) for more details.

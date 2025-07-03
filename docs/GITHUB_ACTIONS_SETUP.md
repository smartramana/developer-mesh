# GitHub Actions CD Pipeline Setup Guide

## Prerequisites
- GitHub repository secrets access
- AWS IAM user with deployment permissions
- EC2 SSH key pair
- Slack webhook (optional)

## Required GitHub Secrets

### 1. AWS Credentials
```bash
AWS_ACCESS_KEY_ID         # IAM user access key
AWS_SECRET_ACCESS_KEY     # IAM user secret key
```

**Required IAM Permissions:**
- EC2: DescribeInstances, DescribeSecurityGroups
- RDS: DescribeDBInstances
- ElastiCache: DescribeCacheClusters
- S3: GetObject, PutObject (for S3_BUCKET)
- SQS: SendMessage, ReceiveMessage (for SQS_QUEUE_URL)
- CloudWatch: PutMetricData

### 2. EC2 Access
```bash
EC2_SSH_PRIVATE_KEY       # Private key for EC2 access
```

**Setup:**
```bash
# Copy the content of your EC2 key
cat ~/.ssh/nat-instance-temp
# Add to GitHub Secrets (include -----BEGIN/END RSA PRIVATE KEY-----)
```

### 3. Database Configuration
```bash
DATABASE_HOST             # devops-mcp-postgres.cshaq28kmnw8.us-east-1.rds.amazonaws.com
DATABASE_PASSWORD         # RDS master password
```

### 4. Redis Configuration
```bash
REDIS_ENDPOINT            # master.devops-mcp-redis-encrypted.qem3fz.use1.cache.amazonaws.com:6379
```

### 5. AWS Resources
```bash
S3_BUCKET                 # sean-mcp-dev-contexts
SQS_QUEUE_URL            # https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
```

### 6. Application Secrets
```bash
ADMIN_API_KEY            # Your admin API key
GITHUB_TOKEN             # GitHub personal access token (for GHCR access)
```

### 7. Notifications (Optional)
```bash
SLACK_WEBHOOK            # Slack incoming webhook URL
```

## Setting Secrets via GitHub CLI

```bash
# Install GitHub CLI
brew install gh

# Authenticate
gh auth login

# Set secrets
gh secret set AWS_ACCESS_KEY_ID -b "your-access-key"
gh secret set AWS_SECRET_ACCESS_KEY -b "your-secret-key"
gh secret set EC2_SSH_PRIVATE_KEY < ~/.ssh/nat-instance-temp
gh secret set DATABASE_PASSWORD -b "your-db-password"
gh secret set DATABASE_HOST -b "devops-mcp-postgres.cshaq28kmnw8.us-east-1.rds.amazonaws.com"
gh secret set REDIS_ENDPOINT -b "master.devops-mcp-redis-encrypted.qem3fz.use1.cache.amazonaws.com:6379"
gh secret set S3_BUCKET -b "sean-mcp-dev-contexts"
gh secret set SQS_QUEUE_URL -b "https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test"
gh secret set ADMIN_API_KEY -b "your-admin-key"
gh secret set GITHUB_TOKEN -b "ghp_your_token"
```

## Environment Configuration

### Production Environment Setup
1. Go to Settings → Environments
2. Create "production" environment
3. Add protection rules:
   - Required reviewers: 1
   - Restrict to main branch
   - Add specific users/teams

### Environment Secrets
You can also set secrets at the environment level for better security:
- Move production-specific secrets to the production environment
- Keep only shared secrets at repository level

## Testing the Pipeline

### 1. Dry Run
```bash
# Create a test branch
git checkout -b test-deployment

# Make a small change
echo "# Test" >> README.md
git add . && git commit -m "test: deployment pipeline"
git push origin test-deployment

# Open PR and check that CI passes
```

### 2. Manual Workflow Dispatch
- Go to Actions tab
- Select "Deploy to Production"
- Click "Run workflow"
- Choose branch and options

### 3. Monitor Deployment
```bash
# Watch logs
gh run watch

# Check deployment status
gh run list --workflow=deploy-production-v2.yml
```

## Rollback Procedures

### Automatic Rollback
The pipeline automatically rolls back if:
- Health checks fail
- Smoke tests fail
- Container startup fails

### Manual Rollback
```bash
# SSH to instance
ssh -i ~/.ssh/nat-instance-temp ec2-user@44.211.47.174

# Rollback to previous version
cd /home/ec2-user/devops-mcp
docker-compose down
sed -i 's|:main-.*|:latest|g' docker-compose.yml
docker-compose up -d
```

## Troubleshooting

### Common Issues

1. **SSH Connection Failed**
   - Check EC2_SSH_PRIVATE_KEY format
   - Verify security group allows GitHub Actions IPs
   - Check instance is running

2. **Image Pull Failed**
   - Verify GITHUB_TOKEN has `read:packages` scope
   - Check image exists in GHCR
   - Verify image tag format

3. **Health Check Failed**
   - Check container logs on EC2
   - Verify configuration files
   - Check AWS service connectivity

4. **Migration Failed**
   - Check DATABASE_PASSWORD is correct
   - Verify RDS security group
   - Check migration files exist

### Debug Commands
```bash
# Check GitHub secrets (names only)
gh secret list

# View workflow runs
gh run list

# Download logs
gh run download <run-id>

# Re-run failed workflow
gh run rerun <run-id>
```

## Security Best Practices

1. **Rotate Secrets Quarterly**
   - AWS access keys
   - Database passwords
   - API keys

2. **Use IAM Roles When Possible**
   - Consider EC2 instance profiles
   - Use IRSA for EKS deployments

3. **Audit Access**
   - Review who has access to secrets
   - Monitor secret usage in workflows
   - Enable GitHub audit logs

4. **Principle of Least Privilege**
   - Create deployment-specific IAM user
   - Limit permissions to required resources
   - Use resource tags for access control

## Next Steps

1. **Enable the Workflow**
   - Set all required secrets
   - Test with manual dispatch
   - Enable automatic deployment

2. **Add Monitoring**
   - CloudWatch alarms
   - Deployment metrics
   - Error tracking

3. **Enhance Testing**
   - Add integration tests
   - Performance benchmarks
   - Security scanning

4. **Multi-Environment**
   - Add staging environment
   - Create environment-specific workflows
   - Implement promotion process
# AWS Scripts

This directory contains scripts for managing AWS services access in development.

## Scripts

### connect-elasticache.sh
Creates an SSH tunnel through the bastion host to access ElastiCache Redis.
- **Usage**: `./scripts/aws/connect-elasticache.sh`
- **Purpose**: Forwards localhost:6379 to ElastiCache endpoint
- **Required**: SSH key at `~/.ssh/dev-bastion-key.pem`
- **Bastion**: 107.21.156.59

### update-s3-ip.sh
Updates S3 bucket policy with your current public IP address.
- **Usage**: `./scripts/aws/update-s3-ip.sh`
- **Purpose**: Ensures S3 access from your current location
- **Bucket**: sean-mcp-dev-contexts

### test-aws-services.sh
Validates connectivity to all configured AWS services.
- **Usage**: `./scripts/aws/test-aws-services.sh`
- **Tests**: S3, SQS, ElastiCache (via tunnel), Bedrock
- **Purpose**: Pre-flight check before running tests

## Required Environment Variables

These scripts require the following environment variables to be set in your `.env` file:

```bash
# For connect-elasticache.sh
BASTION_HOST_IP=<your-bastion-ip>
BASTION_KEY_FILE=$HOME/.ssh/<your-key.pem>
ELASTICACHE_ENDPOINT=<your-elasticache-endpoint>

# For update-s3-ip.sh
S3_BUCKET=<your-s3-bucket-name>
AWS_ACCOUNT_ID=<your-aws-account-id>  # Optional, auto-detected
BASTION_HOST_IP=<your-bastion-ip>     # Optional, for bastion access

# For all scripts
AWS_REGION=<your-aws-region>
AWS_ACCESS_KEY_ID=<your-access-key>
AWS_SECRET_ACCESS_KEY=<your-secret-key>
```

## Security Notes

- **No hardcoded credentials**: All sensitive values come from environment variables
- **AWS credentials**: Never commit AWS credentials to the repository
- **SSH keys**: Keep your bastion SSH key secure and never commit it
- **IP restrictions**: S3 and security groups restrict access by IP
- **ElastiCache**: Only accessible via SSH tunnel through bastion
- **Account isolation**: Scripts use AWS STS to detect account ID dynamically
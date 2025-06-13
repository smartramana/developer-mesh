# Local Development with Real AWS Services

This guide explains how to run the DevOps MCP services locally while connecting to real AWS services.

## Overview

The configuration supports a hybrid development model where:
- Services run as local binaries (not in Docker containers)
- AWS services (ElastiCache, S3, SQS, Bedrock) are used instead of local alternatives
- Local PostgreSQL is still used for the database

## Configuration

### 1. Environment Variables

Copy the example environment file and update with your AWS credentials:

```bash
cp .env.aws-local.example .env
```

Key environment variables:

```bash
# AWS Credentials
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key

# ElastiCache Redis
REDIS_HOST=sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com
REDIS_PORT=6379

# SQS Queue
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
WORKER_QUEUE_TYPE=sqs

# S3 Storage
S3_BUCKET=mcp-dev-contexts
STORAGE_PROVIDER=s3

# Enable AWS services
ELASTICACHE_ENABLED=true
SQS_ENABLED=true
S3_ENABLED=true
BEDROCK_ENABLED=true
```

### 2. Configuration Files

The development configuration (`configs/config.development.yaml`) is pre-configured to use AWS services when the appropriate environment variables are set.

Key configuration sections:

#### Cache (ElastiCache)
```yaml
cache:
  distributed:
    type: "redis"
    address: "${REDIS_HOST:-sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com}:${REDIS_PORT:-6379}"
```

#### Storage (S3)
```yaml
storage:
  context:
    provider: "${STORAGE_PROVIDER:-s3}"
  s3:
    region: "${AWS_REGION:-us-east-1}"
    bucket: "${S3_BUCKET:-mcp-dev-contexts}"
```

#### Worker (SQS)
```yaml
worker:
  queue_type: "${WORKER_QUEUE_TYPE:-sqs}"
  sqs:
    queue_url: "${SQS_QUEUE_URL:-https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test}"
```

#### Embeddings (Bedrock)
```yaml
embedding:
  providers:
    bedrock:
      enabled: ${BEDROCK_ENABLED:-true}
      region: "${AWS_REGION:-us-east-1}"
```

## Running Services Locally

### 1. Start Local PostgreSQL

```bash
# Using Docker for PostgreSQL only
docker run -d \
  --name postgres-dev \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=devops_mcp_dev \
  -p 5432:5432 \
  postgres:15
```

### 2. Run Database Migrations

```bash
make migrate-local
```

### 3. Start Services

Start each service in a separate terminal:

```bash
# Terminal 1: MCP Server
cd apps/mcp-server
go run cmd/server/main.go

# Terminal 2: REST API
cd apps/rest-api
go run cmd/api/main.go

# Terminal 3: Worker (if using SQS)
cd apps/worker
go run cmd/worker/main.go
```

## Switching Between AWS and Local Services

### Use Local Services Only

To use local services instead of AWS:

```bash
# In .env file
REDIS_HOST=localhost
STORAGE_PROVIDER=filesystem
WORKER_QUEUE_TYPE=memory
ELASTICACHE_ENABLED=false
SQS_ENABLED=false
S3_ENABLED=false
BEDROCK_ENABLED=false
```

### Use Mixed Mode

You can mix local and AWS services:

```bash
# Use AWS for storage and embeddings, local for cache and queue
REDIS_HOST=localhost
STORAGE_PROVIDER=s3
WORKER_QUEUE_TYPE=memory
BEDROCK_ENABLED=true
```

## Troubleshooting

### Connection Issues

1. **ElastiCache**: Ensure your local machine can connect to the ElastiCache endpoint
   - Check security groups
   - Verify VPC connectivity
   - Test with: `redis-cli -h sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com ping`

2. **S3**: Verify AWS credentials and bucket permissions
   - Test with: `aws s3 ls s3://mcp-dev-contexts/`

3. **SQS**: Check queue permissions and URL
   - Test with: `aws sqs receive-message --queue-url https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test`

### Performance Considerations

- **Latency**: Expect higher latency when connecting to AWS services from local machine
- **Costs**: Be aware of AWS service costs (data transfer, API calls)
- **Rate Limits**: AWS services have rate limits that may differ from local alternatives

## Best Practices

1. **Credentials**: Never commit AWS credentials to version control
2. **Environment Separation**: Use different AWS resources for each developer
3. **Cost Management**: Monitor AWS usage to avoid unexpected charges
4. **Fallback**: Keep local service configuration as fallback option

## Configuration Hierarchy

1. Environment variables (highest priority)
2. `configs/config.development.yaml`
3. `configs/config.base.yaml` (lowest priority)

Environment variables always override configuration file values.
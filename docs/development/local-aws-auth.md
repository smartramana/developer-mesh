# Local Development with AWS Authentication

This guide explains how to set up local development with AWS service integrations.

## Overview

When developing locally, you can't use IRSA (IAM Roles for Service Accounts) since it requires running in an EKS cluster. Instead, you'll need to use alternative authentication methods:

1. Standard AWS credential providers (credentials file, environment variables)
2. Local emulators for AWS services
3. Fallback to non-IAM authentication methods

## Option 1: Using AWS Credentials Locally

### Prerequisites

1. AWS CLI installed and configured
2. Access to AWS services (RDS, ElastiCache, S3)
3. IAM user with appropriate permissions

### Setup

1. Configure AWS credentials in your `~/.aws/credentials` file:

```ini
[mcp-development]
aws_access_key_id = YOUR_ACCESS_KEY
aws_secret_access_key = YOUR_SECRET_KEY
region = us-west-2
```

2. Update your environment variables to use this profile:

```bash
export AWS_PROFILE=mcp-development
```

3. When running MCP Server locally, it will automatically use these credentials.

## Option 2: Using Local AWS Emulators

For faster development without connecting to actual AWS services, you can use local emulators.

### LocalStack for S3

The project already includes LocalStack configuration in the `docker-compose.yml` file. LocalStack emulates S3 and other AWS services locally.

To use LocalStack:

1. Start the services with Docker Compose:

```bash
docker-compose up -d localstack
```

2. Configure MCP to use the LocalStack endpoint:

```
MCP_STORAGE_S3_ENDPOINT=http://localhost:4566
MCP_STORAGE_S3_FORCE_PATH_STYLE=true
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
```

### Local PostgreSQL for RDS

You can use a standard PostgreSQL instance running in Docker:

```bash
docker-compose up -d postgres
```

Configure environment variables to use standard authentication:

```
MCP_DATABASE_DSN=postgres://postgres:postgres@localhost:5432/mcp?sslmode=disable
```

### Local Redis for ElastiCache

For Redis in standard mode:

```bash
docker-compose up -d redis
```

Configure environment variables:

```
MCP_CACHE_ADDRESS=localhost:6379
```

For Redis in cluster mode, you can use redis-cluster:

```yaml
# Add this to your docker-compose.yml
redis-cluster:
  image: docker.io/bitnami/redis-cluster:7.0
  ports:
    - '6379:6379'
    - '6380:6380'
    - '6381:6381'
    - '6382:6382'
    - '6383:6383'
    - '6384:6384'
  environment:
    - REDIS_PASSWORD=bitnami
    - REDIS_NODES=redis-cluster-0:6379 redis-cluster-1:6380 redis-cluster-2:6381 redis-cluster-3:6382 redis-cluster-4:6383 redis-cluster-5:6384
    - REDIS_CLUSTER_REPLICAS=1
    - REDIS_CLUSTER_CREATOR=yes
```

Configure environment variables:

```
MCP_CACHE_TYPE=redis_cluster
MCP_CACHE_ADDRESSES=localhost:6379,localhost:6380,localhost:6381,localhost:6382,localhost:6383,localhost:6384
MCP_CACHE_PASSWORD=bitnami
```

## Option 3: Mixed Authentication Example

This example shows a mixed approach with real AWS S3, but local PostgreSQL and Redis:

```bash
#!/bin/bash
# start-local-dev.sh

# Start local services
docker-compose up -d postgres redis

# Set AWS S3 authentication
export AWS_PROFILE=mcp-development

# Set local database and cache credentials
export MCP_DATABASE_DSN=postgres://postgres:postgres@localhost:5432/mcp?sslmode=disable
export MCP_CACHE_ADDRESS=localhost:6379

# Use AWS S3 for storage
export MCP_STORAGE_TYPE=s3
export MCP_STORAGE_S3_BUCKET=your-dev-bucket
export MCP_STORAGE_S3_REGION=us-west-2
export MCP_STORAGE_CONTEXT_STORAGE_PROVIDER=s3

# Start the server
go run ./cmd/server/main.go
```

## Option 4: Using modified docker-compose.yml for local Redis cluster mode

Create a modified docker-compose file for local development with Redis in cluster mode:

```yaml
# docker-compose.cluster.yml
version: '3.8'

services:
  redis-node-0:
    image: redis:7.0-alpine
    command: redis-server /usr/local/etc/redis/redis.conf
    ports:
      - "7000:7000"
    volumes:
      - ./configs/redis-cluster/redis-0.conf:/usr/local/etc/redis/redis.conf
    networks:
      - mcp-network

  redis-node-1:
    image: redis:7.0-alpine
    command: redis-server /usr/local/etc/redis/redis.conf
    ports:
      - "7001:7001"
    volumes:
      - ./configs/redis-cluster/redis-1.conf:/usr/local/etc/redis/redis.conf
    networks:
      - mcp-network

  redis-node-2:
    image: redis:7.0-alpine
    command: redis-server /usr/local/etc/redis/redis.conf
    ports:
      - "7002:7002"
    volumes:
      - ./configs/redis-cluster/redis-2.conf:/usr/local/etc/redis/redis.conf
    networks:
      - mcp-network

  redis-node-3:
    image: redis:7.0-alpine
    command: redis-server /usr/local/etc/redis/redis.conf
    ports:
      - "7003:7003"
    volumes:
      - ./configs/redis-cluster/redis-3.conf:/usr/local/etc/redis/redis.conf
    networks:
      - mcp-network

  redis-node-4:
    image: redis:7.0-alpine
    command: redis-server /usr/local/etc/redis/redis.conf
    ports:
      - "7004:7004"
    volumes:
      - ./configs/redis-cluster/redis-4.conf:/usr/local/etc/redis/redis.conf
    networks:
      - mcp-network

  redis-node-5:
    image: redis:7.0-alpine
    command: redis-server /usr/local/etc/redis/redis.conf
    ports:
      - "7005:7005"
    volumes:
      - ./configs/redis-cluster/redis-5.conf:/usr/local/etc/redis/redis.conf
    networks:
      - mcp-network

  redis-cluster-init:
    image: redis:7.0-alpine
    depends_on:
      - redis-node-0
      - redis-node-1
      - redis-node-2
      - redis-node-3
      - redis-node-4
      - redis-node-5
    command: >
      sh -c "redis-cli --cluster create 
             redis-node-0:7000 redis-node-1:7001 redis-node-2:7002 
             redis-node-3:7003 redis-node-4:7004 redis-node-5:7005 
             --cluster-replicas 1 --cluster-yes"
    networks:
      - mcp-network

networks:
  mcp-network:
    driver: bridge
```

Then you would need to create Redis configuration files for each node in the `configs/redis-cluster/` directory.

## Switching Between Local and AWS Authentication

To simplify switching between local development and AWS authentication, you can use environment-specific configuration files:

1. Create environment-specific configuration files:
   - `configs/config.dev.yaml` - For local development
   - `configs/config.aws.yaml` - For AWS authentication

2. Create a script to switch between them:

```bash
#!/bin/bash
# switch-env.sh

if [ "$1" == "local" ]; then
  echo "Switching to local development environment"
  export MCP_CONFIG_FILE=configs/config.dev.yaml
elif [ "$1" == "aws" ]; then
  echo "Switching to AWS environment"
  export MCP_CONFIG_FILE=configs/config.aws.yaml
else
  echo "Usage: ./switch-env.sh [local|aws]"
  exit 1
fi

# Display current configuration
echo "Using configuration file: $MCP_CONFIG_FILE"
```

## Troubleshooting

### AWS Authentication Issues

If you encounter authentication issues with AWS services:

1. Verify your AWS credentials are correctly configured:

```bash
aws sts get-caller-identity
```

2. Check that your IAM user has the necessary permissions.

3. For S3, try using the AWS CLI to test access:

```bash
aws s3 ls s3://your-bucket-name/
```

### Redis Cluster Mode Issues

If Redis cluster mode is not working correctly:

1. Check cluster status:

```bash
redis-cli -p 7000 cluster nodes
```

2. Verify cluster slots are correctly assigned:

```bash
redis-cli -p 7000 cluster slots
```

3. Test connection from your application:

```bash
redis-cli -c -p 7000 set test value
redis-cli -c -p 7001 get test
```

The `-c` flag enables cluster mode in the Redis CLI.

### Database Connection Issues

If you can't connect to the database:

1. Verify the database is running:

```bash
docker-compose ps postgres
```

2. Test connection directly:

```bash
psql -h localhost -p 5432 -U postgres -d mcp
```

3. Check logs for connection errors:

```bash
docker-compose logs postgres
```

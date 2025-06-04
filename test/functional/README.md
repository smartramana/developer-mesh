# Functional Tests

This directory contains functional tests for the DevOps MCP platform.

## Setup

### 1. Start Required Services

The functional tests require several services to be running. Use docker-compose to start them:

```bash
# From the project root directory
make docker-compose-up
# or
docker compose -f docker-compose.local.yml up -d
```

This will start:
- MCP Server (port 8080)
- REST API (port 8081)
- Mock Server (port 8082)
- PostgreSQL with pgvector
- Redis
- LocalStack (S3 and SQS)
- Worker service

### 2. Configure Environment

The functional tests require several environment variables. You have three options:

#### Option A: Use Default Values (Recommended for local testing)
```bash
# The check_test_env.sh script will set default values
source ./check_test_env.sh
```

#### Option B: Use .env File
```bash
# Copy the example env file
cp .env.example .env
# Edit .env with your values if needed
# Then source the setup script
source ./setup_env.sh
```

#### Option C: Set Environment Variables Manually
```bash
export MCP_SERVER_URL=http://localhost:8080
export MCP_API_KEY=dev-admin-key-1234567890
export MOCKSERVER_URL=http://localhost:8082
export GITHUB_TOKEN=test-github-token
export GITHUB_REPO=test-repo
export GITHUB_OWNER=test-org
export ELASTICACHE_ENDPOINT=localhost
export ELASTICACHE_PORT=6379
export MCP_GITHUB_WEBHOOK_SECRET=docker-github-webhook-secret
export REDIS_ADDR=localhost:6379
```

### 3. Verify Setup

Run the setup script to verify all services are accessible:

```bash
./setup_env.sh
```

## Running Tests

Once everything is set up, run the functional tests:

```bash
# Run all functional tests
go test -v ./...

# Run specific test suites
go test -v ./api
go test -v ./mcp
go test -v ./webhook

# Run with specific timeout
go test -v -timeout 5m ./...
```

## Troubleshooting

### Services Not Accessible

If the setup script shows services are not accessible:

1. Check if docker-compose is running:
   ```bash
   docker compose -f docker-compose.local.yml ps
   ```

2. Check service logs:
   ```bash
   docker compose -f docker-compose.local.yml logs <service-name>
   ```

3. Ensure LocalStack initialization completed:
   ```bash
   docker compose -f docker-compose.local.yml logs localstack-init
   ```

### Environment Variables

If tests fail due to missing environment variables:

1. Run `./check_test_env.sh` to see which variables are missing
2. Either set them manually or use the `.env` file approach
3. Make sure to `source` the scripts, not just execute them

### LocalStack Issues

If you see S3 or SQS related errors:

1. Check LocalStack is healthy:
   ```bash
   curl http://localhost:4566/_localstack/health
   ```

2. Verify S3 bucket was created:
   ```bash
   aws --endpoint-url=http://localhost:4566 s3 ls
   ```

3. Verify SQS queue was created:
   ```bash
   aws --endpoint-url=http://localhost:4566 sqs list-queues
   ```

## Clean Up

To stop all services:

```bash
make docker-compose-down
# or
docker compose -f docker-compose.local.yml down
```

To clean up volumes as well:

```bash
docker compose -f docker-compose.local.yml down -v
```
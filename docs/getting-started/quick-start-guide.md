# Quick Start Guide

Get DevOps MCP running locally in under 5 minutes.

## Prerequisites

Ensure you have the following installed:

- **[Go](https://golang.org/doc/install)** 1.24 or later
- **[Docker](https://www.docker.com/get-started)** and Docker Compose
- **[Git](https://git-scm.com/downloads)**
- **Make** (usually pre-installed on Linux/Mac)

### Optional (for full features)
- AWS CLI configured (for S3/SQS features)
- PostgreSQL client tools (for direct DB access)

## 🚀 Quick Setup

### 1. Clone and Configure

```bash
# Clone the repository
git clone https://github.com/S-Corkum/devops-mcp.git
cd devops-mcp

# Copy configuration template
cp config.yaml.example config.yaml
# Edit config.yaml with your settings (especially API tokens)

# (Optional) Edit config.yaml for your environment
# Most defaults work out of the box
```

### 2. Start Infrastructure

```bash
# Start PostgreSQL, Redis, and LocalStack
make dev-setup

# Wait for services to be ready (usually ~10 seconds)
# Check logs if needed
make docker-compose-logs

# Note: This command runs:
# - PostgreSQL 17 with pgvector extension
# - Redis 7 Alpine
# - LocalStack for AWS services (SQS)
# - Automatic SQS queue creation
```

### 3. Initialize Database

```bash
# Run database migrations
make migrate-local

# This creates tables and indexes including pgvector
# Migrations are located in apps/rest-api/migrations/sql/
```

### 4. Build and Run Services

**Option A: Run All Services (Recommended)**

```bash
# First build all services
make build

# Then in separate terminal windows:

# Terminal 1 - MCP Server (port 8080)
make run-mcp-server

# Terminal 2 - REST API (port 8081)
make run-rest-api

# Terminal 3 - Worker (processes SQS messages)
make run-worker
```

**Option B: Run with Docker Compose**

```bash
# Build and run all services
make local-dev

# This automatically:
# - Builds all service containers
# - Starts PostgreSQL, Redis, LocalStack
# - Creates SQS queues
# - Runs all three services
# - Exposes ports 8080 (MCP) and 8081 (REST API)
```

### 5. Verify Installation

```bash
# Check MCP Server health
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","version":"1.0.0"}

# Check REST API health
curl http://localhost:8081/health

# Expected response:
# {"status":"healthy","components":{"database":"up","redis":"up"}}
```

### 6. API Documentation

Swagger UI is available for both services:

- **MCP Server Swagger**: http://localhost:8080/swagger/index.html
- **REST API Swagger**: http://localhost:8081/swagger/index.html

Note: Generate/update Swagger docs with:
```bash
make swagger
```

## 🎯 First API Call

Create your first context:

```bash
# Create a context
curl -X POST http://localhost:8081/api/v1/contexts \
  -H "Content-Type: application/json" \
  -H "X-API-Key: change-this-admin-key" \
  -d '{
    "name": "My First Context",
    "type": "conversation",
    "metadata": {
      "description": "Testing DevOps MCP"
    }
  }'

# Response includes the created context with ID
# Note: Update the API key to match your config.yaml
```

## 🛠️ Common Operations

### View Logs

```bash
# All services
make docker-compose-logs

# Specific service
docker-compose -f docker-compose.local.yml logs mcp-server -f
```

### Run Tests

```bash
# Unit tests (fast)
make test

# Test specific module
make test-mcp-server
make test-rest-api
make test-worker

# Integration tests (requires Docker)
make test-integration

# Functional tests (requires full stack)
make test-functional

# Test coverage report
make test-coverage-html
open coverage.html

# Run tests with specific focus
make test-functional-focus FOCUS="Health Endpoint"
```

### Stop Services

```bash
# Stop Docker Compose services
make docker-compose-down

# Stop individual services (Ctrl+C in their terminals)
```

## 📁 Project Structure Overview

```
devops-mcp/
├── apps/               # Microservices (Go workspace modules)
│   ├── mcp-server/     # MCP protocol implementation
│   ├── rest-api/       # REST API endpoints  
│   └── worker/         # Async job processor
├── pkg/                # Shared libraries
├── configs/            # Configuration files
├── docs/               # Documentation
├── scripts/            # Utility scripts
├── test/               # Integration & functional tests
├── go.work             # Go workspace configuration
└── Makefile            # Build automation
```

## 🔧 Configuration

### Environment Variables

Common environment variables:

```bash
# Database
export DATABASE_URL="postgres://dev:dev@localhost:5432/dev?sslmode=disable"
# Or use the DSN format:
export DATABASE_DSN="postgresql://dev:dev@localhost:5432/dev?sslmode=disable"

# Redis
export REDIS_URL="redis://localhost:6379"

# AWS (optional)
export AWS_REGION="us-west-2"
export AWS_PROFILE="default"

# Logging
export LOG_LEVEL="debug"
```

### Configuration File

Key settings in `config.yaml`:

```yaml
api:
  listen_address: ":8080"
  enable_cors: true

database:
  host: localhost
  port: 5432
  database: mcp
  
cache:
  type: redis
  address: localhost:6379
```

## 🧪 Testing the Integration

### 1. GitHub Webhook Test

```bash
# Send a test webhook
./scripts/test-github-webhook.sh

# Check worker logs for processing
```

### 2. Vector Search Test

```bash
# Create embedding
curl -X POST http://localhost:8081/api/v1/vectors \
  -H "Content-Type: application/json" \
  -H "X-API-Key: change-this-admin-key" \
  -d '{
    "content": "DevOps automation with AI",
    "context_id": "your-context-id"
  }'

# Search similar content
curl -X POST http://localhost:8081/api/v1/vectors/search \
  -H "Content-Type: application/json" \
  -H "X-API-Key: change-this-reader-key" \
  -d '{
    "query": "AI-powered DevOps tools",
    "limit": 5
  }'
```

### 3. Useful Helper Scripts

```bash
# Health check all services
./scripts/health-check.sh

# Validate all endpoints
./scripts/validate-endpoints.sh

# Test GitHub integration
./scripts/test-github-integration.sh

# Redis connectivity check
./scripts/redis-check.sh
```

## 🚨 Troubleshooting

### Port Already in Use

```bash
# Find process using port
lsof -i :8080  # MCP Server
lsof -i :8081  # REST API

# Kill process
kill -9 <PID>

# Or kill by port
kill -9 $(lsof -t -i:8080)
```

### Database Connection Failed

```bash
# Check PostgreSQL is running
docker-compose -f docker-compose.local.yml ps

# Restart PostgreSQL
docker-compose -f docker-compose.local.yml restart postgres
```

### Build Errors

```bash
# Clean and rebuild
make clean
go work sync
make build

# If module errors occur:
cd apps/mcp-server && go mod tidy
cd apps/rest-api && go mod tidy
cd apps/worker && go mod tidy
go work sync
```

### Permission Denied

```bash
# Fix script permissions
chmod +x scripts/*.sh
```

### Go Version Issues

```bash
# Check Go version (must be 1.24+)
go version

# If using older version, update Go:
# macOS: brew upgrade go
# Linux: Follow https://golang.org/doc/install
```

### LocalStack/SQS Issues

```bash
# Check LocalStack is running
docker-compose -f docker-compose.local.yml ps localstack

# Manually create SQS queue if needed
aws --endpoint-url=http://localhost:4566 sqs create-queue --queue-name tasks
```

## 📚 Next Steps

Now that you have DevOps MCP running:

1. **Explore Examples**: Check out [integration examples](../examples/README.md)
2. **Read Architecture**: Understand the [system design](../architecture/system-overview.md)
3. **Try API Endpoints**: Review the [API documentation](../api-reference/vector-search-api.md)
4. **Setup IDE**: Configure your [development environment](../developer/development-environment.md)

## 💡 Tips

- Use `make help` to see all available commands (note: help target may not exist, check Makefile)
- Enable debug logging with `LOG_LEVEL=debug`
- Check logs with `make docker-compose-logs` or `docker-compose -f docker-compose.local.yml logs -f`
- Use `docker-compose -f docker-compose.local.yml ps` to verify service status
- All services log to stdout by default

## 🆘 Getting Help

If you encounter issues:

1. Check the [troubleshooting guide](../troubleshooting/)
2. Search [GitHub Issues](https://github.com/S-Corkum/devops-mcp/issues)
3. Ask in [Discussions](https://github.com/S-Corkum/devops-mcp/discussions)

---

**Congratulations!** You now have a fully functional DevOps MCP development environment. 🎉
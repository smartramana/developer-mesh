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
cp config.yaml.template config.yaml

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
```

### 3. Initialize Database

```bash
# Run database migrations
make migrate-local

# This creates tables and indexes including pgvector
```

### 4. Build and Run Services

**Option A: Run All Services (Recommended)**

```bash
# In separate terminal windows:

# Terminal 1 - MCP Server
make run-mcp-server

# Terminal 2 - REST API
make run-rest-api

# Terminal 3 - Worker
make run-worker
```

**Option B: Run with Docker Compose**

```bash
# Build and run all services
docker-compose -f docker-compose.local.yml up --build
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

## 🎯 First API Call

Create your first context:

```bash
# Create a context
curl -X POST http://localhost:8081/api/v1/contexts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My First Context",
    "type": "conversation",
    "metadata": {
      "description": "Testing DevOps MCP"
    }
  }'

# Response includes the created context with ID
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
# Unit tests
make test

# Integration tests (requires running services)
make test-integration

# Test coverage report
make test-coverage-html
open coverage.html
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
├── apps/           # Microservices
│   ├── mcp-server/ # MCP protocol implementation
│   ├── rest-api/   # REST API endpoints  
│   └── worker/     # Async job processor
├── pkg/            # Shared libraries
├── configs/        # Configuration files
├── migrations/     # Database migrations
└── scripts/        # Utility scripts
```

## 🔧 Configuration

### Environment Variables

Common environment variables:

```bash
# Database
export DATABASE_URL="postgres://user:pass@localhost:5432/mcp?sslmode=disable"

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
  -d '{
    "content": "DevOps automation with AI",
    "context_id": "your-context-id"
  }'

# Search similar content
curl -X POST http://localhost:8081/api/v1/vectors/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "AI-powered DevOps tools",
    "limit": 5
  }'
```

## 🚨 Troubleshooting

### Port Already in Use

```bash
# Find process using port
lsof -i :8080

# Kill process
kill -9 <PID>
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
```

### Permission Denied

```bash
# Fix script permissions
chmod +x scripts/*.sh
```

## 📚 Next Steps

Now that you have DevOps MCP running:

1. **Explore Examples**: Check out [integration examples](../examples/README.md)
2. **Read Architecture**: Understand the [system design](../architecture/system-overview.md)
3. **Try API Endpoints**: Review the [API documentation](../api-reference/vector-search-api.md)
4. **Setup IDE**: Configure your [development environment](../developer/development-environment.md)

## 💡 Tips

- Use `make help` to see all available commands
- Enable debug logging with `LOG_LEVEL=debug`
- Check `./logs/` directory for detailed logs
- Use `docker-compose ps` to verify service status

## 🆘 Getting Help

If you encounter issues:

1. Check the [troubleshooting guide](../troubleshooting/)
2. Search [GitHub Issues](https://github.com/S-Corkum/devops-mcp/issues)
3. Ask in [Discussions](https://github.com/S-Corkum/devops-mcp/discussions)

---

**Congratulations!** You now have a fully functional DevOps MCP development environment. 🎉
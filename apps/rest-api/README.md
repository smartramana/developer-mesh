# REST API Service

The REST API service provides comprehensive data management and search capabilities for the DevOps MCP platform.

## Overview

This service handles:
- Context management for AI conversations
- Tool integration and execution
- Multi-agent embedding generation and search
- Agent and model configuration
- Webhook processing

## Embedding System

The REST API provides a sophisticated multi-agent embedding system:

- Agent-specific model configuration
- Smart routing between providers (OpenAI, Bedrock, Google)
- Cross-model search with dimension normalization
- Cost tracking and optimization
- Circuit breaker pattern for resilience

Configuration requires at least one embedding provider. See `.env.example` for setup.

## API Endpoints

### Core APIs
- `/api/contexts` - Context management
- `/api/tools` - Tool integration
- `/api/embeddings` - Multi-agent embeddings
- `/api/agents` - Agent configuration
- `/api/models` - Model management
- `/api/search` - Semantic search

### Health & Monitoring
- `/health` - Service health check
- `/metrics` - Prometheus metrics

## Configuration

The service is configured via YAML files in the `configs/` directory:

```yaml
server:
  port: 8081
  mode: production

database:
  host: localhost
  port: 5432
  name: devops_mcp
  
embedding:
  providers:
    openai:
      enabled: true
      api_key: ${OPENAI_API_KEY}
```

## Running Locally

```bash
# Build
make build

# Run migrations
make migrate-local

# Start service
./api

# Or use Docker
docker build -t rest-api .
docker run -p 8081:8081 rest-api
```

## Testing

```bash
# Unit tests
make test

# Integration tests (requires DB)
make test-integration

# Coverage
make test-coverage
```

## Documentation

- [API Reference](../../docs/api-reference/rest-api-reference.md)
- [Embedding API](../../docs/api-reference/embedding-api-reference.md)
- [Configuration Guide](../../docs/operations/configuration-guide.md)
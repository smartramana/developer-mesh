# DevOps MCP System Architecture

## Overview

DevOps MCP (Model Context Protocol) is a production-ready platform that bridges AI agents with DevOps tools. Built using Go workspaces for modularity, it provides a unified API for tool integration, context management, and semantic search capabilities.

## Architecture Principles

- **Microservices Architecture**: Three independent services communicating via APIs and events
- **Clean Architecture**: Clear separation between business logic, adapters, and infrastructure
- **Event-Driven Design**: Asynchronous processing for scalability and resilience
- **Go Workspace**: Monorepo with multiple modules for code sharing and independent deployment
- **Cloud-Native**: Designed for containerized deployment with AWS service integration

## System Components

### 🔵 MCP Server (`apps/mcp-server`)

The MCP Server implements the Model Context Protocol specification:

- **Protocol Implementation**: Handles MCP protocol messages and routing
- **Tool Management**: Registers and manages available DevOps tools
- **Context Coordination**: Manages conversation contexts across tools
- **Event Publishing**: Publishes events for asynchronous processing

Key Features:
- WebSocket support for real-time communication
- Tool discovery and capability negotiation
- Session management with context persistence

### 🟢 REST API Service (`apps/rest-api`)

The REST API provides HTTP endpoints for external integrations:

- **Resource Management**: CRUD operations for contexts, agents, and models
- **Vector Operations**: Semantic search and embedding management
- **Webhook Handling**: Processes GitHub and other tool webhooks
- **API Gateway**: Unified entry point for all HTTP-based operations

Key Features:
- OpenAPI 3.0 specification
- Rate limiting and authentication
- CORS support for browser-based clients
- Pagination and filtering

### 🟠 Worker Service (`apps/worker`)

The Worker processes asynchronous tasks:

- **Event Processing**: Consumes events from SQS queues
- **Background Jobs**: Long-running operations and batch processing
- **Retry Logic**: Exponential backoff for failed operations
- **Idempotency**: Ensures operations are processed exactly once

Key Features:
- Dead letter queue handling
- Concurrent processing with configurable workers
- Circuit breaker pattern for external services

### 📦 Shared Libraries (`pkg/`)

Reusable packages across all services:

```
pkg/
├── adapters/       # External service integrations
├── common/         # Shared utilities and types
├── database/       # Database abstractions and migrations
├── embedding/      # Vector embedding services
├── models/         # Domain models and entities
├── observability/  # Logging, metrics, tracing
└── repository/     # Data access patterns
```

## Data Architecture

### Primary Storage

**PostgreSQL 14+**
- Relational data storage
- pgvector extension for embeddings
- JSONB for flexible schemas
- Row-level security support

**Redis 6.2+**
- Response caching
- Session management
- Distributed locks
- Rate limiting counters

### Object Storage

**AWS S3 (Optional)**
- Large context storage
- File attachments
- Backup and archival

### Message Queue

**AWS SQS**
- Event distribution
- Task queuing
- Dead letter queues
- FIFO support

## Data Flow Patterns

### 1. Synchronous Request Flow

```
Client → REST API → Repository → Database
         ↓
         Response ← Adapter ← Repository
```

### 2. Event-Driven Flow

```
API → Event Bus → SQS → Worker → External Service
                         ↓
                    Database Update
```

### 3. Vector Search Flow

```
Query → Embedding Service → Vector DB (pgvector)
                              ↓
                         Similarity Search
                              ↓
                         Ranked Results
```

## Integration Patterns

### Adapter Pattern

All external integrations use the adapter pattern:

```go
type ToolAdapter interface {
    Execute(ctx context.Context, action string, params map[string]interface{}) (interface{}, error)
    GetCapabilities() []Capability
}
```

Benefits:
- Isolation of external dependencies
- Consistent interface across tools
- Easy testing with mocks
- Gradual migration support

### Repository Pattern

Data access follows the repository pattern:

```go
type Repository[T any] interface {
    Create(ctx context.Context, entity T) (T, error)
    Get(ctx context.Context, id string) (T, error)
    List(ctx context.Context, filter Filter) ([]T, error)
    Update(ctx context.Context, entity T) (T, error)
    Delete(ctx context.Context, id string) error
}
```

## Security Architecture

### Authentication & Authorization

- **JWT Tokens**: Stateless authentication
- **API Keys**: Service-to-service communication
- **OAuth 2.0**: Third-party integrations
- **RBAC**: Role-based access control

### Data Protection

- **Encryption at Rest**: Database and S3 encryption
- **Encryption in Transit**: TLS 1.3 minimum
- **Secrets Management**: AWS Secrets Manager integration
- **Audit Logging**: All access logged and monitored

## Observability

### Metrics (Prometheus)

- Request rates and latencies
- Error rates by endpoint
- Queue depths and processing times
- Resource utilization

### Tracing (OpenTelemetry)

- Distributed request tracing
- Cross-service correlation
- Performance bottleneck identification

### Logging (Structured)

- JSON-formatted logs
- Contextual information
- Log aggregation support
- Configurable log levels

## Deployment Architecture

### Local Development

```yaml
docker-compose:
  - postgres (with pgvector)
  - redis
  - localstack (SQS)
  - Services (hot reload)
```

### Production (AWS)

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   ALB/NLB   │────▶│  ECS Tasks  │────▶│     RDS     │
└─────────────┘     └─────────────┘     └─────────────┘
                           │                     
                           ▼                     
                    ┌─────────────┐     ┌─────────────┐
                    │     SQS     │     │ ElastiCache │
                    └─────────────┘     └─────────────┘
```

### Kubernetes (Future)

- Helm charts for deployment
- Horizontal pod autoscaling
- Service mesh integration
- GitOps workflow

## Performance Considerations

### Caching Strategy

1. **Redis Cache**: Frequently accessed data
2. **Application Cache**: In-memory LRU cache
3. **CDN**: Static assets and API responses

### Database Optimization

1. **Connection Pooling**: Configurable pool sizes
2. **Query Optimization**: Explain plans and indexes
3. **Partitioning**: Time-based partitions for events

### Scalability

1. **Horizontal Scaling**: All services are stateless
2. **Queue-Based Decoupling**: Async processing
3. **Rate Limiting**: Protect against overload

## Resilience Patterns

### Circuit Breakers
Prevent cascading failures from external services

### Retry Logic
Exponential backoff with jitter

### Bulkheads
Isolate failures to specific components

### Health Checks
Liveness and readiness probes

## Future Architecture Considerations

1. **GraphQL Gateway**: Unified query interface
2. **Event Sourcing**: Complete audit trail
3. **CQRS**: Separate read/write models
4. **Multi-Region**: Geographic distribution
5. **Service Mesh**: Advanced traffic management

## References

- [Go Workspace Structure](go-workspace-structure.md)
- [Adapter Pattern Implementation](adapter-pattern.md)
- [API Documentation](../api-reference/vector-search-api.md)
- [Development Environment](../developer/development-environment.md)
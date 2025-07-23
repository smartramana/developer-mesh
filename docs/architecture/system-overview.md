# Developer Mesh System Architecture

## Overview

Developer Mesh is an AI Agent Orchestration Platform that enables intelligent routing and coordination of multiple AI agents for DevOps workflows. Built using Go workspaces for modularity, it provides sophisticated multi-agent orchestration, real-time collaboration features, and production-ready AWS integrations.

## Architecture Principles

- **Microservices Architecture**: Three independent services communicating via APIs and events
- **AI-Native Design**: Built from the ground up for multi-agent orchestration and coordination
- **Clean Architecture**: Clear separation between business logic, adapters, and infrastructure
- **Event-Driven Design**: Real-time WebSocket communication with asynchronous task processing
- **Go Workspace**: Monorepo with multiple modules for code sharing and independent deployment
- **Cloud-Native**: Production AWS integration with Bedrock, SQS, S3, and ElastiCache

## System Components

### 🔵 MCP Server (`apps/mcp-server`)

The MCP Server is the core AI agent orchestration hub:

- **Agent Registry**: Manages AI agent registration, capabilities, and real-time status
- **Task Assignment Engine**: Intelligent routing with multiple strategies (capability-match, least-loaded, cost-optimized)
- **Binary WebSocket Protocol**: High-performance communication with compression support
- **Multi-Agent Collaboration**: Orchestrates complex workflows across multiple AI agents

Key Features:
- Binary WebSocket protocol with automatic gzip compression (>1KB messages)
- Real-time agent discovery and capability-based routing
- Workload tracking and dynamic load balancing
- Task delegation and collaboration patterns (MapReduce, parallel, pipeline)
- Circuit breaker pattern for resilient agent communication

### 🟢 REST API Service (`apps/rest-api`)

The REST API provides HTTP endpoints for external integrations:

- **Agent Management**: Register agents, query capabilities, monitor workload
- **Task Submission**: Submit tasks with routing preferences and requirements
- **Embedding Operations**: Generate and search embeddings via AWS Bedrock
- **Tool Integration**: GitHub adapter for DevOps workflow automation

Key Features:
- All endpoints use `/api/v1/*` path prefix
- Multi-model embedding support (Titan, Cohere, Claude)
- Cost tracking and optimization for AI operations
- JWT and API key authentication
- Comprehensive Swagger/OpenAPI documentation

### 🟠 Worker Service (`apps/worker`)

The Worker handles distributed task processing:

- **Task Distribution**: Processes tasks assigned to AI agents
- **Embedding Pipeline**: Batch processing for vector embeddings
- **Notification Delivery**: Sends real-time updates via WebSocket
- **Workflow Coordination**: Manages multi-step AI workflows

Key Features:
- SQS integration for reliable task delivery
- Concurrent processing with agent workload awareness
- Cost tracking for AI model usage
- Dead letter queue for failed task handling

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

**Redis 7+**
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

## Collaboration Features

### CRDT-Based Collaborative Editing

The platform includes advanced CRDT (Conflict-free Replicated Data Type) implementations for real-time collaboration:

- **DocumentCRDT**: Collaborative text editing with fractional indexing
- **StateCRDT**: Distributed state management with path-based updates
- **Vector Clocks**: Causality tracking for distributed operations
- **Implemented CRDTs**:
  - GCounter (grow-only counter)
  - PNCounter (increment/decrement counter)
  - LWWRegister (last-write-wins register)
  - ORSet (observed-remove set)

### Binary WebSocket Protocol

High-performance binary protocol for agent communication:

```
Header (12 bytes):
┌─────────┬───────┬──────────────┬──────────────┬──────────┐
│ Version │ Flags │ Message Type │ Payload Size │ Reserved │
│ 1 byte  │ 1 byte│   2 bytes    │   4 bytes    │  4 bytes │
└─────────┴───────┴──────────────┴──────────────┴──────────┘

Features:
- Automatic gzip compression for messages > 1KB
- Message batching for improved throughput
- Buffer pooling for reduced GC pressure
- Max payload size: ~4GB
- Max decompressed size: 10MB (security limit)
```

## Data Flow Patterns

### 1. AI Agent Registration Flow

```
Agent → WebSocket → MCP Server → Agent Registry
                         ↓
                    Capability Index
                         ↓
                    Task Router Update
```

### 2. Task Assignment Flow

```
Task Request → REST API → Assignment Engine → Agent Selection
                                    ↓
                            WebSocket Notification
                                    ↓
                               Agent Processing
```

### 3. Multi-Agent Collaboration Flow

```
Initiator Agent → Task Delegation → Agent Discovery
                         ↓
                  Capability Matching
                         ↓
                  Parallel Execution → Result Aggregation
```

### 4. Vector Embedding Flow

```
Content → Bedrock API → Embedding Generation
                              ↓
                         Cost Tracking
                              ↓
                    pgvector Storage → Similarity Search
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

- **JWT Tokens**: Stateless authentication (implemented)
- **API Keys**: Service-to-service communication (implemented)
- **OAuth 2.0**: Third-party integrations (interface defined, providers pending)
- **RBAC**: Role-based access control (Casbin planned, not yet implemented)

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
  - AWS SQS (production)
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

### AI Agent Performance

1. **Task Routing**: Optimized routing decisions with cached capabilities
2. **Binary Protocol**: Significant message size reduction with compression
3. **Connection Pooling**: Reusable WebSocket connections per agent
4. **Workload Balancing**: Real-time load distribution across agents

### Multi-Level Caching

1. **Memory Cache**: Hot embeddings and agent capabilities
2. **Redis Cache**: Distributed cache for agent state and embeddings
3. **Database Cache**: Persistent storage with pgvector indexes
4. **Cost Cache**: Model pricing data for routing decisions

### Embedding Optimization

1. **Batch Processing**: Reduce API calls to Bedrock
2. **Provider Failover**: Automatic switching on rate limits
3. **Quality/Cost Trade-offs**: Configurable routing strategies
4. **Cache Hit Rates**: Minimize regeneration costs

### Scalability

1. **Agent Scaling**: Designed for high concurrency with multiple AI agents
2. **Task Parallelization**: MapReduce patterns for large workloads
3. **Circuit Breakers**: Prevent cascade failures
4. **Queue Sharding**: Distribute load across SQS queues

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

1. **Advanced AI Orchestration**:
   - Hierarchical agent organizations
   - Learning-based task routing
   - Agent capability evolution
   - Multi-modal agent support

2. **Enhanced Collaboration**:
   - Full CRDT delta synchronization
   - Conflict resolution strategies
   - Real-time collaborative debugging
   - Agent consensus mechanisms

3. **Enterprise Features**:
   - Casbin RBAC implementation (planned)
   - OAuth provider integrations (pending implementation)
   - Advanced audit logging
   - Multi-tenant agent isolation

4. **Performance Enhancements**:
   - GPU-accelerated embeddings
   - Edge agent deployment
   - Predictive task scheduling
   - Adaptive compression algorithms

5. **Integration Expansion**:
   - Additional DevOps tool adapters
   - Cloud provider agnostic design
   - Kubernetes operator for agents
   - GitOps workflow automation

## References

- [Go Workspace Structure](go-workspace-structure.md)
- [Adapter Pattern Implementation](adapter-pattern.md)
- [API Documentation](../api-reference/vector-search-api.md)
- [Development Environment](../developer/development-environment.md)
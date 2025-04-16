# MCP Server Architecture

This document describes the architecture of the Model Context Protocol (MCP) Server, which provides both context management for AI agents and DevOps tool integrations.

## High-Level Architecture

The MCP Server is designed around a modular architecture that supports two primary functions:

1. **Context Management**: Storing, retrieving, and manipulating conversation contexts for AI agents
2. **DevOps Integration**: Providing a unified API for AI agents to interact with DevOps tools

![High-Level Architecture](../docs/images/mcp-architecture.png)

## Core Components

### 1. Core Engine

The Core Engine is the central processing unit of the MCP Server, responsible for:

- Coordinating between context management and DevOps tool adapters
- Managing events from various sources
- Providing a unified interface for the API server

The Engine maintains references to all adapters and the context manager, allowing it to route requests and ensure operations are properly recorded in contexts.

### 2. Context Manager

The Context Manager handles all aspects of context storage, retrieval, and manipulation:

- **Context Storage**: Persists contexts in the database and cache
- **Context Windowing**: Intelligently manages context size with various truncation strategies
- **Context Search**: Provides capabilities to search within contexts
- **Context Summarization**: Can generate summaries of contexts

The Context Manager is designed to be highly efficient, using a combination of database storage and in-memory caching to provide fast access to contexts.

### 3. Adapters

Adapters provide interfaces to external DevOps tools and services:

- **GitHub Adapter**: Integrates with GitHub's API and webhook system
- **Harness Adapter**: Integrates with Harness CI/CD and feature flags
- **SonarQube Adapter**: Integrates with SonarQube for code quality
- **Artifactory Adapter**: Integrates with JFrog Artifactory
- **Xray Adapter**: Integrates with JFrog Xray security scanning

Each adapter implements a common interface that includes:
- Getting data from the external service
- Executing actions with context awareness
- Handling webhooks from the external service
- Subscribing to specific event types

### 4. Adapter Context Integration

The Adapter Context Integration component bridges the gap between adapter operations and context management:

- Records all adapter operations in the relevant context
- Records webhook events in contexts
- Provides a unified interface for context-aware operations

This integration ensures that all interactions with external tools are properly recorded in the context, allowing AI agents to maintain a complete history of their actions.

### 5. API Server

The API Server provides HTTP endpoints for interacting with the MCP platform:

- **Context API**: Endpoints for managing contexts
- **Tool API**: Endpoints for interacting with DevOps tools
- **Webhook Handlers**: Process incoming webhooks from both agents and DevOps tools
- **Authentication**: JWT and API key-based authentication

The API Server uses the Gin framework for routing and middleware, with a structured approach to handling requests.

### 6. Database and Storage

The MCP Server uses a combination of database and object storage for optimal performance:

#### Database (PostgreSQL)

The Database component handles persistent storage for:

- **Context References**: Metadata and references to context data
- **Vector Embeddings**: Stores vector embeddings for semantic search
- **System Configuration**: Stores system-wide configuration
- **Metrics**: Stores historical metrics for monitoring

The implementation uses PostgreSQL with the pg_vector extension for vector operations, with appropriate indexes and optimizations.

#### Object Storage (S3)

The S3 Storage component provides:

- **Large Context Storage**: Efficiently stores large context data
- **Scalable Storage**: Handles growing context data without database constraints
- **Cost-Efficient Storage**: Reduces database storage costs
- **Flexible Implementation**: Works with AWS S3 or any S3-compatible service

The S3 implementation supports multipart uploads/downloads, server-side encryption, and optimized concurrent operations.

### 7. Cache

The Cache component provides fast access to frequently accessed data:

- **Context Caching**: Caches contexts to reduce database load
- **Tool Response Caching**: Caches responses from external tools
- **Rate Limiting**: Supports rate limiting for API endpoints

The current implementation uses Redis for distributed caching, with configurable TTLs and eviction policies.

## Data Flow

### Context Management Flow

1. An AI agent requests to create a new context via the Context API
2. The API Server routes the request to the Context Manager
3. The Context Manager creates the context in the database
4. The context is cached for fast access
5. The context ID is returned to the agent

For subsequent operations:
1. The agent sends messages or other content to add to the context
2. The Context Manager updates the context in the database and cache
3. If the context exceeds its maximum token limit, the Context Manager applies the configured truncation strategy

### Tool Integration Flow

1. An AI agent requests to execute an action on a DevOps tool via the Tool API
2. The API Server routes the request to the Core Engine
3. The Core Engine identifies the appropriate adapter and calls the ExecuteAdapterAction method
4. The adapter executes the action on the external service
5. The Adapter Context Integration component records the operation in the context
6. The result is returned to the agent

### Webhook Flow

1. A DevOps tool sends a webhook to the MCP Server
2. The appropriate webhook handler validates the webhook signature
3. The handler routes the webhook to the corresponding adapter
4. The adapter processes the webhook and generates events
5. If an agent ID is available, the webhook is recorded in the relevant context

## Security Model

The MCP Server implements a comprehensive security model:

1. **Authentication**:
   - JWT-based authentication for API endpoints
   - API key authentication for monitoring endpoints
   - Webhook signature verification

2. **Authorization**:
   - Role-based access control for API endpoints
   - Tool-specific permissions

3. **Safety Restrictions**:
   - GitHub: Can archive repositories but cannot delete them
   - Artifactory: Read-only access (no upload or delete capabilities)
   - Harness: Cannot delete production feature flags or other critical resources
   - Safety checks automatically block dangerous operations
   - All operations are audited and logged

4. **Data Protection**:
   - TLS encryption for all communications
   - Secure storage of credentials and secrets

## Deployment Architecture

The MCP Server is designed to be deployed in various configurations:

1. **Single-Node Deployment**:
   - All components running on a single server
   - Suitable for development and small-scale deployments

2. **Distributed Deployment**:
   - API Server, Database, and Cache running on separate servers
   - Horizontal scaling for high availability
   - Load balancing for the API Server

3. **Containerized Deployment**:
   - Docker Compose for development
   - Kubernetes for production deployments
   - Helm charts for easy deployment

## Performance Considerations

The MCP Server is designed for high performance:

1. **Caching Strategy**:
   - Multi-level caching (memory, Redis)
   - Intelligent cache invalidation
   - Configurable TTLs

2. **Database Optimization**:
   - Connection pooling
   - Prepared statements
   - Appropriate indexing

3. **Concurrency Management**:
   - Worker pools with configurable limits
   - Non-blocking I/O where possible
   - Efficient context windowing algorithms

## Monitoring and Observability

The MCP Server provides comprehensive monitoring and observability:

1. **Metrics**:
   - Request counts, latencies, and error rates
   - Context operations and sizes
   - Tool interactions

2. **Logging**:
   - Structured JSON logging
   - Configurable log levels
   - Request ID tracking

3. **Tracing**:
   - Distributed tracing with OpenTelemetry
   - Span propagation across components

4. **Health Checks**:
   - Component-level health checks
   - Dependency health monitoring

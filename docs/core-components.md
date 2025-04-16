# MCP Server Core Components

This document provides a detailed description of the core components that make up the MCP Server.

## 1. Core Engine

The Core Engine is the central processing unit of the MCP Server, responsible for managing events and orchestrating workflows between different systems.

### Responsibilities

- Event reception and processing
- Adapter management and initialization
- Event subscription and notification
- Health monitoring of integrated systems

### Key Features

- **Concurrent Processing**: Uses Go's goroutines to handle multiple events simultaneously with configurable concurrency limits
- **Graceful Shutdown**: Ensures all events are processed before shutdown
- **Error Handling**: Comprehensive error handling with retries and circuit breakers
- **Event Timeout**: Configurable timeouts for event processing

### Implementation Details

The Core Engine is implemented in `internal/core/engine.go` and consists of:

- **Engine struct**: Main structure holding configuration, adapters, and runtime state
- **Event Channel**: Buffered channel for receiving events from adapters
- **Worker Pool**: Goroutine pool for processing events concurrently
- **Adapter Registry**: Map of initialized adapters by name
- **Subscription System**: Callback registry for event notifications

### Initialization

The Engine is initialized by the `NewEngine` function, which:

1. Creates a new context for the engine
2. Initializes all configured adapters
3. Sets up event handlers for each adapter
4. Starts the event processing goroutines
5. Returns the initialized engine

### Event Flow

1. Events are received from adapters or webhook handlers
2. Events are placed in the buffered channel
3. Worker goroutines pick up events for processing
4. Events are processed based on their source and type
5. Results are stored or forwarded as needed

## 2. API Server

The API Server provides HTTP endpoints for interacting with the MCP platform, including webhook endpoints, REST API, and monitoring endpoints.

### Responsibilities

- Handling incoming HTTP requests
- Authentication and authorization
- Request validation and rate limiting
- Webhook processing
- Health and metrics exposure

### Key Features

- **Middleware Support**: Logging, CORS, authentication, metrics collection
- **Webhook Validation**: Signature verification for all webhook endpoints
- **Rate Limiting**: Configurable rate limiting with Redis backend
- **TLS Support**: Optional TLS encryption for all endpoints

### Implementation Details

The API Server is implemented in `internal/api/server.go` and related files:

- **Server struct**: Main structure holding router, handlers, and configuration
- **Gin Framework**: Uses Gin for HTTP routing and middleware
- **Middleware Chain**: Configurable middleware stack for request processing
- **Route Registry**: Dynamic route registration based on configuration

### Endpoints

- **API Endpoints**: `/api/v1/...` for MCP protocol operations
- **Webhook Endpoints**: `/webhook/...` for receiving events from integrated systems
- **Health Endpoint**: `/health` for system health checks
- **Metrics Endpoint**: `/metrics` for Prometheus metrics

## 3. Adapters

Adapters provide the interface between the MCP Server and external systems, handling communication and translation between different APIs.

### Common Adapter Interface

All adapters implement the common interface defined in `internal/adapters/adapter.go`:

```go
type Adapter interface {
    // Initialize sets up the adapter with configuration
    Initialize(ctx context.Context, config interface{}) error

    // GetData retrieves data from the external service
    GetData(ctx context.Context, query interface{}) (interface{}, error)

    // Health returns the health status of the adapter
    Health() string

    // Close gracefully shuts down the adapter
    Close() error
}
```

### Adapter Base Implementation

The `BaseAdapter` struct provides common functionality for all adapters:

- Retry logic with exponential backoff
- Error classification (retryable vs. non-retryable)
- Common configuration parameters

### GitHub Adapter

The GitHub adapter (`internal/adapters/github/github.go`) integrates with GitHub's API and webhook system:

- **Events Supported**: Pull requests, pushes, and other GitHub events
- **API Interactions**: Repository data, pull requests, issues, etc.
- **Mock Mode**: Support for development without real GitHub credentials

### Harness Adapter

The Harness adapter integrates with Harness CI/CD and feature flags:

- **Events Supported**: CI builds, CD deployments, STO experiments, feature flags
- **API Interactions**: Pipeline data, build information, deployment status, etc.
- **Mock Mode**: Simulated Harness responses for testing

### SonarQube Adapter

The SonarQube adapter integrates with SonarQube code quality platform:

- **Events Supported**: Quality gate status, analysis completion
- **API Interactions**: Project data, quality metrics, issues, etc.
- **Mock Mode**: Simulated SonarQube responses for testing

### Artifactory Adapter

The Artifactory adapter integrates with JFrog Artifactory:

- **Events Supported**: Artifact creation, deletion, property changes
- **API Interactions**: Repository data, artifact information, storage stats, etc.
- **Mock Mode**: Simulated Artifactory responses for testing

### Xray Adapter

The Xray adapter integrates with JFrog Xray security scanning:

- **Events Supported**: Security violations, license violations, scan completion
- **API Interactions**: Vulnerability data, license information, scan results, etc.
- **Mock Mode**: Simulated Xray responses for testing

## 4. Storage Components

The Storage components handle persistent storage for the MCP Server, using a combination of relational database and object storage.

### Storage Interface

The storage interface provides abstract operations for data storage and retrieval:

- CRUD operations for various entity types
- Query building and execution
- Transaction support
- Migration handling
- Large object storage and retrieval

### PostgreSQL Database

The PostgreSQL database stores:

- **Context References**: Metadata and references to context data
- **Vector Embeddings**: Vector data for semantic search (using pg_vector extension)
- **Event Records**: Historical record of processed events
- **Adapter Configurations**: Stored configuration for adapters
- **Integration Mappings**: Mappings between entities in different systems
- **User Data**: User accounts and authorization information

Key features include:
- Connection pooling for performance
- Prepared statements to prevent SQL injection
- Schema migrations for versioning
- Transaction support for atomic operations
- Vector operations for semantic search

### S3 Object Storage

The S3 storage component provides efficient storage for large context data:

- **Multi-part Upload/Download**: Efficient transfer of large context data
- **Concurrent Operations**: Parallel processing for better performance
- **Server-side Encryption**: Data protection at rest
- **Compatibility**: Works with AWS S3 or any S3-compatible service
- **Scalability**: Virtually unlimited storage capacity

Implementation details:
- AWS SDK for Go v2 for S3 operations
- Configurable part sizes for multipart operations
- Configurable concurrency for parallel processing
- Automatic retry mechanisms for resilience
- Support for various S3 security features

## 5. Cache Components

The Cache components provide fast access to frequently accessed data.

### Cache Interface

The cache interface defines common operations for all cache implementations:

- Get/Set operations with TTL
- Batch operations for efficiency
- Invalidation patterns
- Health checks

### Redis Implementation

The current implementation uses Redis as the distributed cache:

- Connection pooling for performance
- Configurable TTLs for different data types
- Pattern-based invalidation
- Pipeline support for batch operations

### Caching Strategies

Different caching strategies are used for different data types:

- **API Response Caching**: Caching responses from external APIs
- **Configuration Caching**: Caching configuration data for quick access
- **Entity Caching**: Caching frequently accessed entities
- **Rate Limit Storage**: Using cache for distributed rate limiting

## 6. Vector Search Components

The Vector Search components provide semantic search capabilities using vector embeddings.

### Vector Repository

The Vector Repository manages the storage and retrieval of vector embeddings:

- Storage of vector embeddings in PostgreSQL
- Efficient similarity search using cosine distance
- Indexing for fast retrieval
- Batch operations for efficiency

### Vector API

The Vector API provides endpoints for vector operations:

- Store embeddings for context items
- Search for similar embeddings based on vector similarity
- Retrieve embeddings for a context
- Delete embeddings when contexts are removed

### Implementation Details

The vector search functionality is implemented using:

- PostgreSQL pg_vector extension for vector operations
- Efficient indexing methods (IVF, HNSW)
- Optimized query patterns for similarity search
- Hybrid approach where agents generate embeddings and MCP Server handles storage and search

## 7. Metrics Components

The Metrics components collect and expose system performance metrics.

### Metrics Interface

The metrics interface defines common operations for metrics collection:

- Counter, gauge, and histogram metrics
- Custom metric registration
- Label support for multi-dimensional metrics
- Metrics collection middleware

### Prometheus Implementation

The current implementation uses Prometheus for metrics collection:

- HTTP endpoint for scraping metrics
- Integration with Grafana for visualization
- Custom metrics for MCP-specific operations
- Auto-instrumentation for common operations

### Key Metrics

The system collects various metrics:

- **API Metrics**: Request counts, response times, error rates
- **Adapter Metrics**: API call counts, response times, error rates
- **Event Metrics**: Event counts by source and type, processing times
- **System Metrics**: CPU, memory, goroutine counts, GC stats
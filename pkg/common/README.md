# Common Package

## Overview

The `common` package provides shared utilities, AWS clients, and foundational components used throughout the DevOps MCP platform. It includes production-ready implementations for caching, configuration, error handling, observability, and AWS service integration.

## Package Structure

```
common/
├── aws/           # AWS service clients and authentication
├── cache/         # Multi-level caching implementations
├── config/        # Configuration management
├── errors/        # Error types and handling
├── events/        # Event bus and async messaging
├── logging/       # Structured logging
├── metrics/       # Metrics collection
├── observability/ # Tracing and monitoring
├── util/          # General utilities
└── vector_utils.go # Vector operations for embeddings
```

## AWS Integration

### Client Interface

The AWS package provides a unified client for all AWS services:

```go
// Client provides access to AWS services
type Client interface {
    S3() S3Client
    SQS() SQSClient
    Bedrock() BedrockClient
    RDS() RDSClient
    ElastiCache() ElastiCacheClient
}

// Initialize with configuration
client, err := aws.NewClient(cfg)
```

### Authentication

Supports multiple authentication methods:

```go
// IAM role authentication (preferred for production)
cfg := aws.Config{
    UseIAM: true,
    Region: "us-east-1",
}

// IRSA (IAM Roles for Service Accounts) - auto-detected
// Automatically uses Web Identity Token if available

// Standard credentials (development)
cfg := aws.Config{
    AccessKeyID:     "...",
    SecretAccessKey: "...",
}
```

### S3 Operations

```go
// Upload file
err := client.S3().PutObject(ctx, "bucket", "key", data)

// Download file
data, err := client.S3().GetObject(ctx, "bucket", "key")

// Delete file
err := client.S3().DeleteObject(ctx, "bucket", "key")

// List objects
objects, err := client.S3().ListObjects(ctx, "bucket", "prefix/")
```

### SQS Operations

```go
// Send message
err := client.SQS().SendMessage(ctx, "queue-url", message)

// Receive messages
messages, err := client.SQS().ReceiveMessages(ctx, "queue-url", 10)

// Delete message
err := client.SQS().DeleteMessage(ctx, "queue-url", receiptHandle)
```

### ElastiCache Configuration

```go
// Redis with IAM authentication
cfg := ElastiCacheConfig{
    Endpoint:      "cluster.cache.amazonaws.com:6379",
    UseIAM:        true,
    ClusterMode:   true,
    PoolSize:      100,
    MinIdleConns:  10,
}

client := NewElastiCacheClient(cfg)
```

## Caching

### Multi-Level Cache Architecture

```go
// Cache interface
type Cache interface {
    Get(ctx context.Context, key string, dest interface{}) error
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Exists(ctx context.Context, key string) (bool, error)
}

// Initialize multi-level cache
cache := cache.NewMultiLevelCache(
    cache.NewMemoryCache(1000),     // L1: In-memory
    cache.NewRedisCache(redisClient), // L2: Redis
)
```

### Cache Implementations

1. **Memory Cache**: Fast in-process caching
2. **Redis Cache**: Distributed caching with cluster support
3. **Multi-Level Cache**: Hierarchical caching with fallback
4. **Noop Cache**: For testing or disabled caching

### Usage Examples

```go
// Set with TTL
err := cache.Set(ctx, "user:123", user, 5*time.Minute)

// Get with type safety
var user User
err := cache.Get(ctx, "user:123", &user)

// Check existence
exists, err := cache.Exists(ctx, "user:123")

// Delete
err := cache.Delete(ctx, "user:123")
```

### Cache Metrics

Automatic metrics collection:
- Hit/miss rates
- Operation latencies
- Error counts
- Cache size

## Configuration

### Loading Configuration

```go
// Load from environment and config files
cfg, err := config.Load()

// With custom paths
cfg, err := config.LoadFrom("./configs", "production")

// Access nested values
dbHost := cfg.GetString("database.host")
cacheEnabled := cfg.GetBool("cache.enabled")
```

### Environment Variables

All configuration can be overridden via environment variables:

```bash
# Database configuration
DB_HOST=localhost
DB_PORT=5432
DB_NAME=devops_mcp

# Redis configuration  
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_USE_CLUSTER=true

# AWS configuration
AWS_REGION=us-east-1
AWS_S3_BUCKET=my-bucket
```

### Configuration Structures

```go
// Database configuration
type DatabaseConfig struct {
    Host            string
    Port            int
    Database        string
    Username        string
    Password        string
    SSLMode         string
    MaxConnections  int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
}

// Monitoring configuration
type MonitoringConfig struct {
    MetricsEnabled bool
    TracingEnabled bool
    LogLevel       string
    ServiceName    string
}
```

## Error Handling

### Error Types

```go
// Base error with context
type AdapterError struct {
    Code       string
    Message    string
    Details    map[string]interface{}
    Retryable  bool
    StatusCode int
}

// Common errors
var (
    ErrNotFound     = NewError("NOT_FOUND", "Resource not found", 404)
    ErrUnauthorized = NewError("UNAUTHORIZED", "Unauthorized access", 401)
    ErrRateLimit    = NewError("RATE_LIMIT", "Rate limit exceeded", 429)
)
```

### Error Wrapping

```go
// Wrap with context
err := WrapError(originalErr, "failed to process task",
    "task_id", taskID,
    "retry_count", retryCount,
)

// Check error type
if IsNotFound(err) {
    // Handle not found
}

// Check if retryable
if IsRetryable(err) {
    // Retry operation
}
```

## Event System

### Event Bus

Asynchronous event processing with typed events:

```go
// Initialize event bus
bus := events.NewEventBus()

// Register handler
bus.Subscribe("task.completed", func(ctx context.Context, event Event) error {
    task := event.Data.(*Task)
    // Process completed task
    return nil
})

// Publish event
err := bus.Publish(ctx, Event{
    Type: "task.completed",
    Data: task,
    Metadata: map[string]interface{}{
        "duration": time.Since(startTime),
    },
})
```

### System Events

Pre-defined system events:

```go
// Common event types
const (
    EventAgentRegistered = "agent.registered"
    EventAgentDisconnected = "agent.disconnected"
    EventTaskAssigned = "task.assigned"
    EventTaskCompleted = "task.completed"
    EventWorkflowStarted = "workflow.started"
    EventWorkflowCompleted = "workflow.completed"
)
```

## Logging

### Structured Logging

```go
// Initialize logger
logger := logging.NewLogger("service-name")

// Log with fields
logger.Info("Processing task",
    "task_id", task.ID,
    "agent_id", agent.ID,
    "attempt", retryCount,
)

// Log levels
logger.Debug("Detailed information")
logger.Info("General information")
logger.Warn("Warning condition")
logger.Error("Error occurred", "error", err)
```

### Log Configuration

```go
// Set log level
logger.SetLevel(logging.LevelDebug)

// JSON output for production
logger.SetFormatter(&logrus.JSONFormatter{})

// Human-readable for development
logger.SetFormatter(&logrus.TextFormatter{
    FullTimestamp: true,
})
```

## Observability

### Distributed Tracing

```go
// Initialize tracer
tracer, err := observability.NewTracer("service-name")

// Create span
ctx, span := tracer.Start(ctx, "operation-name",
    trace.WithAttributes(
        attribute.String("task.id", taskID),
        attribute.Int("retry.count", retryCount),
    ),
)
defer span.End()

// Record events
span.AddEvent("cache_miss", trace.WithAttributes(
    attribute.String("cache.key", key),
))

// Set status
span.SetStatus(codes.Error, "operation failed")
```

### Metrics Collection

```go
// Initialize metrics client
metrics := metrics.NewPrometheusClient()

// Counter
taskCounter := metrics.NewCounter("tasks_total",
    "Total number of tasks processed",
    []string{"status", "type"},
)
taskCounter.WithLabelValues("success", "embedding").Inc()

// Histogram
latencyHist := metrics.NewHistogram("operation_duration_seconds",
    "Operation duration in seconds",
    []string{"operation"},
)
latencyHist.WithLabelValues("embed").Observe(duration.Seconds())

// Gauge
activeAgents := metrics.NewGauge("agents_active",
    "Number of active agents",
    []string{"capability"},
)
activeAgents.WithLabelValues("compute").Set(float64(count))
```

## Vector Utilities

Operations for embedding vectors:

```go
// Normalize vector to unit length
normalized := NormalizeL2(vector)

// Calculate similarity
dotProduct := DotProduct(vec1, vec2)
cosineDist := CosineDistance(vec1, vec2)
euclideanDist := EuclideanDistance(vec1, vec2)

// Convert to pgvector format
pgVector := ToPGVector(embeddings)

// Parse from pgvector format
embeddings, err := FromPGVector(pgVector)
```

## Utilities

### UUID Generation

```go
// Generate new UUID
id := util.NewUUID()

// Parse UUID string
id, err := util.ParseUUID("550e8400-e29b-41d4-a716-446655440000")

// Validate UUID
if util.IsValidUUID(str) {
    // Valid UUID format
}
```

### Tenant Management

```go
// Extract tenant from context
tenantID := util.GetTenant(ctx)

// Add tenant to context
ctx = util.WithTenant(ctx, tenantID)

// Check tenant access
if !util.HasTenantAccess(ctx, resourceTenantID) {
    return ErrUnauthorized
}
```

## Best Practices

### 1. Error Handling

Always wrap errors with context:
```go
if err != nil {
    return WrapError(err, "failed to process",
        "operation", "embed",
        "model", model,
    )
}
```

### 2. Context Propagation

Pass context through all operations:
```go
// Propagate context for cancellation and tracing
result, err := cache.Get(ctx, key, &data)
```

### 3. Metrics and Logging

Add observability to critical paths:
```go
start := time.Now()
defer func() {
    duration := time.Since(start)
    histogram.Observe(duration.Seconds())
    logger.Info("Operation completed",
        "duration", duration,
        "success", err == nil,
    )
}()
```

### 4. Resource Cleanup

Always clean up resources:
```go
client, err := NewClient(cfg)
if err != nil {
    return err
}
defer client.Close()
```

## Testing

### Mock Implementations

```go
// Use mock cache for testing
mockCache := cache.NewMockCache()
mockCache.On("Get", "key").Return(data, nil)

// Use mock metrics
mockMetrics := metrics.NewMockClient()
mockMetrics.AssertCalled(t, "IncrementCounter", "tasks_total")
```

### Test Utilities

```go
// Test with temporary config
cfg := config.NewTestConfig()
cfg.Set("redis.host", "localhost")

// Test with in-memory cache
cache := cache.NewMemoryCache(100)
```

## Performance Considerations

- **Caching**: Use multi-level caching for hot data
- **Connection Pooling**: Configure appropriate pool sizes
- **Batch Operations**: Use batch APIs when available
- **Circuit Breakers**: Protect against cascading failures
- **Metrics Sampling**: Sample high-frequency metrics

## Migration Guide

When updating common utilities:

1. Check for breaking changes in interfaces
2. Update all dependent packages
3. Run integration tests
4. Monitor metrics after deployment

## References

- [AWS SDK Documentation](https://aws.github.io/aws-sdk-go-v2/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
- [Prometheus Client Library](https://github.com/prometheus/client_golang)
- [Redis Go Client](https://github.com/go-redis/redis)
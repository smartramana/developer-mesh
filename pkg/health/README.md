# Health Package

> **Purpose**: Health check system for monitoring service readiness and liveness
> **Status**: Basic Implementation
> **Dependencies**: Database, Redis, AWS service checks

## Overview

The health package provides a basic health check system that monitors service dependencies and tracks component status. The actual implementation is simpler than the comprehensive system described in this documentation.

**Implemented Features**:
- Basic health checker with concurrent check execution
- Database, Redis, S3, SQS health checks
- Simple aggregated health status
- Background periodic checks
- Basic metrics recording

**Not Yet Implemented** (but documented below):
- Separate liveness/readiness/startup check types
- Health check caching
- Circuit breaker integration
- HTTP/gRPC health endpoints
- Kubernetes integration
- Advanced monitoring and metrics

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Health Check System                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Health Manager ──► Component Checks ──► Aggregated Status │
│        │                   │                     │          │
│        │                   ├── Database         │          │
│        │                   ├── Redis            │          │
│        │                   ├── AWS Services     │          │
│        │                   └── Custom Checks    │          │
│        │                                         │          │
│        └──► HTTP/gRPC Endpoints ◄───────────────┘          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Actual Implementation

### 1. Health Check Interface (Simplified)

```go
// HealthCheck interface for individual health checks
type HealthCheck interface {
    Name() string
    Check(ctx context.Context) error
}

// Check represents a single health check result
type Check struct {
    Name        string                 `json:"name"`
    Status      Status                 `json:"status"`
    Message     string                 `json:"message,omitempty"`
    LastChecked time.Time              `json:"last_checked"`
    Duration    time.Duration          `json:"duration_ms"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Status represents health status
type Status string

const (
    StatusHealthy   Status = "healthy"
    StatusUnhealthy Status = "unhealthy"
    StatusDegraded  Status = "degraded"
)

// Note: CheckType (liveness/readiness/startup) is NOT implemented
```

### 2. Health Checker (Actual Implementation)

```go
// HealthChecker manages and executes health checks
type HealthChecker struct {
    checks  map[string]HealthCheck
    results map[string]*Check
    mu      sync.RWMutex

    metrics observability.MetricsClient
    logger  observability.Logger

    // Configuration
    checkInterval time.Duration
    timeout       time.Duration
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger observability.Logger, metrics observability.MetricsClient) *HealthChecker {
    return &HealthChecker{
        checks:        make(map[string]HealthCheck),
        results:       make(map[string]*Check),
        metrics:       metrics,
        logger:        logger,
        checkInterval: 30 * time.Second,
        timeout:       5 * time.Second,
    }
}

// RegisterCheck registers a new health check
func (h *HealthChecker) RegisterCheck(name string, check HealthCheck)

// RunChecks executes all registered health checks concurrently
func (h *HealthChecker) RunChecks(ctx context.Context) map[string]*Check

// IsHealthy returns true if all checks are healthy
func (h *HealthChecker) IsHealthy() bool

// GetAggregatedHealth returns the overall health status
func (h *HealthChecker) GetAggregatedHealth() *AggregatedHealth
```

**Note**: No caching, circuit breakers, or check type filtering is implemented.

### 3. Implemented Health Checks

#### Database Health Check

```go
// DatabaseHealthCheck checks database connectivity
type DatabaseHealthCheck struct {
    db       *sql.DB
    timeout  time.Duration
    query    string
}

func NewDatabaseHealthCheck(db *sql.DB) *DatabaseHealthCheck {
    return &DatabaseHealthCheck{
        db:      db,
        timeout: 5 * time.Second,
        query:   "SELECT 1",
    }
}

func (c *DatabaseHealthCheck) Check(ctx context.Context) *CheckResult {
    result := &CheckResult{
        Timestamp: time.Now(),
        Details:   make(map[string]interface{}),
    }
    
    // Test connection
    start := time.Now()
    err := c.db.PingContext(ctx)
    pingDuration := time.Since(start)
    
    if err != nil {
        result.Status = StatusUnhealthy
        result.Message = fmt.Sprintf("database ping failed: %v", err)
        result.Error = err
        return result
    }
    
    // Test query
    var dummy int
    err = c.db.QueryRowContext(ctx, c.query).Scan(&dummy)
    queryDuration := time.Since(start) - pingDuration
    
    if err != nil {
        result.Status = StatusUnhealthy
        result.Message = fmt.Sprintf("database query failed: %v", err)
        result.Error = err
        return result
    }
    
    // Get connection stats
    stats := c.db.Stats()
    
    result.Status = StatusHealthy
    result.Details["ping_duration_ms"] = pingDuration.Milliseconds()
    result.Details["query_duration_ms"] = queryDuration.Milliseconds()
    result.Details["open_connections"] = stats.OpenConnections
    result.Details["in_use"] = stats.InUse
    result.Details["idle"] = stats.Idle
    
    // Check connection pool health
    if float64(stats.InUse)/float64(stats.MaxOpenConnections) > 0.9 {
        result.Status = StatusDegraded
        result.Message = "connection pool near capacity"
    }
    
    return result
}

func (c *DatabaseHealthCheck) Name() string     { return "database" }
func (c *DatabaseHealthCheck) Type() CheckType  { return CheckTypeReadiness }
func (c *DatabaseHealthCheck) Critical() bool   { return true }
```

#### Redis Health Check

```go
// RedisHealthCheck checks Redis connectivity
type RedisHealthCheck struct {
    client *redis.Client
}

func NewRedisHealthCheck(client *redis.Client) *RedisHealthCheck {
    return &RedisHealthCheck{client: client}
}

func (c *RedisHealthCheck) Check(ctx context.Context) *CheckResult {
    result := &CheckResult{
        Timestamp: time.Now(),
        Details:   make(map[string]interface{}),
    }
    
    // Ping Redis
    start := time.Now()
    pong, err := c.client.Ping(ctx).Result()
    duration := time.Since(start)
    
    if err != nil {
        result.Status = StatusUnhealthy
        result.Message = fmt.Sprintf("Redis ping failed: %v", err)
        result.Error = err
        return result
    }
    
    if pong != "PONG" {
        result.Status = StatusUnhealthy
        result.Message = fmt.Sprintf("unexpected ping response: %s", pong)
        return result
    }
    
    // Get Redis info
    info, err := c.client.Info(ctx, "server", "clients", "memory").Result()
    if err == nil {
        // Parse info into details
        result.Details["redis_info"] = parseRedisInfo(info)
    }
    
    result.Status = StatusHealthy
    result.Details["ping_duration_ms"] = duration.Milliseconds()
    
    // Check memory usage
    if memInfo, ok := result.Details["redis_info"].(map[string]interface{}); ok {
        if usedMemory, ok := memInfo["used_memory_rss"].(int64); ok {
            if maxMemory, ok := memInfo["maxmemory"].(int64); ok && maxMemory > 0 {
                usage := float64(usedMemory) / float64(maxMemory)
                if usage > 0.9 {
                    result.Status = StatusDegraded
                    result.Message = fmt.Sprintf("Redis memory usage high: %.1f%%", usage*100)
                }
            }
        }
    }
    
    return result
}

func (c *RedisHealthCheck) Name() string     { return "redis" }
func (c *RedisHealthCheck) Type() CheckType  { return CheckTypeReadiness }
func (c *RedisHealthCheck) Critical() bool   { return false }
```

#### AWS Service Health Checks

```go
// S3HealthCheck verifies S3 access
type S3HealthCheck struct {
    client *s3.Client
    bucket string
}

func NewS3HealthCheck(client *s3.Client, bucket string) *S3HealthCheck {
    return &S3HealthCheck{
        client: client,
        bucket: bucket,
    }
}

func (c *S3HealthCheck) Check(ctx context.Context) *CheckResult {
    result := &CheckResult{
        Timestamp: time.Now(),
        Details:   make(map[string]interface{}),
    }
    
    // List bucket (with limit 1)
    start := time.Now()
    _, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
        Bucket:  aws.String(c.bucket),
        MaxKeys: aws.Int32(1),
    })
    duration := time.Since(start)
    
    if err != nil {
        result.Status = StatusUnhealthy
        result.Message = fmt.Sprintf("S3 access failed: %v", err)
        result.Error = err
        return result
    }
    
    result.Status = StatusHealthy
    result.Details["bucket"] = c.bucket
    result.Details["operation_duration_ms"] = duration.Milliseconds()
    
    return result
}

func (c *S3HealthCheck) Name() string     { return "s3" }
func (c *S3HealthCheck) Type() CheckType  { return CheckTypeReadiness }
func (c *S3HealthCheck) Critical() bool   { return false }

// SQSHealthCheck verifies SQS access
type SQSHealthCheck struct {
    client   *sqs.Client
    queueURL string
}

func (c *SQSHealthCheck) Check(ctx context.Context) *CheckResult {
    result := &CheckResult{
        Timestamp: time.Now(),
        Details:   make(map[string]interface{}),
    }
    
    // Get queue attributes
    start := time.Now()
    attrs, err := c.client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
        QueueUrl: aws.String(c.queueURL),
        AttributeNames: []types.QueueAttributeName{
            types.QueueAttributeNameApproximateNumberOfMessages,
        },
    })
    duration := time.Since(start)
    
    if err != nil {
        result.Status = StatusUnhealthy
        result.Message = fmt.Sprintf("SQS access failed: %v", err)
        result.Error = err
        return result
    }
    
    result.Status = StatusHealthy
    result.Details["queue_url"] = c.queueURL
    result.Details["operation_duration_ms"] = duration.Milliseconds()
    
    if msgCount, ok := attrs.Attributes["ApproximateNumberOfMessages"]; ok {
        count, _ := strconv.Atoi(msgCount)
        result.Details["message_count"] = count
        
        // Alert if queue is backing up
        if count > 1000 {
            result.Status = StatusDegraded
            result.Message = fmt.Sprintf("queue depth high: %d messages", count)
        }
    }
    
    return result
}
```

### 4. Custom Health Checks

```go
// FunctionHealthCheck for custom logic
type FunctionHealthCheck struct {
    name     string
    checkFn  func(ctx context.Context) error
    critical bool
    checkType CheckType
}

func NewFunctionHealthCheck(name string, fn func(context.Context) error) *FunctionHealthCheck {
    return &FunctionHealthCheck{
        name:      name,
        checkFn:   fn,
        critical:  false,
        checkType: CheckTypeReadiness,
    }
}

func (c *FunctionHealthCheck) Check(ctx context.Context) *CheckResult {
    result := &CheckResult{
        Timestamp: time.Now(),
    }
    
    if err := c.checkFn(ctx); err != nil {
        result.Status = StatusUnhealthy
        result.Message = err.Error()
        result.Error = err
    } else {
        result.Status = StatusHealthy
    }
    
    return result
}

// Example custom checks
func RegisterCustomChecks(manager *HealthManager) {
    // Disk space check
    manager.Register(NewFunctionHealthCheck("disk_space", func(ctx context.Context) error {
        var stat syscall.Statfs_t
        if err := syscall.Statfs("/", &stat); err != nil {
            return err
        }
        
        available := stat.Bavail * uint64(stat.Bsize)
        total := stat.Blocks * uint64(stat.Bsize)
        usagePercent := float64(total-available) / float64(total) * 100
        
        if usagePercent > 90 {
            return fmt.Errorf("disk usage critical: %.1f%%", usagePercent)
        }
        if usagePercent > 80 {
            return fmt.Errorf("disk usage high: %.1f%%", usagePercent)
        }
        
        return nil
    }))
    
    // Memory check
    manager.Register(NewFunctionHealthCheck("memory", func(ctx context.Context) error {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        
        usedMB := m.Alloc / 1024 / 1024
        totalMB := m.Sys / 1024 / 1024
        
        if usedMB > 1024 { // 1GB threshold
            return fmt.Errorf("memory usage high: %d MB", usedMB)
        }
        
        return nil
    }))
}
```

## Using Health Checks

### Basic Usage

```go
// Create health checker
healthChecker := health.NewHealthChecker(logger, metrics)

// Register checks
healthChecker.RegisterCheck("database", 
    health.NewDatabaseHealthCheck("postgres", db))
healthChecker.RegisterCheck("redis", 
    health.NewRedisHealthCheck("redis", redisClient))
healthChecker.RegisterCheck("s3", 
    health.NewS3HealthCheck("s3", s3Client, bucketName))

// Run checks manually
results := healthChecker.RunChecks(ctx)

// Or start background checks
go healthChecker.StartBackgroundChecks(ctx)

// Get aggregated health
health := healthChecker.GetAggregatedHealth()
if health.Status == health.StatusUnhealthy {
    // Handle unhealthy state
}
```

### HTTP Endpoint Integration (Manual)

The package doesn't include HTTP handlers. You need to create them:

```go
// Example Gin handler
router.GET("/health", func(c *gin.Context) {
    health := healthChecker.GetAggregatedHealth()
    
    statusCode := http.StatusOK
    if health.Status == health.StatusUnhealthy {
        statusCode = http.StatusServiceUnavailable
    }
    
    c.JSON(statusCode, health)
})
```

## Planned Features (Not Yet Implemented)

The following sections describe planned features that are not yet implemented:

### HTTP Health Endpoints
- Separate liveness/readiness/startup endpoints
- Verbose mode with detailed check results
- Kubernetes-compatible responses

### Advanced Health Checks
- Circuit breaker integration
- Disk space and memory checks
- Custom threshold configuration
- Check result caching

### Monitoring Integration
- Prometheus metrics with detailed labels
- Health check duration histograms
- Error tracking and alerting

## Current Implementation Limitations

1. **No Check Types**: All checks are treated the same (no liveness vs readiness)
2. **No Caching**: Each check runs every time without caching
3. **No HTTP Handlers**: Must implement your own HTTP endpoints
4. **Basic Metrics**: Only records status and duration
5. **No Circuit Breakers**: No integration with circuit breaker patterns
6. **Fixed Timeouts**: 5-second timeout for all checks
7. **No Critical Flag**: All failing checks are treated equally

## Basic Metrics (Actual Implementation)

```go
func (h *HealthChecker) recordMetrics(name string, check *Check) {
    // Record check status (1.0 for healthy, 0.0 for unhealthy)
    statusValue := 0.0
    if check.Status == StatusHealthy {
        statusValue = 1.0
    }

    h.metrics.RecordGauge("health_check_status", statusValue, map[string]string{
        "component": name,
    })

    // Record check duration
    h.metrics.RecordHistogram("health_check_duration_seconds", 
        check.Duration.Seconds(), map[string]string{
        "component": name,
    })
}
```

**Note**: The actual metrics are much simpler than the comprehensive Prometheus metrics shown in the planned features.

## Configuration

### Environment Variables

```bash
# Health Check Configuration
HEALTH_CHECK_TIMEOUT=5s
HEALTH_CHECK_CACHE_DURATION=10s
HEALTH_CHECK_INTERVAL=30s

# Component Timeouts
HEALTH_DB_TIMEOUT=3s
HEALTH_REDIS_TIMEOUT=2s
HEALTH_AWS_TIMEOUT=5s

# Thresholds
HEALTH_DISK_THRESHOLD=90
HEALTH_MEMORY_THRESHOLD=1024  # MB
HEALTH_CONNECTION_POOL_THRESHOLD=0.9
```

### Configuration File

```yaml
health:
  timeout: 5s
  cache_duration: 10s
  interval: 30s
  
  checks:
    database:
      enabled: true
      timeout: 3s
      critical: true
      
    redis:
      enabled: true
      timeout: 2s
      critical: false
      
    s3:
      enabled: true
      timeout: 5s
      critical: false
      bucket: ${S3_BUCKET}
      
    sqs:
      enabled: true
      timeout: 5s
      critical: false
      queue_url: ${SQS_QUEUE_URL}
      
  thresholds:
    disk_usage_percent: 90
    memory_usage_mb: 1024
    connection_pool_usage: 0.9
    queue_depth: 1000
```

## Best Practices (For Current Implementation)

1. **Timeouts**: All checks use a 5-second timeout
2. **Background Checks**: Use `StartBackgroundChecks` for periodic monitoring
3. **Custom Checks**: Use `ServiceHealthCheck` for custom logic
4. **Error Handling**: Health checks should return errors, not panic
5. **Metrics**: Ensure metrics client is provided for monitoring

## Future Development

To implement the full health system described in this README:

1. Add check types (liveness, readiness, startup)
2. Implement result caching
3. Add HTTP handler with multiple endpoints
4. Integrate circuit breakers
5. Add more detailed metrics
6. Support critical vs non-critical checks
7. Add Kubernetes-compatible responses

---

Package Version: 0.1.0 (Basic Implementation)
Last Updated: 2024-01-23
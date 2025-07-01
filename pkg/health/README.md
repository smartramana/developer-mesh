# Health Package

> **Purpose**: Comprehensive health check system for monitoring service readiness and liveness
> **Status**: Production Ready
> **Dependencies**: HTTP/gRPC health endpoints, dependency checks, circuit breakers

## Overview

The health package provides a standardized health check system that monitors service dependencies, tracks component status, and exposes health endpoints for orchestration platforms like Kubernetes and AWS ELB.

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

## Core Components

### 1. Health Check Interface

```go
// HealthChecker defines health check operations
type HealthChecker interface {
    // Check performs a health check
    Check(ctx context.Context) *CheckResult
    
    // Name returns the check name
    Name() string
    
    // Type returns the check type (liveness/readiness)
    Type() CheckType
    
    // Critical indicates if this check failure should mark service unhealthy
    Critical() bool
}

// CheckResult contains health check results
type CheckResult struct {
    Status      Status                 `json:"status"`
    Message     string                 `json:"message,omitempty"`
    Timestamp   time.Time              `json:"timestamp"`
    Duration    time.Duration          `json:"duration"`
    Details     map[string]interface{} `json:"details,omitempty"`
    Error       error                  `json:"-"`
}

// Status represents health status
type Status string

const (
    StatusHealthy   Status = "healthy"
    StatusDegraded  Status = "degraded"
    StatusUnhealthy Status = "unhealthy"
)

// CheckType defines health check categories
type CheckType string

const (
    CheckTypeLiveness  CheckType = "liveness"
    CheckTypeReadiness CheckType = "readiness"
    CheckTypeStartup   CheckType = "startup"
)
```

### 2. Health Manager

```go
// HealthManager coordinates health checks
type HealthManager struct {
    checks      map[string]HealthChecker
    mu          sync.RWMutex
    cache       *CheckCache
    config      *HealthConfig
    metrics     *Metrics
    circuitBreaker *CircuitBreaker
}

// NewHealthManager creates a health manager
func NewHealthManager(config *HealthConfig) *HealthManager {
    return &HealthManager{
        checks:  make(map[string]HealthChecker),
        cache:   NewCheckCache(config.CacheDuration),
        config:  config,
        metrics: NewMetrics(),
    }
}

// Register adds a health check
func (m *HealthManager) Register(check HealthChecker) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.checks[check.Name()] = check
    logger.Info("registered health check", 
        "name", check.Name(),
        "type", check.Type(),
        "critical", check.Critical(),
    )
}

// GetHealth returns aggregated health status
func (m *HealthManager) GetHealth(ctx context.Context, checkType CheckType) *HealthStatus {
    m.mu.RLock()
    checks := m.filterChecks(checkType)
    m.mu.RUnlock()
    
    results := make(map[string]*CheckResult)
    status := StatusHealthy
    
    // Run checks in parallel
    var wg sync.WaitGroup
    resultsChan := make(chan struct {
        name   string
        result *CheckResult
    }, len(checks))
    
    for name, check := range checks {
        wg.Add(1)
        go func(n string, c HealthChecker) {
            defer wg.Done()
            
            // Check cache first
            if cached := m.cache.Get(n); cached != nil {
                resultsChan <- struct {
                    name   string
                    result *CheckResult
                }{n, cached}
                return
            }
            
            // Run check with timeout
            checkCtx, cancel := context.WithTimeout(ctx, m.config.CheckTimeout)
            defer cancel()
            
            start := time.Now()
            result := c.Check(checkCtx)
            result.Duration = time.Since(start)
            
            // Cache result
            m.cache.Set(n, result)
            
            // Update metrics
            m.metrics.RecordCheck(n, result)
            
            resultsChan <- struct {
                name   string
                result *CheckResult
            }{n, result}
        }(name, check)
    }
    
    // Collect results
    go func() {
        wg.Wait()
        close(resultsChan)
    }()
    
    for r := range resultsChan {
        results[r.name] = r.result
        
        // Update overall status
        if r.result.Status == StatusUnhealthy && checks[r.name].Critical() {
            status = StatusUnhealthy
        } else if r.result.Status == StatusDegraded && status == StatusHealthy {
            status = StatusDegraded
        }
    }
    
    return &HealthStatus{
        Status:    status,
        Checks:    results,
        Timestamp: time.Now(),
    }
}
```

### 3. Built-in Health Checks

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

## HTTP Health Endpoints

### 1. HTTP Handler

```go
// HTTPHandler provides health endpoints
type HTTPHandler struct {
    manager *HealthManager
}

func NewHTTPHandler(manager *HealthManager) *HTTPHandler {
    return &HTTPHandler{manager: manager}
}

// RegisterRoutes adds health endpoints
func (h *HTTPHandler) RegisterRoutes(router *gin.Engine) {
    router.GET("/health", h.handleHealth)
    router.GET("/health/live", h.handleLiveness)
    router.GET("/health/ready", h.handleReadiness)
    router.GET("/health/startup", h.handleStartup)
}

func (h *HTTPHandler) handleHealth(c *gin.Context) {
    verbose := c.Query("verbose") == "true"
    
    health := h.manager.GetHealth(c.Request.Context(), CheckTypeReadiness)
    
    response := gin.H{
        "status":    health.Status,
        "timestamp": health.Timestamp,
    }
    
    if verbose {
        response["checks"] = health.Checks
    }
    
    statusCode := http.StatusOK
    if health.Status == StatusUnhealthy {
        statusCode = http.StatusServiceUnavailable
    }
    
    c.JSON(statusCode, response)
}

func (h *HTTPHandler) handleLiveness(c *gin.Context) {
    health := h.manager.GetHealth(c.Request.Context(), CheckTypeLiveness)
    
    if health.Status == StatusHealthy {
        c.JSON(http.StatusOK, gin.H{"status": "ok"})
    } else {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "status": "unhealthy",
            "checks": health.Checks,
        })
    }
}

func (h *HTTPHandler) handleReadiness(c *gin.Context) {
    health := h.manager.GetHealth(c.Request.Context(), CheckTypeReadiness)
    
    response := gin.H{
        "status": health.Status,
        "checks": make(map[string]string),
    }
    
    // Simplified check results
    for name, result := range health.Checks {
        response["checks"].(map[string]string)[name] = string(result.Status)
    }
    
    statusCode := http.StatusOK
    if health.Status != StatusHealthy {
        statusCode = http.StatusServiceUnavailable
    }
    
    c.JSON(statusCode, response)
}
```

### 2. Kubernetes Integration

```yaml
# Kubernetes deployment with health checks
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-server
spec:
  template:
    spec:
      containers:
      - name: mcp-server
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
          
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 2
          
        startupProbe:
          httpGet:
            path: /health/startup
            port: 8080
          initialDelaySeconds: 0
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 30
```

## Circuit Breaker Integration

```go
// CircuitBreakerHealthCheck monitors circuit breaker status
type CircuitBreakerHealthCheck struct {
    breakers map[string]*gobreaker.CircuitBreaker
}

func (c *CircuitBreakerHealthCheck) Check(ctx context.Context) *CheckResult {
    result := &CheckResult{
        Timestamp: time.Now(),
        Status:    StatusHealthy,
        Details:   make(map[string]interface{}),
    }
    
    unhealthyCount := 0
    breakerStatus := make(map[string]string)
    
    for name, breaker := range c.breakers {
        state := breaker.State()
        breakerStatus[name] = state.String()
        
        if state == gobreaker.StateOpen {
            unhealthyCount++
        }
    }
    
    result.Details["circuit_breakers"] = breakerStatus
    result.Details["open_breakers"] = unhealthyCount
    
    if unhealthyCount > 0 {
        result.Status = StatusDegraded
        result.Message = fmt.Sprintf("%d circuit breakers open", unhealthyCount)
    }
    
    return result
}
```

## Monitoring & Metrics

```go
var (
    healthCheckDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_health_check_duration_seconds",
            Help:    "Health check duration",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
        },
        []string{"check_name", "check_type"},
    )
    
    healthCheckStatus = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_health_check_status",
            Help: "Health check status (0=healthy, 1=degraded, 2=unhealthy)",
        },
        []string{"check_name", "check_type"},
    )
    
    healthCheckErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_health_check_errors_total",
            Help: "Total number of health check errors",
        },
        []string{"check_name", "check_type"},
    )
)

// Metrics records health check metrics
type Metrics struct{}

func (m *Metrics) RecordCheck(name string, result *CheckResult) {
    status := 0.0
    switch result.Status {
    case StatusDegraded:
        status = 1.0
    case StatusUnhealthy:
        status = 2.0
    }
    
    healthCheckStatus.WithLabelValues(name, "readiness").Set(status)
    healthCheckDuration.WithLabelValues(name, "readiness").Observe(result.Duration.Seconds())
    
    if result.Error != nil {
        healthCheckErrors.WithLabelValues(name, "readiness").Inc()
    }
}
```

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

## Best Practices

1. **Granular Checks**: Separate liveness and readiness checks
2. **Timeouts**: Set appropriate timeouts for each check
3. **Caching**: Cache results to avoid overloading dependencies
4. **Critical vs Non-Critical**: Mark only essential dependencies as critical
5. **Graceful Degradation**: Continue operating with degraded status when possible
6. **Detailed Responses**: Include helpful details for debugging
7. **Circuit Breaking**: Integrate with circuit breakers for failing dependencies
8. **Monitoring**: Export metrics for all health checks

---

Package Version: 1.0.0
Last Updated: 2024-01-10
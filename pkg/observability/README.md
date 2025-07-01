# Observability Package

The `observability` package provides comprehensive instrumentation for metrics, logging, and distributed tracing across the DevOps MCP platform.

## Overview

This package implements the three pillars of observability:
- **Metrics**: Prometheus-based metrics collection
- **Logging**: Structured JSON logging with trace correlation
- **Tracing**: OpenTelemetry-based distributed tracing

## Features

- Unified observability initialization
- Automatic trace context propagation
- Structured logging with trace correlation
- Custom metrics for business KPIs
- WebSocket-specific instrumentation
- Cost tracking and attribution
- Performance monitoring
- Error tracking and alerting

## Installation

```bash
go get github.com/your-org/devops-mcp/pkg/observability
```

## Quick Start

```go
package main

import (
    "context"
    "github.com/your-org/devops-mcp/pkg/observability"
)

func main() {
    // Initialize observability
    obs, err := observability.New(observability.Config{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        
        // Tracing
        TracingEnabled:  true,
        JaegerEndpoint:  "http://jaeger:14268/api/traces",
        SamplingRate:    0.01, // 1%
        
        // Metrics
        MetricsEnabled:  true,
        MetricsPort:     9090,
        
        // Logging
        LogLevel:        "info",
        LogFormat:       "json",
    })
    if err != nil {
        panic(err)
    }
    defer obs.Shutdown(context.Background())
    
    // Use in your application
    ctx := context.Background()
    ctx, span := obs.Tracer.Start(ctx, "main.operation")
    defer span.End()
    
    obs.Logger.Info(ctx, "Starting operation", 
        observability.Field("user_id", "123"))
}
```

## Core Components

### 1. Tracer

OpenTelemetry-based distributed tracing:

```go
// Start a span
ctx, span := obs.Tracer.Start(ctx, "operation.name",
    trace.WithSpanKind(trace.SpanKindServer),
    trace.WithAttributes(
        attribute.String("user.id", userID),
        attribute.Float64("cost.estimated", 0.05),
    ),
)
defer span.End()

// Add events
span.AddEvent("Processing started", trace.WithAttributes(
    attribute.Int("item.count", len(items)),
))

// Record errors
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

### 2. Logger

Structured logging with automatic trace correlation:

```go
// Log with context (includes trace ID automatically)
obs.Logger.Info(ctx, "Processing request",
    observability.Field("request_id", requestID),
    observability.Field("method", "POST"),
)

// Log errors with stack trace
obs.Logger.Error(ctx, "Operation failed",
    observability.ErrorField(err),
    observability.Field("retry_count", 3),
)

// Create child logger with fields
logger := obs.Logger.With(
    observability.Field("component", "worker"),
    observability.Field("tenant_id", tenantID),
)
```

### 3. Metrics

Prometheus metrics with custom business KPIs:

```go
// Counter
obs.Metrics.IncrementCounter(ctx, "requests_total", 
    observability.Label("method", "POST"),
    observability.Label("status", "200"),
)

// Gauge
obs.Metrics.SetGauge(ctx, "active_connections", 42,
    observability.Label("protocol", "websocket"),
)

// Histogram
obs.Metrics.RecordHistogram(ctx, "request_duration_seconds", 0.125,
    observability.Label("endpoint", "/api/v1/tasks"),
)

// Summary
obs.Metrics.RecordSummary(ctx, "task_processing_time", 5.2,
    observability.Label("task_type", "embedding"),
)
```

## Advanced Features

### WebSocket Instrumentation

Specialized instrumentation for WebSocket connections:

```go
// WebSocket connection tracking
conn := obs.WebSocket.TrackConnection(ctx, wsConn, observability.WSOptions{
    AgentID:         agentID,
    ProtocolVersion: "2.0",
    Capabilities:    []string{"embeddings", "inference"},
})

// Message tracking
msg := &WSMessage{Type: "task_assignment", Payload: data}
err := conn.SendMessage(ctx, msg)
// Automatically tracks: message size, compression, latency

// Ping/pong monitoring
conn.StartPingMonitoring(30 * time.Second)
```

### Cost Tracking

Track AI model costs and resource usage:

```go
// Start cost tracking span
ctx, costSpan := obs.Cost.StartOperation(ctx, "bedrock.inference",
    observability.CostEstimate(0.05),
    observability.Model("claude-3-sonnet"),
)
defer costSpan.End()

// Record actual cost
costSpan.RecordCost(0.048)

// Check budget
if exceeded, remaining := obs.Cost.CheckBudget(ctx, tenantID); exceeded {
    return ErrBudgetExceeded
}
```

### Multi-Service Correlation

Automatic context propagation across services:

```go
// HTTP client with tracing
client := obs.InstrumentHTTPClient(http.DefaultClient)
resp, err := client.Get(ctx, "http://other-service/api/data")

// gRPC with tracing
conn, err := grpc.Dial(target,
    grpc.WithUnaryInterceptor(obs.UnaryClientInterceptor()),
    grpc.WithStreamInterceptor(obs.StreamClientInterceptor()),
)

// SQS with trace propagation
producer := obs.InstrumentSQSProducer(sqsClient)
err = producer.SendMessage(ctx, &sqs.SendMessageInput{
    QueueUrl:    aws.String(queueURL),
    MessageBody: aws.String(body),
})
```

## Configuration

### Full Configuration Options

```go
type Config struct {
    // Service identification
    ServiceName    string
    ServiceVersion string
    Environment    string
    
    // Tracing configuration
    TracingEnabled     bool
    JaegerEndpoint     string
    SamplingRate       float64
    SamplingRules      []SamplingRule
    TraceIDHeader      string // Default: "X-Trace-ID"
    BaggageEnabled     bool
    
    // Metrics configuration
    MetricsEnabled     bool
    MetricsPort        int
    MetricsPath        string // Default: "/metrics"
    CustomMetrics      []MetricDefinition
    HistogramBuckets   []float64
    
    // Logging configuration
    LogLevel           string // debug, info, warn, error
    LogFormat          string // json, text
    LogOutput          string // stdout, file
    LogFile            string
    LogSampling        bool
    LogSamplingInitial int
    LogSamplingAfter   int
    
    // Performance tuning
    BatchInterval      time.Duration
    MaxBatchSize       int
    QueueSize          int
    WorkerCount        int
    
    // Resource limits
    MaxActiveSpans     int
    MaxAttributes      int
    MaxEvents          int
    MaxLinks           int
    
    // Feature flags
    EnableProfiling    bool
    EnableDebugEndpoints bool
}
```

### Environment Variables

All configuration can be overridden via environment variables:

```bash
# Service identification
export OTEL_SERVICE_NAME=my-service
export OTEL_SERVICE_VERSION=1.0.0
export ENVIRONMENT=production

# Tracing
export OTEL_EXPORTER_JAEGER_ENDPOINT=http://jaeger:14268/api/traces
export TRACE_SAMPLING_RATE=0.01
export TRACE_ENABLED=true

# Metrics
export METRICS_ENABLED=true
export METRICS_PORT=9090

# Logging
export LOG_LEVEL=info
export LOG_FORMAT=json
```

## API Reference

### Tracer API

```go
// Tracer provides distributed tracing capabilities
type Tracer interface {
    // Start creates a new span
    Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
    
    // Extract extracts trace context from carrier
    Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context
    
    // Inject injects trace context into carrier
    Inject(ctx context.Context, carrier propagation.TextMapCarrier)
}
```

### Logger API

```go
// Logger provides structured logging
type Logger interface {
    // Log at different levels
    Debug(ctx context.Context, msg string, fields ...Field)
    Info(ctx context.Context, msg string, fields ...Field)
    Warn(ctx context.Context, msg string, fields ...Field)
    Error(ctx context.Context, msg string, fields ...Field)
    
    // Create child logger with fields
    With(fields ...Field) Logger
    
    // Check if level is enabled
    IsDebugEnabled() bool
}

// Field represents a log field
type Field struct {
    Key   string
    Value interface{}
}
```

### Metrics API

```go
// Metrics provides metric collection
type Metrics interface {
    // Counter operations
    IncrementCounter(ctx context.Context, name string, labels ...Label)
    AddCounter(ctx context.Context, name string, value float64, labels ...Label)
    
    // Gauge operations
    SetGauge(ctx context.Context, name string, value float64, labels ...Label)
    IncrementGauge(ctx context.Context, name string, labels ...Label)
    DecrementGauge(ctx context.Context, name string, labels ...Label)
    
    // Histogram operations
    RecordHistogram(ctx context.Context, name string, value float64, labels ...Label)
    
    // Summary operations
    RecordSummary(ctx context.Context, name string, value float64, labels ...Label)
}

// Label represents a metric label
type Label struct {
    Name  string
    Value string
}
```

## Middleware

### HTTP Middleware

```go
// Gin middleware
router := gin.New()
router.Use(obs.GinMiddleware())

// Standard HTTP middleware
handler := obs.HTTPMiddleware(http.HandlerFunc(myHandler))

// Custom middleware options
handler := obs.HTTPMiddleware(myHandler, 
    observability.WithOperationName("custom.operation"),
    observability.WithFilter(func(r *http.Request) bool {
        return r.URL.Path != "/health"
    }),
)
```

### gRPC Interceptors

```go
// Server interceptors
server := grpc.NewServer(
    grpc.UnaryInterceptor(obs.UnaryServerInterceptor()),
    grpc.StreamInterceptor(obs.StreamServerInterceptor()),
)

// Client interceptors
conn, err := grpc.Dial(address,
    grpc.WithUnaryInterceptor(obs.UnaryClientInterceptor()),
    grpc.WithStreamInterceptor(obs.StreamClientInterceptor()),
)
```

## Custom Instrumentation

### Creating Custom Metrics

```go
// Define custom metric
costPerRequest := obs.Metrics.NewHistogram(
    "mcp_cost_per_request_usd",
    "Cost per request in USD",
    []float64{0.001, 0.01, 0.1, 1.0, 10.0}, // Buckets
)

// Use in code
costPerRequest.Record(ctx, 0.05,
    observability.Label("model", "claude-3"),
    observability.Label("operation", "embedding"),
)
```

### Custom Span Attributes

```go
// Define reusable attributes
var (
    AttrUserID     = attribute.Key("mcp.user.id")
    AttrTenantID   = attribute.Key("mcp.tenant.id")
    AttrCostUSD    = attribute.Key("mcp.cost.usd")
    AttrModelName  = attribute.Key("mcp.model.name")
)

// Use in spans
span.SetAttributes(
    AttrUserID.String(userID),
    AttrTenantID.String(tenantID),
    AttrCostUSD.Float64(0.05),
    AttrModelName.String("claude-3-sonnet"),
)
```

### Custom Propagators

```go
// Implement custom propagator for MCP-specific context
type MCPPropagator struct{}

func (p *MCPPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
    if mcpCtx := GetMCPContext(ctx); mcpCtx != nil {
        carrier.Set("X-MCP-Tenant-ID", mcpCtx.TenantID)
        carrier.Set("X-MCP-Cost-Limit", fmt.Sprintf("%.2f", mcpCtx.CostLimit))
    }
}

func (p *MCPPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
    tenantID := carrier.Get("X-MCP-Tenant-ID")
    costLimit, _ := strconv.ParseFloat(carrier.Get("X-MCP-Cost-Limit"), 64)
    
    return WithMCPContext(ctx, &MCPContext{
        TenantID:  tenantID,
        CostLimit: costLimit,
    })
}

// Register propagator
obs.RegisterPropagator(&MCPPropagator{})
```

## Best Practices

### 1. Context Usage

Always pass context for proper correlation:

```go
// Good
obs.Logger.Info(ctx, "Processing started")

// Bad - loses trace correlation
obs.Logger.Info(context.Background(), "Processing started")
```

### 2. Span Naming

Use hierarchical names for operations:

```go
// Good
span := obs.Tracer.Start(ctx, "repository.user.FindByEmail")
span := obs.Tracer.Start(ctx, "http.handler.CreateTask")
span := obs.Tracer.Start(ctx, "cache.redis.Get")

// Bad
span := obs.Tracer.Start(ctx, "find")
span := obs.Tracer.Start(ctx, "handler")
```

### 3. Error Handling

Always record errors in spans:

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
    obs.Logger.Error(ctx, "Operation failed", 
        observability.ErrorField(err))
    return err
}
```

### 4. Resource Management

Properly close spans and flush data:

```go
// Always end spans
ctx, span := obs.Tracer.Start(ctx, "operation")
defer span.End()

// Shutdown gracefully
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := obs.Shutdown(ctx); err != nil {
    log.Printf("Failed to shutdown observability: %v", err)
}
```

### 5. Sampling Configuration

Configure appropriate sampling for production:

```go
// Production configuration
obs, err := observability.New(observability.Config{
    SamplingRate: 0.01, // 1% base rate
    SamplingRules: []SamplingRule{
        {
            Name:     "errors",
            Condition: func(attrs []attribute.KeyValue) bool {
                return hasError(attrs)
            },
            Rate: 1.0, // 100% for errors
        },
        {
            Name:     "slow_requests",
            Condition: func(attrs []attribute.KeyValue) bool {
                return getDuration(attrs) > 1000
            },
            Rate: 0.5, // 50% for slow requests
        },
    },
})
```

## Performance Considerations

### Overhead Measurements

Typical overhead with 1% sampling:
- CPU: < 0.5%
- Memory: ~10MB per service
- Latency: < 0.1ms per request

### Optimization Tips

1. **Use sampling**: Don't trace every request in production
2. **Limit attributes**: Keep span attributes minimal
3. **Batch exports**: Configure appropriate batch intervals
4. **Async logging**: Use buffered logging for high throughput
5. **Selective instrumentation**: Only instrument critical paths

## Troubleshooting

### Common Issues

1. **Missing Traces**
   - Check sampling configuration
   - Verify Jaeger endpoint is accessible
   - Ensure context is propagated correctly

2. **High Memory Usage**
   - Reduce max active spans
   - Lower sampling rate
   - Check for span leaks (missing End() calls)

3. **Missing Trace Correlation**
   - Ensure context is passed to all operations
   - Check propagator configuration
   - Verify middleware order

### Debug Mode

Enable debug mode for troubleshooting:

```go
obs, err := observability.New(observability.Config{
    EnableDebugEndpoints: true,
    LogLevel: "debug",
})

// Access debug endpoints
// GET /debug/traces - Recent traces
// GET /debug/metrics - Internal metrics
// GET /debug/config - Current configuration
```

## Migration Guide

### From OpenTracing

```go
// Old (OpenTracing)
span := opentracing.StartSpan("operation")
defer span.Finish()

// New (OpenTelemetry)
ctx, span := obs.Tracer.Start(ctx, "operation")
defer span.End()
```

### From Prometheus Client

```go
// Old (Direct Prometheus)
counter := prometheus.NewCounter(prometheus.CounterOpts{
    Name: "requests_total",
})
counter.Inc()

// New (Observability package)
obs.Metrics.IncrementCounter(ctx, "requests_total")
```

## Examples

See the [examples](./examples) directory for complete examples:
- [Basic Usage](./examples/basic/main.go)
- [HTTP Service](./examples/http/main.go)
- [gRPC Service](./examples/grpc/main.go)
- [WebSocket Server](./examples/websocket/main.go)
- [Worker with SQS](./examples/worker/main.go)
- [Custom Metrics](./examples/metrics/main.go)

## Contributing

1. Follow the existing code style
2. Add tests for new features
3. Update documentation
4. Run benchmarks for performance-critical changes

## License

This package is part of the DevOps MCP platform and follows the same license terms.
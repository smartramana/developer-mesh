# Observability Package

The `observability` package provides unified logging, metrics, and tracing capabilities for the DevOps MCP platform.

## Overview

This package implements observability features:
- **Logging**: Standard logging with different log levels
- **Metrics**: Metrics collection interface with various recording methods
- **Tracing**: Basic OpenTelemetry integration (when enabled)

## Features

- Unified observability initialization
- Standard and no-op logger implementations
- Metrics client interface with operation-specific methods
- Optional tracing support
- Context-based component management

## Installation

```bash
go get github.com/S-Corkum/devops-mcp/pkg/observability
```

## Quick Start

```go
package main

import (
    "context"
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

func main() {
    // Initialize observability
    err := observability.Initialize(observability.Config{
        Tracing: observability.TracingConfig{
            Enabled:     true,
            ServiceName: "my-service",
            Environment: "production",
            Endpoint:    "http://jaeger:14268/api/traces",
        },
    })
    if err != nil {
        panic(err)
    }
    defer observability.ObservabilityShutdown()
    
    // Use default logger
    observability.DefaultLogger.Info("Starting operation", map[string]interface{}{
        "user_id": "123",
    })
    
    // Use default metrics
    observability.DefaultMetricsClient.IncrementCounter("requests", 1)
}
```

## Core Components

### 1. Logger

The package provides a standard logger implementation with log levels:

```go
// Using the default logger
observability.DefaultLogger.Info("Processing request", map[string]interface{}{
    "request_id": requestID,
    "method":     "POST",
})

// Log at different levels
observability.DefaultLogger.Debug("Debug message", nil)
observability.DefaultLogger.Warn("Warning message", map[string]interface{}{"count": 10})
observability.DefaultLogger.Error("Error occurred", map[string]interface{}{"error": err.Error()})

// Formatted logging
observability.DefaultLogger.Infof("Processing %d items", itemCount)
observability.DefaultLogger.Errorf("Failed to process: %v", err)

// Create logger with prefix
logger := observability.NewLogger("my-component")
logger.Info("Component initialized", nil)
```

### 2. Metrics

The MetricsClient interface provides various metric recording methods:

```go
// Basic counter increment
observability.DefaultMetricsClient.IncrementCounter("requests", 1)
observability.DefaultMetricsClient.IncrementCounterWithLabels("requests", 1, map[string]string{
    "method": "POST",
    "status": "200",
})

// Record gauges
observability.DefaultMetricsClient.RecordGauge("active_connections", 42, map[string]string{
    "protocol": "websocket",
})

// Record histograms
observability.DefaultMetricsClient.RecordHistogram("request_duration_seconds", 0.125, map[string]string{
    "endpoint": "/api/v1/tasks",
})

// Operation-specific metrics
observability.DefaultMetricsClient.RecordOperation("api", "create_task", true, 0.05, map[string]string{
    "tenant_id": "123",
})

// Timer convenience method
stop := observability.DefaultMetricsClient.StartTimer("operation_duration", map[string]string{
    "operation": "embedding",
})
// ... do work ...
stop() // Records the duration
```

### 3. Tracing

Basic OpenTelemetry integration when enabled:

```go
// Tracing is initialized if enabled in config
// Uses DefaultStartSpan function
ctx, span := observability.DefaultStartSpan(ctx, "operation.name",
    attribute.String("user.id", userID),
)
defer span.End()

// Add events to span
span.AddEvent("Processing started", map[string]interface{}{
    "item_count": len(items),
})

// Record errors
if err != nil {
    span.RecordError(err)
    span.SetStatus(1, err.Error()) // 1 = Error status
}
```

## Context Management

The package supports storing observability components in context:

```go
// Store components in context
ctx := observability.With(ctx, logger, metricsClient, startSpanFunc)

// Extract components from context
logger, metrics, startSpan := observability.FromContext(ctx)

// Components fall back to defaults if not in context
logger.Info("Using logger from context or default", nil)
```

## Configuration

### Configuration Structure

```go
type Config struct {
    Tracing TracingConfig `json:"tracing,omitempty"`
    Metrics MetricsConfig `json:"metrics,omitempty"`
    Logging LoggingConfig `json:"logging,omitempty"`
}

type TracingConfig struct {
    Enabled     bool   `json:"enabled"`
    ServiceName string `json:"service_name,omitempty"`
    Environment string `json:"environment,omitempty"`
    Endpoint    string `json:"endpoint,omitempty"`
}

type MetricsConfig struct {
    Enabled      bool          `json:"enabled"`
    Type         string        `json:"type,omitempty"`
    Endpoint     string        `json:"endpoint,omitempty"`
    Namespace    string        `json:"namespace,omitempty"`
    PushGateway  string        `json:"push_gateway,omitempty"`
    PushInterval time.Duration `json:"push_interval,omitempty"`
}

type LoggingConfig struct {
    Level    string `json:"level,omitempty"`    // debug, info, warn, error
    Format   string `json:"format,omitempty"`   // json, text
    Output   string `json:"output,omitempty"`   // stdout, file
    FilePath string `json:"file_path,omitempty"`
}
```

## API Reference

### Logger Interface

```go
type Logger interface {
    // Core logging methods with fields
    Debug(msg string, fields map[string]interface{})
    Info(msg string, fields map[string]interface{})
    Warn(msg string, fields map[string]interface{})
    Error(msg string, fields map[string]interface{})
    Fatal(msg string, fields map[string]interface{})

    // Formatted logging methods
    Debugf(format string, args ...interface{})
    Infof(format string, args ...interface{})
    Warnf(format string, args ...interface{})
    Errorf(format string, args ...interface{})
    Fatalf(format string, args ...interface{})

    // Context methods
    WithPrefix(prefix string) Logger
    With(fields map[string]interface{}) Logger
}
```

### MetricsClient Interface

```go
type MetricsClient interface {
    // Core metrics recording methods
    RecordEvent(source, eventType string)
    RecordLatency(operation string, duration time.Duration)
    RecordCounter(name string, value float64, labels map[string]string)
    RecordGauge(name string, value float64, labels map[string]string)
    RecordHistogram(name string, value float64, labels map[string]string)
    RecordTimer(name string, duration time.Duration, labels map[string]string)

    // Operation-specific metrics
    RecordCacheOperation(operation string, success bool, durationSeconds float64)
    RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string)
    RecordAPIOperation(api string, operation string, success bool, durationSeconds float64)
    RecordDatabaseOperation(operation string, success bool, durationSeconds float64)

    // Convenience methods
    StartTimer(name string, labels map[string]string) func()
    IncrementCounter(name string, value float64)
    IncrementCounterWithLabels(name string, value float64, labels map[string]string)
    RecordDuration(name string, duration time.Duration)

    // Lifecycle management
    Close() error
}
```

### Span Interface

```go
type Span interface {
    End()
    SetAttribute(key string, value interface{})
    AddEvent(name string, attributes map[string]interface{})
    RecordError(err error)
    SetStatus(code int, description string)
    SpanContext() trace.SpanContext
    TracerProvider() trace.TracerProvider
}

// StartSpanFunc is a function that creates and starts a new span
type StartSpanFunc func(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, Span)
```

## Best Practices

### 1. Logger Usage

Create loggers with meaningful prefixes:

```go
// Good - identifies component
logger := observability.NewLogger("task-service")
logger := observability.NewLogger("websocket-handler")

// Use fields for structured data
logger.Info("Task processed", map[string]interface{}{
    "task_id": taskID,
    "duration_ms": duration.Milliseconds(),
    "status": "success",
})
```

### 2. Metrics Recording

Use appropriate metric types:

```go
// Counters for counts that only increase
metrics.IncrementCounter("requests_total", 1)

// Gauges for values that can go up or down
metrics.RecordGauge("queue_depth", 42, nil)

// Histograms for distributions (latency, sizes)
metrics.RecordHistogram("response_time_seconds", 0.125, map[string]string{
    "endpoint": "/api/tasks",
})

// Use operation-specific methods when available
metrics.RecordAPIOperation("tasks", "create", true, 0.05)
```

### 3. Error Handling

Log errors with context:

```go
if err != nil {
    logger.Error("Operation failed", map[string]interface{}{
        "error": err.Error(),
        "operation": "create_task",
        "user_id": userID,
    })
    
    // Record error in span if tracing enabled
    if span != nil {
        span.RecordError(err)
        span.SetStatus(1, err.Error())
    }
    
    return err
}
```

### 4. Resource Management

Always close resources properly:

```go
// End spans
ctx, span := observability.DefaultStartSpan(ctx, "operation")
defer span.End()

// Shutdown observability on app exit
defer observability.ObservabilityShutdown()

// Close metrics client
defer observability.DefaultMetricsClient.Close()
```

## Implementation Notes

### Current Implementation

**Implemented:**
- Basic logger with standard log package
- No-op logger for testing
- Metrics client interface
- Optional OpenTelemetry tracing
- Context-based component storage
- Global default instances

**Not Implemented:**
- Structured JSON logging
- Log sampling
- Advanced tracing features (baggage, custom propagators)
- HTTP/gRPC middleware
- WebSocket instrumentation
- Cost tracking
- Debug endpoints
- Prometheus metrics implementation (interface only)

### Logger Implementation

The StandardLogger uses Go's standard log package with:
- Timestamp formatting
- Log level filtering
- Field formatting as key=value pairs
- Fatal exits the program

### Metrics Implementation

The package defines the MetricsClient interface but doesn't provide a concrete implementation. Users need to provide their own implementation or use the metrics package from pkg/common.

### Tracing Implementation

Basic OpenTelemetry support when enabled:
- Initializes tracer with Jaeger exporter
- Provides StartSpan function
- No advanced features like sampling rules or custom attributes

## Examples

### Basic Usage

```go
package main

import (
    "github.com/S-Corkum/devops-mcp/pkg/observability"
)

func main() {
    // Initialize with defaults
    observability.Initialize(observability.Config{})
    defer observability.ObservabilityShutdown()
    
    // Use logger
    logger := observability.DefaultLogger
    logger.Info("Application started", nil)
    
    // Record metrics
    metrics := observability.DefaultMetricsClient
    metrics.IncrementCounter("app.started", 1)
    
    // Process something
    timer := metrics.StartTimer("process.duration", nil)
    processData()
    timer()
    
    logger.Info("Processing complete", map[string]interface{}{
        "items_processed": 100,
    })
}
```

## License

This package is part of the DevOps MCP platform.
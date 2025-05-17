// Package observability provides observability functionality for the MCP system.
package observability

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Config is the configuration for observability
type Config struct {
	Tracing TracingConfig
	Metrics MetricsConfig
	Logging LoggingConfig
}

// TracingConfig is the configuration for tracing
type TracingConfig struct {
	Enabled bool
	Endpoint string
	ServiceName string
}

// MetricsConfig is the configuration for metrics
type MetricsConfig struct {
	Enabled bool
	Endpoint string
	Namespace string
}

// LoggingConfig is the configuration for logging
type LoggingConfig struct {
	Level string
	Format string
}

// Logger is the logger interface
type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
	Fatal(msg string, fields map[string]interface{})
	WithPrefix(prefix string) Logger
}

// Span represents a trace span
type Span interface {
	End()
	SetAttributes(attributes ...attribute.KeyValue)
	RecordError(err error, options ...trace.EventOption)
	SetStatus(code int, description string)
}

// StartSpan starts a new span and returns the updated context and span
func StartSpan(ctx context.Context, name string) (context.Context, Span) {
	// For now, return a no-op span since we're just making the code compile
	return ctx, &noopSpan{}
}

// noopSpan is a no-op implementation of the Span interface
type noopSpan struct{}

// End ends the span
func (s *noopSpan) End() {}

// SetAttributes sets attributes on the span
func (s *noopSpan) SetAttributes(attributes ...attribute.KeyValue) {}

// RecordError records an error in the span
func (s *noopSpan) RecordError(err error, options ...trace.EventOption) {}

// SetStatus sets the status of the span
func (s *noopSpan) SetStatus(code int, description string) {}

// simpleLogger is a basic implementation of the Logger interface
type simpleLogger struct {
	name string
}

// DefaultLogger is the default logger for the application
var DefaultLogger Logger = NewLogger("default")

// NewLogger creates a new logger with the given name
func NewLogger(name string) Logger {
	return &simpleLogger{name: name}
}

// Debug logs a debug message
func (l *simpleLogger) Debug(msg string, fields map[string]interface{}) {
	log.Printf("[DEBUG] %s: %s %v", l.name, msg, fields)
}

// Info logs an info message
func (l *simpleLogger) Info(msg string, fields map[string]interface{}) {
	log.Printf("[INFO] %s: %s %v", l.name, msg, fields)
}

// Warn logs a warning message
func (l *simpleLogger) Warn(msg string, fields map[string]interface{}) {
	log.Printf("[WARN] %s: %s %v", l.name, msg, fields)
}

// Error logs an error message
func (l *simpleLogger) Error(msg string, fields map[string]interface{}) {
	log.Printf("[ERROR] %s: %s %v", l.name, msg, fields)
}

// Fatal logs a fatal message and exits
func (l *simpleLogger) Fatal(msg string, fields map[string]interface{}) {
	log.Fatalf("[FATAL] %s: %s %v", l.name, msg, fields)
}

// WithPrefix creates a new logger with the combined name
func (l *simpleLogger) WithPrefix(prefix string) Logger {
	return NewLogger(l.name + "." + prefix)
}

// MetricsClient is the metrics client interface
type MetricsClient interface {
	IncrementCounter(name string, value float64, tags map[string]string)
	RecordHistogram(name string, value float64, tags map[string]string)
	RecordGauge(name string, value float64, tags map[string]string)
	// RecordCounter is an alias for IncrementCounter to maintain compatibility with common/metrics.Client
	RecordCounter(name string, value float64, tags map[string]string)
	// RecordEvent records an event metric for compatibility with common/metrics.Client
	RecordEvent(source, eventType string)
	// RecordLatency records a latency metric for compatibility with common/metrics.Client
	RecordLatency(operation string, duration time.Duration)
	// RecordDuration records a duration metric (alias for RecordLatency) for compatibility
	RecordDuration(operation string, duration time.Duration)
	// RecordOperation records metrics for a completed operation (legacy version)
	RecordOperation(operationName string, actionName string, success bool, duration float64, tags map[string]string)
	// RecordOperationWithContext records an operation metric with a function to execute
	RecordOperationWithContext(ctx context.Context, operation string, f func() error) error
	Close() error
}

// simpleMetricsClient is a no-op implementation of the MetricsClient interface
type simpleMetricsClient struct{}

// IncrementCounter increments a counter metric
func (m *simpleMetricsClient) IncrementCounter(name string, value float64, tags map[string]string) {
	// No-op implementation
}

// RecordHistogram records a histogram metric
func (m *simpleMetricsClient) RecordHistogram(name string, value float64, tags map[string]string) {
	// No-op implementation
}

// RecordGauge records a gauge metric
func (m *simpleMetricsClient) RecordGauge(name string, value float64, tags map[string]string) {
	// No-op implementation
}

// RecordCounter increments a counter metric (alias for IncrementCounter)
func (m *simpleMetricsClient) RecordCounter(name string, value float64, tags map[string]string) {
	// Just call IncrementCounter with the same parameters
	m.IncrementCounter(name, value, tags)
}

// RecordEvent records an event metric
func (m *simpleMetricsClient) RecordEvent(source, eventType string) {
	// No-op implementation
}

// RecordLatency records a latency metric
func (m *simpleMetricsClient) RecordLatency(operation string, duration time.Duration) {
	// No-op implementation
}

// RecordDuration records a duration metric (alias for RecordLatency)
func (m *simpleMetricsClient) RecordDuration(operation string, duration time.Duration) {
	// Just call RecordLatency with the same parameters
	m.RecordLatency(operation, duration)
}

// RecordOperation records metrics for a completed operation (legacy version)
func (m *simpleMetricsClient) RecordOperation(operationName string, actionName string, success bool, duration float64, tags map[string]string) {
	// Record the operation in metrics
	m.RecordDuration(operationName+"."+actionName, time.Duration(duration*float64(time.Second)))
	
	// Record success/failure
	if success {
		m.IncrementCounter(operationName+"."+actionName+".success", 1, tags)
	} else {
		m.IncrementCounter(operationName+"."+actionName+".error", 1, tags)
	}
}

// RecordOperationWithContext records an operation metric with a function to execute
func (m *simpleMetricsClient) RecordOperationWithContext(ctx context.Context, operation string, f func() error) error {
	// Record the start time
	start := time.Now()
	
	// Execute the provided function
	err := f()
	
	// Record the duration regardless of whether there was an error
	duration := time.Since(start)
	m.RecordDuration(operation, duration)
	
	// Record success/failure
	success := err == nil
	m.RecordOperation(operation, "execute", success, duration.Seconds(), nil)
	
	return err
}

// Close closes the metrics client
func (m *simpleMetricsClient) Close() error {
	// No-op implementation
	return nil
}

// NewMetricsClient creates a new metrics client
func NewMetricsClient() MetricsClient {
	return &simpleMetricsClient{}
}

// InitTracing initializes tracing
func InitTracing(cfg Config) error {
	// Stub implementation
	return nil
}

// Shutdown shuts down all observability components
func Shutdown() {
	// Stub implementation
}

// Package observability provides backward compatibility for code that still imports
// from github.com/S-Corkum/devops-mcp/internal/observability.
//
// Deprecated: This package is being migrated to github.com/S-Corkum/devops-mcp/pkg/observability
// as part of the Go workspace migration. Please update your imports to use the new path.
package observability

import (
	"context"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/feature"
	newobs "github.com/S-Corkum/devops-mcp/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Initialize default logger and metrics client using the new package
func init() {
	DefaultLogger = newobs.DefaultLogger
	if feature.IsEnabled("USE_NEW_OBSERVABILITY") {
		// Log deprecation warning
		DefaultLogger.Warn("Using deprecated internal/observability package", map[string]interface{}{
			"message": "This package will be removed in a future version. Please update imports to pkg/observability.",
		})
	}
}

// Type re-exports for backward compatibility
type (
	LogLevel         = newobs.LogLevel
	Logger           = newobs.Logger
	TracingConfig    = newobs.TracingConfig
	MetricsClient    = newobs.MetricsClient
)

// Constant re-exports for backward compatibility
var (
	LogLevelDebug = newobs.LogLevelDebug
	LogLevelInfo  = newobs.LogLevelInfo
	LogLevelWarn  = newobs.LogLevelWarn
	LogLevelError = newobs.LogLevelError
	
	// Tracing attribute keys
	ContextOperationAttributeKey = newobs.ContextOperationAttributeKey
	ContextModelAttributeKey     = newobs.ContextModelAttributeKey
	ContextTokensAttributeKey    = newobs.ContextTokensAttributeKey
	VectorOperationAttributeKey  = newobs.VectorOperationAttributeKey
	ToolNameAttributeKey         = newobs.ToolNameAttributeKey
	ToolActionAttributeKey       = newobs.ToolActionAttributeKey
)

// Re-implement functions directly delegating to the new package
func NewLogger(serviceName string) *Logger {
	return newobs.NewLogger(serviceName)
}

func NewMetricsClient() MetricsClient {
	return newobs.NewMetricsClient()
}

func InitTracing(cfg TracingConfig) (func(), error) {
	return newobs.InitTracing(cfg)
}

func GetTracer() trace.Tracer {
	return newobs.GetTracer()
}

func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	ctx, span := newobs.StartSpan(ctx, name)
	// Convert from the wrapped Span interface back to the opentelemetry Span
	// This assumes that the implementation is using the otelSpanWrapper
	return ctx, getUnderlyingSpan(span)
}

// Helper to extract underlying opentelemetry span from wrapper
func getUnderlyingSpan(span newobs.Span) trace.Span {
	// Use type assertion to get the underlying span
	// This assumes we're using the internal implementation of the Span interface
	wrapper, ok := span.(*newobs.otelSpanWrapper)
	if ok {
		return wrapper.span
	}
	
	// Fallback to a noop span if we can't get the underlying span
	return trace.NewNoopTracerProvider().Tracer("").Start(context.Background(), "noop")[1]
}

func AddSpanEvent(ctx context.Context, name string, attributes ...attribute.KeyValue) {
	newobs.AddSpanEvent(ctx, name, attributes...)
}

func SetSpanStatus(ctx context.Context, err error) {
	newobs.SetSpanStatus(ctx, err)
}

func AddSpanAttributes(ctx context.Context, attributes ...attribute.KeyValue) {
	newobs.AddSpanAttributes(ctx, attributes...)
}

func TraceContext(ctx context.Context, operation string, modelID string) (context.Context, trace.Span) {
	ctx, span := newobs.TraceContext(ctx, operation, modelID)
	return ctx, getUnderlyingSpan(span)
}

func TraceVector(ctx context.Context, operation string) (context.Context, trace.Span) {
	ctx, span := newobs.TraceVector(ctx, operation)
	return ctx, getUnderlyingSpan(span)
}

func TraceTool(ctx context.Context, tool string, action string) (context.Context, trace.Span) {
	ctx, span := newobs.TraceTool(ctx, tool, action)
	return ctx, getUnderlyingSpan(span)
}

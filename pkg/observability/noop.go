// Package observability provides unified observability functionality for the MCP system.
package observability

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// NoopSpan is a no-op implementation of the Span interface
type NoopSpan struct{}

// End is a no-op implementation
func (s *NoopSpan) End() {}

// SetAttribute is a no-op implementation
func (s *NoopSpan) SetAttribute(key string, value interface{}) {}

// AddEvent is a no-op implementation
func (s *NoopSpan) AddEvent(name string, attributes map[string]interface{}) {}

// RecordError is a no-op implementation
func (s *NoopSpan) RecordError(err error) {}

// SetStatus is a no-op implementation
func (s *NoopSpan) SetStatus(code int, description string) {}

// SpanContext is a no-op implementation
func (s *NoopSpan) SpanContext() trace.SpanContext {
	return trace.SpanContext{}
}

// TracerProvider is a no-op implementation
func (s *NoopSpan) TracerProvider() trace.TracerProvider {
	return nil
}

// NoopStartSpan is a no-op implementation of StartSpanFunc
func NoopStartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, Span) {
	return ctx, &NoopSpan{}
}

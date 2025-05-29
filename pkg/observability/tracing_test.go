package observability

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestSpanWrapper(t *testing.T) {
	// Init tracing with a disabled config to use NoOp tracer
	cfg := TracingConfig{Enabled: false}
	cleanup, err := InitTracing(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer cleanup()

	// Start a span
	ctx, span := StartSpan(context.Background(), "test-span")
	defer span.End()

	// Test span methods
	span.AddEvent("test-event", map[string]interface{}{"key": "value"})
	span.SetAttribute("attribute", "value")
	span.RecordError(errors.New("test error"))

	// Ensure context is properly set
	if ctx == nil {
		t.Error("Expected non-nil context from StartSpan")
	}
}

func TestTraceContext(t *testing.T) {
	// Init tracing with a disabled config to use NoOp tracer
	cfg := TracingConfig{Enabled: false}
	_, err := InitTracing(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}

	// Test TraceContext
	ctx, span := TraceContext(context.Background(), "test-operation", "test-model")
	defer span.End()

	// Ensure context is properly set
	if ctx == nil {
		t.Error("Expected non-nil context from TraceContext")
	}
}

func TestTraceVector(t *testing.T) {
	// Init tracing with a disabled config to use NoOp tracer
	cfg := TracingConfig{Enabled: false}
	_, err := InitTracing(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}

	// Test TraceVector
	ctx, span := TraceVector(context.Background(), "test-operation")
	defer span.End()

	// Ensure context is properly set
	if ctx == nil {
		t.Error("Expected non-nil context from TraceVector")
	}
}

func TestTraceTool(t *testing.T) {
	// Init tracing with a disabled config to use NoOp tracer
	cfg := TracingConfig{Enabled: false}
	_, err := InitTracing(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}

	// Test TraceTool
	ctx, span := TraceTool(context.Background(), "test-tool", "test-action")
	defer span.End()

	// Ensure context is properly set
	if ctx == nil {
		t.Error("Expected non-nil context from TraceTool")
	}
}

func TestAddSpanEvent(t *testing.T) {
	// Init tracing with a disabled config to use NoOp tracer
	cfg := TracingConfig{Enabled: false}
	_, err := InitTracing(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}

	// Create a span
	ctx, _ := StartSpan(context.Background(), "test-span")

	// Test AddSpanEvent - should not panic
	AddSpanEvent(ctx, "test-event", attribute.String("key", "value"))
}

func TestSetSpanStatus(t *testing.T) {
	// Init tracing with a disabled config to use NoOp tracer
	cfg := TracingConfig{Enabled: false}
	_, err := InitTracing(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}

	// Create a span
	ctx, _ := StartSpan(context.Background(), "test-span")

	// Test SetSpanStatus - should not panic
	SetSpanStatus(ctx, errors.New("test error"))
	SetSpanStatus(ctx, nil) // Should do nothing
}

func TestAddSpanAttributes(t *testing.T) {
	// Init tracing with a disabled config to use NoOp tracer
	cfg := TracingConfig{Enabled: false}
	_, err := InitTracing(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}

	// Create a span
	ctx, _ := StartSpan(context.Background(), "test-span")

	// Test AddSpanAttributes - should not panic
	AddSpanAttributes(ctx, attribute.String("key", "value"))
}

func TestRecordError(t *testing.T) {
	// Init tracing with a disabled config to use NoOp tracer
	cfg := TracingConfig{Enabled: false}
	_, err := InitTracing(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}

	// Create a span
	ctx, _ := StartSpan(context.Background(), "test-span")

	// Test RecordError - should not panic
	RecordError(ctx, errors.New("test error"))
	RecordError(ctx, nil) // Should do nothing
}

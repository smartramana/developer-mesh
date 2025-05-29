package logging

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TracingConfig holds configuration for tracing
type TracingConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	ServiceName string `mapstructure:"service_name"`
	Environment string `mapstructure:"environment"`
	Endpoint    string `mapstructure:"endpoint"`
}

var (
	tracer     trace.Tracer
	tracerInit bool
)

// InitTracing initializes OpenTelemetry tracing
func InitTracing(cfg TracingConfig) (func(), error) {
	if !cfg.Enabled {
		log.Println("Tracing is disabled")
		tracerInit = false
		return func() {}, nil
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = "mcp-server"
	}

	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	if cfg.Endpoint == "" {
		cfg.Endpoint = "localhost:4317"
	}

	ctx := context.Background()

	// Create gRPC connection to collector
	conn, err := grpc.DialContext(ctx, cfg.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	// Create OTLP exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Set global trace provider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer
	tracer = otel.Tracer(cfg.ServiceName)
	tracerInit = true

	log.Printf("Tracing initialized with service name: %s, environment: %s", cfg.ServiceName, cfg.Environment)

	// Return a cleanup function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}, nil
}

// GetTracer returns the global tracer
func GetTracer() trace.Tracer {
	if !tracerInit {
		log.Println("Warning: Tracing not initialized, operations will not be traced")
		return trace.NewNoopTracerProvider().Tracer("")
	}
	return tracer
}

// StartSpan starts a new span
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, name)
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attributes ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attributes...))
}

// SetSpanStatus sets the status of the current span
func SetSpanStatus(ctx context.Context, err error) {
	if err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attributes ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attributes...)
}

// ContextOperationAttributeKey is the attribute key for context operations
const ContextOperationAttributeKey = attribute.Key("context.operation")

// ContextModelAttributeKey is the attribute key for context model IDs
const ContextModelAttributeKey = attribute.Key("context.model_id")

// ContextTokensAttributeKey is the attribute key for context token counts
const ContextTokensAttributeKey = attribute.Key("context.tokens")

// VectorOperationAttributeKey is the attribute key for vector operations
const VectorOperationAttributeKey = attribute.Key("vector.operation")

// ToolNameAttributeKey is the attribute key for tool names
const ToolNameAttributeKey = attribute.Key("tool.name")

// ToolActionAttributeKey is the attribute key for tool actions
const ToolActionAttributeKey = attribute.Key("tool.action")

// TraceContext wraps span operations for a specific context
func TraceContext(ctx context.Context, operation string, modelID string) (context.Context, trace.Span) {
	ctx, span := StartSpan(ctx, "context."+operation)
	span.SetAttributes(
		ContextOperationAttributeKey.String(operation),
		ContextModelAttributeKey.String(modelID),
	)
	return ctx, span
}

// TraceVector wraps span operations for vector operations
func TraceVector(ctx context.Context, operation string) (context.Context, trace.Span) {
	ctx, span := StartSpan(ctx, "vector."+operation)
	span.SetAttributes(
		VectorOperationAttributeKey.String(operation),
	)
	return ctx, span
}

// TraceTool wraps span operations for tool operations
func TraceTool(ctx context.Context, tool string, action string) (context.Context, trace.Span) {
	ctx, span := StartSpan(ctx, "tool."+action)
	span.SetAttributes(
		ToolNameAttributeKey.String(tool),
		ToolActionAttributeKey.String(action),
	)
	return ctx, span
}

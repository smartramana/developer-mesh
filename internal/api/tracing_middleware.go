package api

import (
	"fmt"
	"time"

	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// TracingMiddleware adds OpenTelemetry tracing to requests
func TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timing the request
		startTime := time.Now()
		
		// Get HTTP request details
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		
		// Extract trace information from headers
		propagator := propagation.TraceContext{}
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		
		// Start a new span
		spanName := fmt.Sprintf("%s %s", method, path)
		ctx, span := observability.StartSpan(ctx, spanName)
		defer span.End()
		
		// Set span attributes
		span.SetAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("http.url", c.Request.URL.String()),
			attribute.String("http.user_agent", c.Request.UserAgent()),
			attribute.String("http.client_ip", c.ClientIP()),
		)
		
		// Store span in the context
		c.Request = c.Request.WithContext(ctx)
		
		// Process request
		c.Next()
		
		// Calculate duration
		duration := time.Since(startTime)
		
		// Get response status
		status := fmt.Sprintf("%d", c.Writer.Status())
		
		// Record metrics
		metricsClient := observability.NewMetricsClient()
		metricsClient.RecordAPIRequest(path, method, status, duration.Seconds())
		
		// Set additional span attributes
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int("http.response_size", c.Writer.Size()),
			attribute.Float64("http.duration_ms", float64(duration.Milliseconds())),
		)
		
		// Record errors
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				span.RecordError(err.Err)
			}
			span.SetStatus(oteltrace.StatusCodeError, c.Errors.Last().Error())
		} else {
			span.SetStatus(oteltrace.StatusCodeOk, "")
		}
	}
}

// AIOperationTracingMiddleware adds AI-specific tracing to requests
func AIOperationTracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get existing context
		ctx := c.Request.Context()
		
		// Extract span from context
		span := trace.SpanFromContext(ctx)
		
		// Add AI-specific attributes to span
		if contextID := c.Param("id"); contextID != "" {
			span.SetAttributes(attribute.String("ai.context.id", contextID))
		}
		
		if modelID := c.Query("model_id"); modelID != "" {
			span.SetAttributes(attribute.String("ai.model.id", modelID))
		}
		
		// Continue processing
		c.Next()
		
		// Check for custom headers that might contain AI metrics
		if tokenCount := c.GetHeader("X-AI-Token-Count"); tokenCount != "" {
			span.SetAttributes(attribute.String("ai.token_count", tokenCount))
		}
		
		if latency := c.GetHeader("X-AI-Latency"); latency != "" {
			span.SetAttributes(attribute.String("ai.latency", latency))
		}
	}
}

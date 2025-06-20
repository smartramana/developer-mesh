package observability

import (
	"context"
)

// Context keys for observability
type contextKey string

const (
	correlationIDKey contextKey = "correlation_id"
	causationIDKey   contextKey = "causation_id"
)

// GetCorrelationID gets the correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if v, ok := ctx.Value(correlationIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("correlation_id").(string); ok {
		return v
	}
	return ""
}

// GetCausationID gets the causation ID from context
func GetCausationID(ctx context.Context) string {
	if v, ok := ctx.Value(causationIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("causation_id").(string); ok {
		return v
	}
	return ""
}

// WithCorrelationID adds correlation ID to context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// WithCausationID adds causation ID to context
func WithCausationID(ctx context.Context, causationID string) context.Context {
	return context.WithValue(ctx, causationIDKey, causationID)
}

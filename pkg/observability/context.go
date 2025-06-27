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

// Context storage keys for additional metadata
const (
	tenantIDKey    contextKey = "tenant_id"
	userIDKey      contextKey = "user_id"
	requestIDKey   contextKey = "request_id"
	spanContextKey contextKey = "span_context"
	operationKey   contextKey = "operation"
)

// WithTenantID adds tenant ID to context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// GetTenantID gets the tenant ID from context
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(tenantIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("tenant_id").(string); ok {
		return v
	}
	return ""
}

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID gets the user ID from context
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("user_id").(string); ok {
		return v
	}
	return ""
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID gets the request ID from context
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("request_id").(string); ok {
		return v
	}
	return ""
}

// WithOperation adds operation name to context
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, operationKey, operation)
}

// GetOperation gets the operation name from context
func GetOperation(ctx context.Context) string {
	if v, ok := ctx.Value(operationKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("operation").(string); ok {
		return v
	}
	return ""
}

// ExtractMetadata extracts all observability metadata from context
func ExtractMetadata(ctx context.Context) map[string]string {
	metadata := make(map[string]string)

	if correlationID := GetCorrelationID(ctx); correlationID != "" {
		metadata["correlation_id"] = correlationID
	}
	if causationID := GetCausationID(ctx); causationID != "" {
		metadata["causation_id"] = causationID
	}
	if tenantID := GetTenantID(ctx); tenantID != "" {
		metadata["tenant_id"] = tenantID
	}
	if userID := GetUserID(ctx); userID != "" {
		metadata["user_id"] = userID
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		metadata["request_id"] = requestID
	}
	if operation := GetOperation(ctx); operation != "" {
		metadata["operation"] = operation
	}

	return metadata
}

// InjectMetadata injects metadata map into context
func InjectMetadata(ctx context.Context, metadata map[string]string) context.Context {
	for key, value := range metadata {
		switch key {
		case "correlation_id":
			ctx = WithCorrelationID(ctx, value)
		case "causation_id":
			ctx = WithCausationID(ctx, value)
		case "tenant_id":
			ctx = WithTenantID(ctx, value)
		case "user_id":
			ctx = WithUserID(ctx, value)
		case "request_id":
			ctx = WithRequestID(ctx, value)
		case "operation":
			ctx = WithOperation(ctx, value)
		}
	}
	return ctx
}

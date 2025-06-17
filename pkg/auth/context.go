package auth

import (
	"context"

	"github.com/google/uuid"
)

// Context keys
const (
	tenantIDKey contextKey = "tenant_id"
	userIDKey   contextKey = "user_id"
	agentIDKey  contextKey = "agent_id"
)

// GetTenantID gets the tenant ID from context
func GetTenantID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(tenantIDKey).(uuid.UUID); ok {
		return v
	}
	if v, ok := ctx.Value("tenant_id").(uuid.UUID); ok {
		return v
	}
	// Try string conversion
	if v, ok := ctx.Value("tenant_id").(string); ok {
		if id, err := uuid.Parse(v); err == nil {
			return id
		}
	}
	return uuid.Nil
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

// GetAgentID gets the agent ID from context
func GetAgentID(ctx context.Context) string {
	if v, ok := ctx.Value(agentIDKey).(string); ok {
		return v
	}
	if v, ok := ctx.Value("agent_id").(string); ok {
		return v
	}
	return ""
}

// WithTenantID adds tenant ID to context
func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithAgentID adds agent ID to context
func WithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDKey, agentID)
}
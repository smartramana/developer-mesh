package auth

import (
	"context"
)

// Define additional context keys for passthrough functionality
const (
	// PassthroughTokenKey is the key for storing passthrough token in context
	PassthroughTokenKey contextKey = "passthrough_token"
	// TokenProviderKey is the key for storing token provider in context
	TokenProviderKey contextKey = "token_provider"
	// GatewayIDKey is the key for storing gateway ID in context
	GatewayIDKey contextKey = "gateway_id"
)

// PassthroughToken represents a token to be passed to external services
type PassthroughToken struct {
	Provider string   // github, gitlab, bitbucket
	Token    string   // The actual token
	Scopes   []string // Token scopes if known
}

// WithPassthroughToken adds a passthrough token to the context
func WithPassthroughToken(ctx context.Context, token PassthroughToken) context.Context {
	return context.WithValue(ctx, PassthroughTokenKey, token)
}

// GetPassthroughToken retrieves a passthrough token from the context
func GetPassthroughToken(ctx context.Context) (*PassthroughToken, bool) {
	token, ok := ctx.Value(PassthroughTokenKey).(PassthroughToken)
	if !ok {
		return nil, false
	}
	return &token, true
}

// WithTokenProvider adds the token provider to the context
func WithTokenProvider(ctx context.Context, provider string) context.Context {
	return context.WithValue(ctx, TokenProviderKey, provider)
}

// GetTokenProvider retrieves the token provider from the context
func GetTokenProvider(ctx context.Context) (string, bool) {
	provider, ok := ctx.Value(TokenProviderKey).(string)
	return provider, ok
}

// WithGatewayID adds the gateway ID to the context
func WithGatewayID(ctx context.Context, gatewayID string) context.Context {
	return context.WithValue(ctx, GatewayIDKey, gatewayID)
}

// GetGatewayID retrieves the gateway ID from the context
func GetGatewayID(ctx context.Context) (string, bool) {
	gatewayID, ok := ctx.Value(GatewayIDKey).(string)
	return gatewayID, ok
}

// ValidateProviderAllowed checks if a provider is in the allowed services list
func ValidateProviderAllowed(provider string, allowedServices []string) bool {
	if len(allowedServices) == 0 {
		return false
	}

	for _, allowed := range allowedServices {
		if allowed == provider {
			return true
		}
	}

	return false
}

// GetPassthroughTokenFromGin retrieves a passthrough token from Gin context
// This is an alternative to GetPassthroughToken for use in Gin handlers
func GetPassthroughTokenFromGin(c interface{}) (*PassthroughToken, bool) {
	// Type assert to gin.Context
	type ginContext interface {
		Get(key string) (value interface{}, exists bool)
	}

	gc, ok := c.(ginContext)
	if !ok {
		return nil, false
	}

	tokenInterface, exists := gc.Get("passthrough_token")
	if !exists {
		return nil, false
	}

	token, ok := tokenInterface.(PassthroughToken)
	if !ok {
		return nil, false
	}

	return &token, true
}

// ExtractAllowedServices extracts allowed services from user metadata
func ExtractAllowedServices(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}

	// Try to extract as []string first
	if services, ok := metadata["allowed_services"].([]string); ok {
		return services
	}

	// Try to extract as []interface{} and convert
	if servicesInterface, ok := metadata["allowed_services"].([]interface{}); ok {
		services := make([]string, 0, len(servicesInterface))
		for _, s := range servicesInterface {
			if str, ok := s.(string); ok {
				services = append(services, str)
			}
		}
		return services
	}

	return nil
}

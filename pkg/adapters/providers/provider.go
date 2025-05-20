package providers

import (
	"context"
)

// Provider defines the interface for adapter providers
type Provider interface {
	// Initialize initializes the provider
	Initialize(ctx context.Context) error
	
	// GetName returns the provider name
	GetName() string
	
	// GetCapabilities returns the provider capabilities
	GetCapabilities() []string
	
	// Close releases any resources used by the provider
	Close() error
}

// ProviderConfig holds configuration for providers
type ProviderConfig struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

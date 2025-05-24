package github

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/adapters"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Register registers the GitHub adapter with the factory
func Register(factory *adapters.Factory) error {
	return factory.RegisterProvider("github", New)
}

// ProviderFunc returns the provider function for creating GitHub adapters
func ProviderFunc() adapters.ProviderFunc {
	return New
}
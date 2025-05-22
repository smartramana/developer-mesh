// Package interfaces provides compatibility interfaces for various subsystems
package interfaces

// WebhookConfigInterface is an interface for webhook configuration
// This interface helps break circular dependencies between packages
type WebhookConfigInterface interface {
	// Enabled returns whether webhooks are enabled
	Enabled() bool
	
	// GitHubSecret returns the GitHub webhook secret
	GitHubSecret() string
	
	// GitHubEndpoint returns the GitHub webhook endpoint
	GitHubEndpoint() string
	
	// GitHubIPValidationEnabled returns whether GitHub IP validation is enabled
	GitHubIPValidationEnabled() bool
	
	// GitHubAllowedEvents returns the allowed GitHub webhook events
	GitHubAllowedEvents() []string
}

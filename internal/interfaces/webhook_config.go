package interfaces

// WebhookConfigInterface defines the interface for webhook configuration
type WebhookConfigInterface interface {
	// Enabled returns whether webhooks are enabled
	Enabled() bool
	
	// GitHub related configuration
	GitHubSecret() string
	GitHubEndpoint() string
	GitHubIPValidationEnabled() bool
	GitHubAllowedEvents() []string
}
// Package interfaces provides compatibility interfaces for various subsystems
package interfaces

// WebhookConfig stores webhook configuration for the API server
type WebhookConfig struct {
	EnabledField             bool
	GitHubEndpointField      string
	GitHubSecretField        string
	GitHubIPValidationField  bool
	GitHubAllowedEventsField []string
}

// Enabled returns whether webhooks are enabled
func (c WebhookConfig) Enabled() bool {
	return c.EnabledField
}

// GitHubEndpoint returns the GitHub webhook endpoint
func (c WebhookConfig) GitHubEndpoint() string {
	return c.GitHubEndpointField
}

// GitHubSecret returns the GitHub webhook secret
func (c WebhookConfig) GitHubSecret() string {
	return c.GitHubSecretField
}

// GitHubIPValidationEnabled returns whether GitHub IP validation is enabled
func (c WebhookConfig) GitHubIPValidationEnabled() bool {
	return c.GitHubIPValidationField
}

// GitHubAllowedEvents returns the allowed GitHub webhook events
func (c WebhookConfig) GitHubAllowedEvents() []string {
	return c.GitHubAllowedEventsField
}

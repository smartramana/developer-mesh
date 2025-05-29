package config

// WebhookConfig holds webhook configuration settings
type WebhookConfig struct {
	// General webhook settings
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	
	// GitHub-specific settings
	GitHub GitHubWebhookConfig `mapstructure:"github" json:"github"`
}

// GitHubWebhookConfig holds GitHub-specific webhook configuration
type GitHubWebhookConfig struct {
	Enabled           bool     `mapstructure:"enabled" json:"enabled"`
	Endpoint          string   `mapstructure:"endpoint" json:"endpoint"`
	Secret            string   `mapstructure:"secret" json:"secret"`
	IPValidation      bool     `mapstructure:"ip_validation" json:"ip_validation"`
	AllowedEvents     []string `mapstructure:"allowed_events" json:"allowed_events"`
}

// Methods to implement the webhooks.WebhookConfig interface

// GitHubSecret returns the GitHub webhook secret
func (w *WebhookConfig) GitHubSecret() string {
	if w == nil || !w.Enabled || !w.GitHub.Enabled {
		return ""
	}
	return w.GitHub.Secret
}

// GitHubAllowedEvents returns the list of allowed GitHub events
func (w *WebhookConfig) GitHubAllowedEvents() []string {
	if w == nil || !w.Enabled || !w.GitHub.Enabled {
		return []string{}
	}
	return w.GitHub.AllowedEvents
}

// GitHubIPValidationEnabled returns whether IP validation is enabled
func (w *WebhookConfig) GitHubIPValidationEnabled() bool {
	if w == nil || !w.Enabled || !w.GitHub.Enabled {
		return false
	}
	return w.GitHub.IPValidation
}

// GitHubEndpoint returns the GitHub webhook endpoint
func (w *WebhookConfig) GitHubEndpoint() string {
	if w == nil || !w.Enabled || !w.GitHub.Enabled {
		return ""
	}
	return w.GitHub.Endpoint
}

// IsEnabled returns whether webhooks are enabled
func (w *WebhookConfig) IsEnabled() bool {
	return w != nil && w.Enabled
}

// IsGitHubEnabled returns whether GitHub webhooks are enabled
func (w *WebhookConfig) IsGitHubEnabled() bool {
	return w != nil && w.Enabled && w.GitHub.Enabled && w.GitHub.Endpoint != ""
}
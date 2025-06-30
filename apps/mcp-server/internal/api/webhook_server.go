package api

import (
	"net/http"

	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/api/webhooks"

	"github.com/gorilla/mux"
)

// WebhookProvider represents a webhook provider's registration logic
// Extend this struct for additional providers (e.g., GitLab, Bitbucket)
type WebhookProvider struct {
	Name       string
	Enabled    func() bool
	Endpoint   func() string
	Handler    func() http.HandlerFunc
	Middleware func() mux.MiddlewareFunc // can be nil
}

// RegisterWebhookRoutes registers webhook routes for all providers on the given router
func (s *Server) RegisterWebhookRoutes(router *mux.Router) {
	// Check if webhook configuration exists
	if s.config.Webhook == nil {
		s.logger.Info("Webhook configuration not provided", nil)
		return
	}

	// Check if webhooks are enabled
	if !s.config.Webhook.IsEnabled() {
		s.logger.Info("Webhook support is disabled", nil)
		return
	}

	providers := []WebhookProvider{
		{
			Name:     "github",
			Enabled:  func() bool { return s.config.Webhook.IsGitHubEnabled() },
			Endpoint: func() string { return s.config.Webhook.GitHubEndpoint() },
			Handler:  func() http.HandlerFunc { return webhooks.GitHubWebhookHandler(s.config.Webhook, s.loggerObsAdapter) },
			Middleware: func() mux.MiddlewareFunc {
				ipValidator := webhooks.NewGitHubIPValidator(s.loggerObsAdapter)
				return webhooks.GitHubIPValidationMiddleware(ipValidator, s.config.Webhook, s.loggerObsAdapter)
			},
		},
		// Add more providers here as needed
	}

	for _, provider := range providers {
		if provider.Enabled() && provider.Endpoint() != "" {
			pathPrefix := provider.Endpoint()
			webhookRouter := router.PathPrefix(pathPrefix).Subrouter()

			if provider.Middleware != nil {
				webhookRouter.Use(provider.Middleware())
			}

			// Register both with and without trailing slash
			webhookRouter.HandleFunc("", provider.Handler()).Methods(http.MethodPost)
			webhookRouter.HandleFunc("/", provider.Handler()).Methods(http.MethodPost)

			s.logger.Info("Registered webhook endpoint", map[string]interface{}{"provider": provider.Name, "path": pathPrefix})
		}
	}
}

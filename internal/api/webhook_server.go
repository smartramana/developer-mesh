package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/S-Corkum/devops-mcp/internal/api/webhooks"
)

// WebhookProvider represents a webhook provider's registration logic
// Extend this struct for additional providers (e.g., GitLab, Bitbucket)
type WebhookProvider struct {
	Name        string
	Enabled     func() bool
	Endpoint    func() string
	Handler     func() http.HandlerFunc
	Middleware  func() mux.MiddlewareFunc // can be nil
}

// RegisterWebhookRoutes registers webhook routes for all providers on the given router
func (s *Server) RegisterWebhookRoutes(router *mux.Router) {
	if !(&s.config.Webhook).Enabled() {
		s.logger.Info("Webhook support is disabled", nil)
		return
	}

	providers := []WebhookProvider{
		{
			Name:     "github",
			Enabled:  func() bool { return (&s.config.Webhook).GitHubEndpoint() != "" },
			Endpoint: func() string { return (&s.config.Webhook).GitHubEndpoint() },
			Handler:  func() http.HandlerFunc { return webhooks.GitHubWebhookHandler(&s.config.Webhook, s.logger) },
			Middleware: func() mux.MiddlewareFunc {
				ipValidator := webhooks.NewGitHubIPValidator(s.logger)
				return webhooks.GitHubIPValidationMiddleware(ipValidator, &s.config.Webhook, s.logger)
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

			webhookRouter.HandleFunc("", provider.Handler()).Methods(http.MethodPost)

			s.logger.Info("Registered webhook endpoint", map[string]interface{}{"provider": provider.Name, "path": pathPrefix})
		}
	}
}
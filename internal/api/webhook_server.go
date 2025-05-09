package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// RegisterWebhookRoutes registers webhook routes on the given router
func (s *Server) RegisterWebhookRoutes(router *mux.Router) {
	// Check if webhooks are configured
	if s.config.Webhook == nil || !s.config.Webhook.Enabled() {
		log.Info().Msg("Webhook support is disabled")
		return
	}

	// GitHub webhook
	if s.config.Webhook.GitHubEndpoint() != "" {
		// Create IP validator
		ipValidator := NewGitHubIPValidator()

		// Create webhook handler
		webhookHandler := GitHubWebhookHandler(s.config.Webhook)

		// Create a subrouter for the webhook endpoint
		pathPrefix := s.config.Webhook.GitHubEndpoint()
		webhookRouter := router.PathPrefix(pathPrefix).Subrouter()

		// Apply IP validation middleware
		webhookRouter.Use(GitHubIPValidationMiddleware(ipValidator, s.config.Webhook))

		// Register the handler for POST requests
		webhookRouter.HandleFunc("", webhookHandler).Methods(http.MethodPost)

		log.Info().Str("path", pathPrefix).Msg("Registered GitHub webhook endpoint")
	}
}
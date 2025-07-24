package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// WebhookInitializer handles initialization of webhook configurations from environment variables
type WebhookInitializer struct {
	repo   repository.WebhookConfigRepository
	logger observability.Logger
}

// NewWebhookInitializer creates a new webhook initializer
func NewWebhookInitializer(repo repository.WebhookConfigRepository, logger observability.Logger) *WebhookInitializer {
	if logger == nil {
		logger = observability.NewLogger("webhook-initializer")
	}
	return &WebhookInitializer{
		repo:   repo,
		logger: logger,
	}
}

// InitializeFromEnvironment initializes webhook configurations from environment variables
// It looks for environment variables in the format:
// - WEBHOOK_ORG_<NAME>_SECRET: The webhook secret for the organization
// - WEBHOOK_ORG_<NAME>_EVENTS: Comma-separated list of allowed events (optional)
//
// For backward compatibility, it also supports:
// - MCP_WEBHOOK_SECRET or GITHUB_WEBHOOK_SECRET for the default organization
func (s *WebhookInitializer) InitializeFromEnvironment(ctx context.Context) error {
	s.logger.Info("Initializing webhook configurations from environment", nil)

	// First, handle backward compatibility - check for default webhook secret
	defaultSecret := os.Getenv("MCP_WEBHOOK_SECRET")
	if defaultSecret == "" {
		defaultSecret = os.Getenv("GITHUB_WEBHOOK_SECRET")
	}

	if defaultSecret != "" {
		// Get default organization name from environment or use "developer-mesh"
		defaultOrg := os.Getenv("GITHUB_DEFAULT_ORG")
		if defaultOrg == "" {
			defaultOrg = "developer-mesh"
		}

		// Check if configuration already exists
		existing, err := s.repo.GetByOrganization(ctx, defaultOrg)
		if err != nil {
			// Create new configuration
			s.logger.Info("Creating webhook configuration for default organization", map[string]any{
				"organization": defaultOrg,
			})

			config := &models.WebhookConfigCreate{
				OrganizationName: defaultOrg,
				WebhookSecret:    defaultSecret,
			}

			// Check for allowed events
			allowedEvents := os.Getenv("MCP_GITHUB_ALLOWED_EVENTS")
			if allowedEvents != "" {
				config.AllowedEvents = strings.Split(allowedEvents, ",")
			}

			_, err = s.repo.Create(ctx, config)
			if err != nil {
				return fmt.Errorf("failed to create webhook configuration for %s: %w", defaultOrg, err)
			}
		} else {
			// Update existing configuration if secret has changed
			if existing.WebhookSecret != defaultSecret {
				s.logger.Info("Updating webhook secret for default organization", map[string]any{
					"organization": defaultOrg,
				})

				update := &models.WebhookConfigUpdate{
					WebhookSecret: &defaultSecret,
				}

				// Check if allowed events should be updated
				allowedEvents := os.Getenv("MCP_GITHUB_ALLOWED_EVENTS")
				if allowedEvents != "" {
					events := strings.Split(allowedEvents, ",")
					update.AllowedEvents = events
				}

				_, err = s.repo.Update(ctx, defaultOrg, update)
				if err != nil {
					return fmt.Errorf("failed to update webhook configuration for %s: %w", defaultOrg, err)
				}
			}
		}
	}

	// Now handle any additional organizations defined in environment
	// Look for WEBHOOK_ORG_* environment variables
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "WEBHOOK_ORG_") && strings.Contains(env, "_SECRET=") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := parts[0]
			secret := parts[1]

			// Extract organization name from key
			// Format: WEBHOOK_ORG_<NAME>_SECRET
			if strings.HasSuffix(key, "_SECRET") {
				orgKey := strings.TrimPrefix(key, "WEBHOOK_ORG_")
				orgKey = strings.TrimSuffix(orgKey, "_SECRET")

				// Convert to lowercase and replace underscores with hyphens
				orgName := strings.ToLower(strings.ReplaceAll(orgKey, "_", "-"))

				// Check if configuration already exists
				existing, err := s.repo.GetByOrganization(ctx, orgName)
				if err != nil {
					// Create new configuration
					s.logger.Info("Creating webhook configuration for organization", map[string]any{
						"organization": orgName,
					})

					config := &models.WebhookConfigCreate{
						OrganizationName: orgName,
						WebhookSecret:    secret,
					}

					// Check for allowed events
					eventsKey := fmt.Sprintf("WEBHOOK_ORG_%s_EVENTS", orgKey)
					if events := os.Getenv(eventsKey); events != "" {
						config.AllowedEvents = strings.Split(events, ",")
					}

					_, err = s.repo.Create(ctx, config)
					if err != nil {
						s.logger.Error("Failed to create webhook configuration", map[string]any{
							"organization": orgName,
							"error":        err.Error(),
						})
						// Continue with other organizations
						continue
					}
				} else {
					// Update existing configuration if secret has changed
					if existing.WebhookSecret != secret {
						s.logger.Info("Updating webhook secret for organization", map[string]any{
							"organization": orgName,
						})

						update := &models.WebhookConfigUpdate{
							WebhookSecret: &secret,
						}

						// Check if allowed events should be updated
						eventsKey := fmt.Sprintf("WEBHOOK_ORG_%s_EVENTS", orgKey)
						if events := os.Getenv(eventsKey); events != "" {
							eventList := strings.Split(events, ",")
							update.AllowedEvents = eventList
						}

						_, err = s.repo.Update(ctx, orgName, update)
						if err != nil {
							s.logger.Error("Failed to update webhook configuration", map[string]any{
								"organization": orgName,
								"error":        err.Error(),
							})
							// Continue with other organizations
							continue
						}
					}
				}
			}
		}
	}

	// List all configured organizations for logging
	configs, err := s.repo.List(ctx, false)
	if err == nil {
		s.logger.Info("Webhook configurations initialized", map[string]any{
			"count": len(configs),
			"organizations": func() []string {
				orgs := make([]string, len(configs))
				for i, c := range configs {
					orgs[i] = c.OrganizationName
				}
				return orgs
			}(),
		})
	}

	return nil
}

// InitializeDefaults ensures at least one webhook configuration exists
// This is useful for development/testing environments
func (s *WebhookInitializer) InitializeDefaults(ctx context.Context) error {
	// Check if any configurations exist
	configs, err := s.repo.List(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to list webhook configurations: %w", err)
	}

	if len(configs) == 0 {
		s.logger.Warn("No webhook configurations found, creating default", nil)

		// Create a default configuration for development
		defaultOrg := "developer-mesh"
		defaultSecret := "development-webhook-secret-change-in-production"

		config := &models.WebhookConfigCreate{
			OrganizationName: defaultOrg,
			WebhookSecret:    defaultSecret,
			AllowedEvents:    []string{"issues", "issue_comment", "pull_request", "push", "release"},
		}

		_, err = s.repo.Create(ctx, config)
		if err != nil {
			return fmt.Errorf("failed to create default webhook configuration: %w", err)
		}

		s.logger.Info("Created default webhook configuration", map[string]any{
			"organization": defaultOrg,
		})
	}

	return nil
}

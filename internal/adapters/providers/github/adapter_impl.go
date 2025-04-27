// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"errors"

	githubAdapter "github.com/S-Corkum/mcp-server/internal/adapters/github"

	"github.com/S-Corkum/mcp-server/internal/events"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// GitHubAdapter provides a convenient wrapper around the underlying GitHub adapter
type GitHubAdapter struct {
	adapter       *githubAdapter.GitHubAdapter
	config        Config
	logger        *observability.Logger
	metricsClient observability.MetricsClient
	eventBus      events.EventBusIface
}

// NewAdapter creates a new GitHub adapter instance
func NewAdapter(
	config Config,
	eventBus events.EventBusIface,
	metricsClient observability.MetricsClient,
	logger *observability.Logger,
) (*GitHubAdapter, error) {
	// Validate logger first for test compatibility
	if logger == nil {
		return nil, errors.New("logger cannot be nil")
	}
	// Validate config before proceeding
	if valid, errs := ValidateConfig(config); !valid {
		errMsg := "invalid authentication configuration"
		if len(errs) > 0 {
			errMsg += ": " + errs[0]
		}
		return nil, errors.New(errMsg)
	}

	// Convert config to the underlying adapter config
	adapterConfig := githubAdapter.DefaultConfig()
	adapterConfig.Token = config.Token
	adapterConfig.RequestTimeout = config.Timeout
	adapterConfig.BaseURL = config.BaseURL
	adapterConfig.UploadURL = config.UploadURL
	adapterConfig.AppID = config.AppID
	adapterConfig.AppPrivateKey = config.PrivateKey
	adapterConfig.AppInstallationID = config.InstallID
	adapterConfig.UseApp = (config.AppID != "" && config.PrivateKey != "")
	adapterConfig.DisableWebhooks = config.DisableWebhooks

	// Create underlying adapter
	adapter, err := githubAdapter.New(adapterConfig, logger, metricsClient, eventBus)
	if err != nil {
		return nil, err
	}

	return &GitHubAdapter{
		adapter:       adapter,
		config:        config,
		logger:        logger,
		metricsClient: metricsClient,
		eventBus:      eventBus,
	}, nil
}

// Type returns the adapter type
func (a *GitHubAdapter) Type() string {
	return "github"
}

// Version returns the adapter version
func (a *GitHubAdapter) Version() string {
	return a.adapter.Version()
}

// Health returns the adapter health status
func (a *GitHubAdapter) Health() string {
	return a.adapter.Health()
}

// ExecuteAction executes a GitHub action
func (a *GitHubAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	return a.adapter.ExecuteAction(ctx, contextID, action, params)
}

// HandleWebhook handles a GitHub webhook
func (a *GitHubAdapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	return a.adapter.HandleWebhook(ctx, eventType, payload)
}

// Close closes the adapter and releases resources
func (a *GitHubAdapter) Close() error {
	return a.adapter.Close()
}

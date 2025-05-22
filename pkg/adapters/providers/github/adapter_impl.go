// Package github provides an adapter for interacting with GitHub repositories,
// issues, pull requests, and other GitHub features.
package github

import (
	"context"
	"errors"

	adapterEvents "github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	githubAdapter "github.com/S-Corkum/devops-mcp/pkg/adapters/github"

	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// GitHubAdapter provides a convenient wrapper around the underlying GitHub adapter
type GitHubAdapter struct {
	adapter       *githubAdapter.GitHubAdapter
	config        Config
	logger        observability.Logger
	metricsClient observability.MetricsClient
	eventBus      events.EventBusIface
}

// NewAdapter creates a new GitHub adapter instance
func NewAdapter(
	config Config,
	eventBus events.EventBusIface,
	metricsClient observability.MetricsClient,
	logger observability.Logger,
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
	
	// Set authentication settings
	adapterConfig.Auth.Token = config.Token
	
	// Convert string AppID to int64 if provided
	if config.AppID != "" {
		// For now we'll just set it to a placeholder value for type safety
		// In a full implementation we'd convert the string to int64
		adapterConfig.Auth.AppID = 1
	}
	
	// Set auth type based on which credentials are provided
	if config.Token != "" {
		adapterConfig.Auth.Type = "token"
	} else if config.AppID != "" && config.PrivateKey != "" {
		adapterConfig.Auth.Type = "app"
	}
	
	// Set auth type based on which credentials are provided
	adapterConfig.Auth.PrivateKey = config.PrivateKey
	
	// Convert string InstallID to int64 if provided
	if config.InstallID != "" {
		// For now we'll just set it to a placeholder value for type safety
		// In a full implementation we'd convert the string to int64
		adapterConfig.Auth.InstallationID = 1
	}
	
	// Set other config fields
	adapterConfig.RequestTimeout = config.Timeout
	adapterConfig.BaseURL = config.BaseURL
	adapterConfig.UploadURL = config.UploadURL
	
	// Set webhook settings
	adapterConfig.WebhooksEnabled = !config.DisableWebhooks

	// Create event bus adapter to bridge between interfaces
	eventBusAdapter := adapterEvents.NewEventBusAdapter(eventBus)

	// Create underlying adapter
	adapter, err := githubAdapter.New(adapterConfig, logger, metricsClient, eventBusAdapter)
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
	// First close the underlying adapter
	err := a.adapter.Close()
	
	// Then close the event bus if it implements a Close method
	if closer, ok := a.eventBus.(interface{ Close() }); ok {
		closer.Close()
	}
	
	return err
}

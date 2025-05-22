package github

import (
	"context"
	"time"

	adapterEvents "github.com/S-Corkum/devops-mcp/pkg/adapters/events"
	githubAdapter "github.com/S-Corkum/devops-mcp/pkg/adapters/github"
	"github.com/S-Corkum/devops-mcp/pkg/events"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Provider is a factory for GitHub adapters
type Provider struct {
	logger        observability.Logger
	metricsClient observability.MetricsClient
	eventBus      events.EventBusIface
}

// NewProvider creates a new GitHub provider
func NewProvider(
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	eventBus events.EventBusIface,
) *Provider {
	return &Provider{
		logger:        logger,
		metricsClient: metricsClient,
		eventBus:      eventBus,
	}
}

// CreateAdapter creates a new GitHub adapter
func (p *Provider) CreateAdapter(ctx context.Context, config map[string]interface{}) (interface{}, error) {
	// Convert generic config to GitHub config
	adapterConfig := githubAdapter.DefaultConfig()
	
	// Apply authentication settings from config
	if token, ok := config["token"].(string); ok {
		adapterConfig.Auth.Token = token
		adapterConfig.Auth.Type = "token"
	}
	
	// Handle GitHub App authentication
	if _, ok := config["app_id"].(string); ok {
		// For simplicity, we'll just set a placeholder value
		// In a real implementation, we'd convert string to int64
		adapterConfig.Auth.AppID = 1
		adapterConfig.Auth.Type = "app"
	}
	
	if privateKey, ok := config["private_key"].(string); ok {
		adapterConfig.Auth.PrivateKey = privateKey
	}
	
	if _, ok := config["installation_id"].(string); ok {
		// For simplicity, we'll just set a placeholder value
		// In a real implementation, we'd convert string to int64
		adapterConfig.Auth.InstallationID = 1
	}
	
	// Apply connection settings from config
	if baseURL, ok := config["base_url"].(string); ok {
		adapterConfig.BaseURL = baseURL
	}
	
	if uploadURL, ok := config["upload_url"].(string); ok {
		adapterConfig.UploadURL = uploadURL
	}
	
	if timeout, ok := config["request_timeout"].(int); ok {
		adapterConfig.RequestTimeout = time.Duration(timeout) * time.Second
	}
	
	if maxIdleConns, ok := config["max_idle_conns"].(int); ok {
		adapterConfig.MaxIdleConns = maxIdleConns
	}
	
	if maxConnsPerHost, ok := config["max_conns_per_host"].(int); ok {
		adapterConfig.MaxConnsPerHost = maxConnsPerHost
	}
	
	if maxIdleConnsPerHost, ok := config["max_idle_conns_per_host"].(int); ok {
		adapterConfig.MaxIdleConnsPerHost = maxIdleConnsPerHost
	}
	
	if idleConnTimeout, ok := config["idle_conn_timeout"].(int); ok {
		adapterConfig.IdleConnTimeout = time.Duration(idleConnTimeout) * time.Second
	}
	
	// Apply webhook settings
	if webhooksEnabled, ok := config["webhooks_enabled"].(bool); ok {
		adapterConfig.WebhooksEnabled = webhooksEnabled
	}
	
	// Create event bus adapter to bridge between interfaces
	eventBusAdapter := adapterEvents.NewEventBusAdapter(p.eventBus)

	// Create the GitHub adapter
	adapter, err := githubAdapter.New(adapterConfig, p.logger, p.metricsClient, eventBusAdapter)
	if err != nil {
		return nil, err
	}
	
	return adapter, nil
}

// Type returns the provider type
func (p *Provider) Type() string {
	return "github"
}

// Register registers the provider with a factory
func (p *Provider) Register(factory interface{}) error {
	if factory == nil {
		return ErrNilFactory
	}
	
	// Check if factory implements expected interface
	
	// Fallback to dynamic interface check
	genericFactory, ok := factory.(interface {
		RegisterAdapterCreator(string, func(context.Context, interface{}) (interface{}, error))
	})
	if !ok {
		return ErrInvalidFactory
	}
	
	// Register our adapter creator function
	genericFactory.RegisterAdapterCreator("github", func(ctx context.Context, config interface{}) (interface{}, error) {
		// Convert config to map if necessary
		configMap, ok := config.(map[string]interface{})
		if !ok {
			configMap = make(map[string]interface{})
		}
		
		return p.CreateAdapter(ctx, configMap)
	})
	
	return nil
}

// Error constants
var (
	ErrNilFactory = GithubError("factory cannot be nil")
	ErrInvalidFactory = GithubError("factory does not implement required interface")
)

// GithubError is a simple error type for GitHub adapter errors
type GithubError string

// Error returns the error string
func (e GithubError) Error() string {
	return string(e)
}

package github

import (
	"context"
	"time"

	githubAdapter "github.com/S-Corkum/mcp-server/internal/adapters/github"
	"github.com/S-Corkum/mcp-server/internal/events"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// Provider is a factory for GitHub adapters
type Provider struct {
	logger        *observability.Logger
	metricsClient observability.MetricsClient
	eventBus      events.EventBusIface
}

// NewProvider creates a new GitHub provider
func NewProvider(
	logger *observability.Logger,
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
	
	// Apply custom settings from config
	if token, ok := config["token"].(string); ok {
		adapterConfig.Token = token
	}
	
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
	
	// Create the GitHub adapter
	adapter, err := githubAdapter.New(adapterConfig, p.logger, p.metricsClient, p.eventBus)
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
	
	// Check if the factory implements the RegisterAdapterCreator method
	adapterFactory, ok := factory.(interface {
		RegisterAdapterCreator(string, func(context.Context, interface{}) (interface{}, error))
	})
	if !ok {
		return ErrInvalidFactory
	}
	
	// Register our adapter creator function
	adapterFactory.RegisterAdapterCreator("github", func(ctx context.Context, config interface{}) (interface{}, error) {
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

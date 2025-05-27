package rest

import (
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// Factory provides access to all REST API clients
type Factory struct {
	client *RESTClient
	logger observability.Logger
}

// NewFactory creates a new REST client factory
func NewFactory(baseURL, apiKey string, logger observability.Logger) *Factory {
	config := ClientConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Logger:  logger,
	}

	return &Factory{
		client: NewRESTClient(config),
		logger: logger,
	}
}

// Vector returns a client for the Vector API
func (f *Factory) Vector() *VectorClient {
	return NewVectorClient(f.client)
}

// Agent returns a client for the Agent API
func (f *Factory) Agent() *AgentClient {
	return NewAgentClient(f.client)
}

// Model returns a client for the Model API
func (f *Factory) Model() *ModelClient {
	return NewModelClient(f.client)
}

// Client returns the underlying REST client
func (f *Factory) Client() *RESTClient {
	return f.client
}

// NewContextClient creates a new client for the Context API
func (f *Factory) NewContextClient(logger observability.Logger) *ContextClient {
	if logger == nil {
		logger = f.logger
	}
	return NewContextClient(f.client, logger)
}

// NewSearchClient creates a new client for the Search API
func (f *Factory) NewSearchClient(logger observability.Logger) *SearchClient {
	if logger == nil {
		logger = f.logger
	}
	return NewSearchClient(f.client, logger)
}

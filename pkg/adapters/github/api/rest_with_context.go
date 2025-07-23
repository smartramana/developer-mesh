package api

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/adapters/github/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ContextAwareRESTClient wraps RESTClient to support context-aware authentication
type ContextAwareRESTClient struct {
	*RESTClient
	defaultAuthProvider auth.AuthProvider
	logger              observability.Logger
}

// NewContextAwareRESTClient creates a new context-aware REST client
func NewContextAwareRESTClient(
	restClient *RESTClient,
	defaultAuthProvider auth.AuthProvider,
	logger observability.Logger,
) *ContextAwareRESTClient {
	return &ContextAwareRESTClient{
		RESTClient:          restClient,
		defaultAuthProvider: defaultAuthProvider,
		logger:              logger,
	}
}

// Request makes a request to the GitHub API with context-aware authentication
func (c *ContextAwareRESTClient) Request(ctx context.Context, method, path string, body any, result any) error {
	// Get the appropriate auth provider for this context
	authProvider := auth.GetAuthProviderFromContext(ctx, c.defaultAuthProvider, c.logger)

	// Create a new REST client with the context-specific auth provider
	tempClient := NewRESTClient(
		c.baseURL,
		c.client,
		authProvider,
		c.rateLimitCallback,
		c.logger,
	)

	// Make the request with the temporary client
	return tempClient.Request(ctx, method, path, body, result)
}

// WithAuthProvider creates a new client with a different auth provider
func (c *ContextAwareRESTClient) WithAuthProvider(authProvider auth.AuthProvider) *ContextAwareRESTClient {
	newClient := *c
	newClient.defaultAuthProvider = authProvider
	return &newClient
}

package adapters

import (
	"context"
	"fmt"
)

// Adapter defines the core interface for all adapters
type Adapter interface {
	// Type returns the adapter type identifier
	Type() string
	
	// ExecuteAction executes an action with optional parameters
	ExecuteAction(ctx context.Context, action string, params map[string]interface{}) (interface{}, error)
	
	// HandleWebhook processes webhook events from the external service
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error

	// Health returns the health status of the adapter
	Health() string

	// Close gracefully shuts down the adapter
	Close() error
	
	// Version returns the adapter's version
	Version() string
}

// GenericAdapter wraps a SourceControlAdapter to implement the Adapter interface
type GenericAdapter struct {
	sourceControl SourceControlAdapter
	adapterType   string
	version       string
}

// NewGenericAdapter creates a generic adapter from a source control adapter
func NewGenericAdapter(sourceControl SourceControlAdapter, adapterType string, version string) *GenericAdapter {
	return &GenericAdapter{
		sourceControl: sourceControl,
		adapterType:   adapterType,
		version:       version,
	}
}

// Type returns the adapter type identifier
func (g *GenericAdapter) Type() string {
	return g.adapterType
}

// ExecuteAction executes an action with optional parameters
func (g *GenericAdapter) ExecuteAction(ctx context.Context, action string, params map[string]interface{}) (interface{}, error) {
	// Route actions to appropriate source control methods
	switch action {
	case "get_repository":
		owner, _ := params["owner"].(string)
		repo, _ := params["repo"].(string)
		return g.sourceControl.GetRepository(ctx, owner, repo)
	case "list_repositories":
		owner, _ := params["owner"].(string)
		return g.sourceControl.ListRepositories(ctx, owner)
	case "get_pull_request":
		owner, _ := params["owner"].(string)
		repo, _ := params["repo"].(string)
		number, _ := params["number"].(int)
		return g.sourceControl.GetPullRequest(ctx, owner, repo, number)
	case "list_pull_requests":
		owner, _ := params["owner"].(string)
		repo, _ := params["repo"].(string)
		return g.sourceControl.ListPullRequests(ctx, owner, repo)
	case "get_issue":
		owner, _ := params["owner"].(string)
		repo, _ := params["repo"].(string)
		number, _ := params["number"].(int)
		return g.sourceControl.GetIssue(ctx, owner, repo, number)
	case "list_issues":
		owner, _ := params["owner"].(string)
		repo, _ := params["repo"].(string)
		return g.sourceControl.ListIssues(ctx, owner, repo)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// HandleWebhook processes webhook events from the external service
func (g *GenericAdapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	return g.sourceControl.HandleWebhook(ctx, eventType, payload)
}

// Health returns the health status of the adapter
func (g *GenericAdapter) Health() string {
	ctx := context.Background()
	if err := g.sourceControl.Health(ctx); err != nil {
		return "unhealthy: " + err.Error()
	}
	return "healthy"
}

// Close gracefully shuts down the adapter
func (g *GenericAdapter) Close() error {
	// SourceControlAdapter doesn't have a Close method, so nothing to do
	return nil
}

// Version returns the adapter's version
func (g *GenericAdapter) Version() string {
	return g.version
}
package github

import (
	"context"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/adapters"
	"github.com/developer-mesh/developer-mesh/pkg/adapters/events"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Register registers the GitHub adapter with the factory
func Register(factory *adapters.Factory) error {
	return factory.RegisterProvider("github", providerWrapper)
}

// ProviderFunc returns the provider function for creating GitHub adapters
func ProviderFunc() adapters.ProviderFunc {
	return providerWrapper
}

// providerWrapper adapts the GitHub New function to the ProviderFunc signature
func providerWrapper(ctx context.Context, config adapters.Config, logger observability.Logger) (adapters.SourceControlAdapter, error) {
	// Extract GitHub-specific config from ProviderConfig
	githubConfig := &Config{}

	// Check if there's GitHub configuration in the provider config
	if config.ProviderConfig != nil {
		if ghCfg, ok := config.ProviderConfig["github"].(*Config); ok {
			githubConfig = ghCfg
		}
	}

	// Create metrics client and event bus (these would normally come from a central system)
	// For now, we'll use nil values as placeholders
	var metricsClient observability.MetricsClient
	var eventBus events.EventBus

	// Create the GitHub adapter
	adapter, err := New(githubConfig, logger, metricsClient, eventBus)
	if err != nil {
		return nil, err
	}

	// Wrap in a temporary adapter that implements missing methods
	// This is a temporary solution during migration
	return &sourceControlWrapper{adapter: adapter}, nil
}

// sourceControlWrapper is a temporary wrapper to implement SourceControlAdapter
// TODO: Remove this once GitHubAdapter fully implements SourceControlAdapter
type sourceControlWrapper struct {
	adapter *GitHubAdapter
}

// Repository operations
func (w *sourceControlWrapper) GetRepository(ctx context.Context, owner, repo string) (*adapters.Repository, error) {
	return nil, fmt.Errorf("GetRepository: not implemented")
}

func (w *sourceControlWrapper) ListRepositories(ctx context.Context, owner string) ([]*adapters.Repository, error) {
	return nil, fmt.Errorf("ListRepositories: not implemented")
}

// Pull Request operations
func (w *sourceControlWrapper) GetPullRequest(ctx context.Context, owner, repo string, number int) (*adapters.PullRequest, error) {
	return nil, fmt.Errorf("GetPullRequest: not implemented")
}

func (w *sourceControlWrapper) CreatePullRequest(ctx context.Context, owner, repo string, pr *adapters.PullRequest) (*adapters.PullRequest, error) {
	return nil, fmt.Errorf("CreatePullRequest: not implemented")
}

func (w *sourceControlWrapper) ListPullRequests(ctx context.Context, owner, repo string) ([]*adapters.PullRequest, error) {
	return nil, fmt.Errorf("ListPullRequests: not implemented")
}

// Issue operations
func (w *sourceControlWrapper) GetIssue(ctx context.Context, owner, repo string, number int) (*adapters.Issue, error) {
	return nil, fmt.Errorf("GetIssue: not implemented")
}

func (w *sourceControlWrapper) CreateIssue(ctx context.Context, owner, repo string, issue *adapters.Issue) (*adapters.Issue, error) {
	return nil, fmt.Errorf("CreateIssue: not implemented")
}

func (w *sourceControlWrapper) ListIssues(ctx context.Context, owner, repo string) ([]*adapters.Issue, error) {
	return nil, fmt.Errorf("ListIssues: not implemented")
}

// Webhook operations
func (w *sourceControlWrapper) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// For now, return not implemented
	// TODO: Implement proper webhook handling once the webhook system is fully migrated
	return fmt.Errorf("HandleWebhook: not implemented")
}

// Health check
func (w *sourceControlWrapper) Health(ctx context.Context) error {
	// Simple health check - just verify the adapter is not nil
	if w.adapter == nil {
		return fmt.Errorf("adapter is nil")
	}
	return nil
}

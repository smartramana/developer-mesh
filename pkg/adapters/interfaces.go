// Package adapters provides interfaces and implementations for external service integrations.
package adapters

import (
	"context"
	"time"
)

// Repository represents a source control repository
type Repository struct {
	ID          string
	Name        string
	Owner       string
	Description string
	URL         string
	Private     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PullRequest represents a pull request in source control
type PullRequest struct {
	ID          string
	Number      int
	Title       string
	Description string
	State       string
	Author      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Issue represents an issue in source control
type Issue struct {
	ID          string
	Number      int
	Title       string
	Description string
	State       string
	Author      string
	Labels      []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SourceControlAdapter defines operations for source control systems
type SourceControlAdapter interface {
	// Repository operations
	GetRepository(ctx context.Context, owner, repo string) (*Repository, error)
	ListRepositories(ctx context.Context, owner string) ([]*Repository, error)

	// Pull Request operations
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error)
	CreatePullRequest(ctx context.Context, owner, repo string, pr *PullRequest) (*PullRequest, error)
	ListPullRequests(ctx context.Context, owner, repo string) ([]*PullRequest, error)

	// Issue operations
	GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error)
	CreateIssue(ctx context.Context, owner, repo string, issue *Issue) (*Issue, error)
	ListIssues(ctx context.Context, owner, repo string) ([]*Issue, error)

	// Webhook operations
	HandleWebhook(ctx context.Context, eventType string, payload []byte) error

	// Health check
	Health(ctx context.Context) error
}

// BaseAdapter provides common functionality for all adapters
type BaseAdapter struct {
	Name    string
	Version string
}

// GetName returns the adapter name
func (b *BaseAdapter) GetName() string {
	return b.Name
}

// GetVersion returns the adapter version
func (b *BaseAdapter) GetVersion() string {
	return b.Version
}

// Config represents common configuration for adapters
type Config struct {
	// Common fields
	Timeout    time.Duration
	MaxRetries int
	RateLimit  int

	// Provider-specific config stored as map
	ProviderConfig map[string]any
}

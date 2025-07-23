package decorators

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/adapters"
)

// ResilienceDecorator adds resilience features to any adapter
type ResilienceDecorator struct {
	adapter    adapters.SourceControlAdapter
	maxRetries int
	timeout    time.Duration
}

// NewResilienceDecorator wraps an adapter with resilience features
func NewResilienceDecorator(adapter adapters.SourceControlAdapter, maxRetries int, timeout time.Duration) *ResilienceDecorator {
	return &ResilienceDecorator{
		adapter:    adapter,
		maxRetries: maxRetries,
		timeout:    timeout,
	}
}

// GetRepository implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) GetRepository(ctx context.Context, owner, repo string) (*adapters.Repository, error) {
	return withRetry(ctx, r.maxRetries, func() (*adapters.Repository, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return r.adapter.GetRepository(timeoutCtx, owner, repo)
	})
}

// ListRepositories implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) ListRepositories(ctx context.Context, owner string) ([]*adapters.Repository, error) {
	return withRetry(ctx, r.maxRetries, func() ([]*adapters.Repository, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return r.adapter.ListRepositories(timeoutCtx, owner)
	})
}

// GetPullRequest implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) GetPullRequest(ctx context.Context, owner, repo string, number int) (*adapters.PullRequest, error) {
	return withRetry(ctx, r.maxRetries, func() (*adapters.PullRequest, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return r.adapter.GetPullRequest(timeoutCtx, owner, repo, number)
	})
}

// CreatePullRequest implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) CreatePullRequest(ctx context.Context, owner, repo string, pr *adapters.PullRequest) (*adapters.PullRequest, error) {
	// No retry for create operations
	timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.adapter.CreatePullRequest(timeoutCtx, owner, repo, pr)
}

// ListPullRequests implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) ListPullRequests(ctx context.Context, owner, repo string) ([]*adapters.PullRequest, error) {
	return withRetry(ctx, r.maxRetries, func() ([]*adapters.PullRequest, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return r.adapter.ListPullRequests(timeoutCtx, owner, repo)
	})
}

// GetIssue implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) GetIssue(ctx context.Context, owner, repo string, number int) (*adapters.Issue, error) {
	return withRetry(ctx, r.maxRetries, func() (*adapters.Issue, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return r.adapter.GetIssue(timeoutCtx, owner, repo, number)
	})
}

// CreateIssue implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) CreateIssue(ctx context.Context, owner, repo string, issue *adapters.Issue) (*adapters.Issue, error) {
	// No retry for create operations
	timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.adapter.CreateIssue(timeoutCtx, owner, repo, issue)
}

// ListIssues implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) ListIssues(ctx context.Context, owner, repo string) ([]*adapters.Issue, error) {
	return withRetry(ctx, r.maxRetries, func() ([]*adapters.Issue, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return r.adapter.ListIssues(timeoutCtx, owner, repo)
	})
}

// HandleWebhook implements SourceControlAdapter
func (r *ResilienceDecorator) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// No retry for webhook handling
	timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.adapter.HandleWebhook(timeoutCtx, eventType, payload)
}

// Health implements SourceControlAdapter with retry logic
func (r *ResilienceDecorator) Health(ctx context.Context) error {
	return withRetryError(ctx, r.maxRetries, func() error {
		timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		return r.adapter.Health(timeoutCtx)
	})
}

// withRetry executes a function with retry logic
func withRetry[T any](ctx context.Context, maxRetries int, fn func() (T, error)) (T, error) {
	var result T
	var err error

	for i := 0; i <= maxRetries; i++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		result, err = fn()
		if err == nil {
			return result, nil
		}

		if i < maxRetries {
			// Exponential backoff
			backoff := time.Duration(1<<uint(i)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}

			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return result, ctx.Err()
			case <-timer.C:
			}
		}
	}

	return result, fmt.Errorf("after %d retries: %w", maxRetries, err)
}

// withRetryError executes a function with retry logic (error only return)
func withRetryError(ctx context.Context, maxRetries int, fn func() error) error {
	_, err := withRetry(ctx, maxRetries, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

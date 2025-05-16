package embedding

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/core"
	"github.com/S-Corkum/devops-mcp/internal/storage"
)

// GitHubContentAdapter adapts the GitHubContentManager to the GitHubContentProvider interface
type GitHubContentAdapter struct {
	// Content manager for accessing GitHub content
	contentManager *core.GitHubContentManager
}

// NewGitHubContentAdapter creates a new GitHub content adapter
func NewGitHubContentAdapter(contentManager *core.GitHubContentManager) *GitHubContentAdapter {
	return &GitHubContentAdapter{
		contentManager: contentManager,
	}
}

// GetContent retrieves file content from GitHub
func (a *GitHubContentAdapter) GetContent(ctx context.Context, owner string, repo string, path string) ([]byte, error) {
	// Call the content manager with a ContentType of "file"
	content, _, err := a.contentManager.GetContent(ctx, owner, repo, path, storage.ContentTypeFile, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}
	
	return content, nil
}

// GetIssue retrieves issue details from GitHub
func (a *GitHubContentAdapter) GetIssue(ctx context.Context, owner string, repo string, issueNumber int) (*GitHubIssue, error) {
	// Call the content manager to get issue details
	// This is a placeholder - you'll need to adapt to your actual implementation
	issue, err := a.contentManager.GetIssue(ctx, owner, repo, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue #%d: %w", issueNumber, err)
	}
	
	// Convert from the core.GitHubIssue format to our GitHubIssue format
	return &GitHubIssue{
		Title:     issue.Title,
		Body:      issue.Body,
		State:     issue.State,
		CreatedAt: issue.CreatedAt,
		UpdatedAt: issue.UpdatedAt,
	}, nil
}

// GetIssueComments retrieves issue comments from GitHub
func (a *GitHubContentAdapter) GetIssueComments(ctx context.Context, owner string, repo string, issueNumber int) ([]*GitHubComment, error) {
	// Call the content manager to get issue comments
	// This is a placeholder - you'll need to adapt to your actual implementation
	comments, err := a.contentManager.GetIssueComments(ctx, owner, repo, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for issue #%d: %w", issueNumber, err)
	}
	
	// Convert from the core.GitHubComment format to our GitHubComment format
	result := make([]*GitHubComment, len(comments))
	for i, comment := range comments {
		result[i] = &GitHubComment{
			ID:        comment.ID,
			Body:      comment.Body,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			User: struct {
				Login string `json:"login"`
			}{
				Login: comment.User.Login,
			},
		}
	}
	
	return result, nil
}

// Mock implementations for testing

// MockGitHubContentProvider implements GitHubContentProvider for testing
type MockGitHubContentProvider struct{}

// NewMockGitHubContentProvider creates a new mock GitHub content provider
func NewMockGitHubContentProvider() *MockGitHubContentProvider {
	return &MockGitHubContentProvider{}
}

// GetContent mocks retrieving file content from GitHub
func (m *MockGitHubContentProvider) GetContent(ctx context.Context, owner string, repo string, path string) ([]byte, error) {
	// Return mock content
	return []byte(fmt.Sprintf("Mock content for %s/%s/%s", owner, repo, path)), nil
}

// GetIssue mocks retrieving issue details from GitHub
func (m *MockGitHubContentProvider) GetIssue(ctx context.Context, owner string, repo string, issueNumber int) (*GitHubIssue, error) {
	// Return mock issue
	return &GitHubIssue{
		Title:     fmt.Sprintf("Mock Issue #%d", issueNumber),
		Body:      "This is a mock issue body for testing",
		State:     "open",
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now(),
	}, nil
}

// GetIssueComments mocks retrieving issue comments from GitHub
func (m *MockGitHubContentProvider) GetIssueComments(ctx context.Context, owner string, repo string, issueNumber int) ([]*GitHubComment, error) {
	// Return mock comments
	comments := []*GitHubComment{
		{
			ID:        1,
			Body:      "This is a mock comment for testing",
			CreatedAt: time.Now().Add(-12 * time.Hour),
			UpdatedAt: time.Now(),
			User: struct {
				Login string `json:"login"`
			}{
				Login: "mock-user",
			},
		},
	}
	
	return comments, nil
}

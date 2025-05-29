package embedding

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/core"
	"github.com/S-Corkum/devops-mcp/pkg/storage"
)

// Define local versions of these types for the adapter
// The actual types used by the pipeline are in pipeline.go

// GitHubIssueData represents a GitHub issue for the adapter
type GitHubIssueData struct {
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GitHubCommentData represents a GitHub comment for the adapter
type GitHubCommentData struct {
	ID        int       `json:"id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

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
	content, _, err := a.contentManager.GetContent(ctx, owner, repo, storage.ContentTypeFile, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	return content, nil
}

// GetIssue retrieves issue details from GitHub
func (a *GitHubContentAdapter) GetIssue(ctx context.Context, owner string, repo string, issueNumber int) (*GitHubIssueData, error) {
	// This is a placeholder implementation since the content manager doesn't have GetIssue method
	// In a real implementation, you would call the actual API or another method

	// For now, return a mock issue
	return &GitHubIssueData{
		Title:     fmt.Sprintf("Issue #%d", issueNumber),
		Body:      "Issue body would be retrieved from GitHub API",
		State:     "open",
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now(),
	}, nil
}

// GetIssueComments retrieves issue comments from GitHub
func (a *GitHubContentAdapter) GetIssueComments(ctx context.Context, owner string, repo string, issueNumber int) ([]*GitHubCommentData, error) {
	// This is a placeholder implementation since the content manager doesn't have GetIssueComments method
	// In a real implementation, you would call the actual API or another method

	// For now, return a mock comment
	comments := []*GitHubCommentData{
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
func (m *MockGitHubContentProvider) GetIssue(ctx context.Context, owner string, repo string, issueNumber int) (*GitHubIssueData, error) {
	// Return mock issue
	return &GitHubIssueData{
		Title:     fmt.Sprintf("Mock Issue #%d", issueNumber),
		Body:      "This is a mock issue body for testing",
		State:     "open",
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now(),
	}, nil
}

// GetIssueComments mocks retrieving issue comments from GitHub
func (m *MockGitHubContentProvider) GetIssueComments(ctx context.Context, owner string, repo string, issueNumber int) ([]*GitHubCommentData, error) {
	// Return mock comments
	comments := []*GitHubCommentData{
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

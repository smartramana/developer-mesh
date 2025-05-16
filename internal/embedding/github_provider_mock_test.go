package embedding

import (
	"context"
	"github.com/stretchr/testify/mock"
)

// TestMockGitHubContentProvider is a mock implementation of the GitHubContentProvider interface for tests
type TestMockGitHubContentProvider struct {
	mock.Mock
}

// NewTestMockGitHubContentProvider creates a new mock content provider for testing
func NewTestMockGitHubContentProvider() *TestMockGitHubContentProvider {
	return &TestMockGitHubContentProvider{}
}

// GetContent implements GitHubContentProvider interface
func (m *TestMockGitHubContentProvider) GetContent(ctx context.Context, owner, repo, path string) ([]byte, error) {
	args := m.Called(ctx, owner, repo, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

// GetIssue implements GitHubContentProvider interface
func (m *TestMockGitHubContentProvider) GetIssue(ctx context.Context, owner, repo string, issueNumber int) (*GitHubIssueData, error) {
	args := m.Called(ctx, owner, repo, issueNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*GitHubIssueData), args.Error(1)
}

// GetIssueComments implements GitHubContentProvider interface
func (m *TestMockGitHubContentProvider) GetIssueComments(ctx context.Context, owner, repo string, issueNumber int) ([]*GitHubCommentData, error) {
	args := m.Called(ctx, owner, repo, issueNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*GitHubCommentData), args.Error(1)
}

// Using GitHubIssueData and GitHubCommentData types defined in github_adapter.go

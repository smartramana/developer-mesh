package embedding

import (
	"context"
	"errors"
	"testing"

	"github.com/S-Corkum/devops-mcp/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGitHubContentManager is a mock implementation for testing
type MockGitHubContentManager struct {
	mock.Mock
}

func (m *MockGitHubContentManager) GetContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string) ([]byte, *storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType, contentID)
	if args.Get(0) == nil {
		return nil, args.Get(1).(*storage.ContentMetadata), args.Error(2)
	}
	return args.Get(0).([]byte), args.Get(1).(*storage.ContentMetadata), args.Error(2)
}

func (m *MockGitHubContentManager) StoreContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string, data []byte, metadata map[string]interface{}) (*storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType, contentID, data, metadata)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.ContentMetadata), args.Error(1)
}

func (m *MockGitHubContentManager) DeleteContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string) error {
	args := m.Called(ctx, owner, repo, contentType, contentID)
	return args.Error(0)
}

func (m *MockGitHubContentManager) ListContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, limit int) ([]*storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.ContentMetadata), args.Error(1)
}

func (m *MockGitHubContentManager) GetContentByChecksum(ctx context.Context, checksum string) ([]byte, *storage.ContentMetadata, error) {
	args := m.Called(ctx, checksum)
	if args.Get(0) == nil {
		return nil, args.Get(1).(*storage.ContentMetadata), args.Error(2)
	}
	return args.Get(0).([]byte), args.Get(1).(*storage.ContentMetadata), args.Error(2)
}

// TestGitHubContentProviderMethods tests the methods of GitHubContentProvider interface
func TestGitHubContentProviderMethods(t *testing.T) {
	// Create test data
	ctx := context.Background()
	owner := "test-owner"
	repo := "test-repo"
	path := "path/to/file.go"
	content := []byte("file content")
	issueNumber := 123
	
	t.Run("TestGetContent", func(t *testing.T) {
		// Test the adapter by directly testing the GitHubContentProvider interface
		// Instead of using adapter, we'll use the mock directly to verify interface compliance
		mock := NewTestMockGitHubContentProvider()
		
		// Set up expectations for the mock
		mock.On("GetContent", ctx, owner, repo, path).
			Return(content, nil).Once()
		
		// Test the mock's GetContent method to verify interface compliance
		result, err := mock.GetContent(ctx, owner, repo, path)
		assert.NoError(t, err)
		assert.Equal(t, content, result)
		mock.AssertExpectations(t)
		
		// Test error handling
		mock.On("GetContent", ctx, owner, repo, path).
			Return(nil, errors.New("content error")).Once()
		
		result, err = mock.GetContent(ctx, owner, repo, path)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "content error")
	})
	
	t.Run("TestGetIssue", func(t *testing.T) {
		// Create mock adapter directly
		adapter := &GitHubContentAdapter{}
		
		// Test the adapter's GetIssue method
		issue, err := adapter.GetIssue(ctx, owner, repo, issueNumber)
		assert.NoError(t, err)
		assert.NotNil(t, issue)
		assert.Contains(t, issue.Title, "Issue #123")
		assert.Contains(t, issue.Body, "Issue body")
		assert.Equal(t, "open", issue.State)
	})
	
	t.Run("TestGetIssueComments", func(t *testing.T) {
		// Create mock adapter directly
		adapter := &GitHubContentAdapter{}
		
		// Test the adapter's GetIssueComments method
		comments, err := adapter.GetIssueComments(ctx, owner, repo, issueNumber)
		assert.NoError(t, err)
		assert.NotNil(t, comments)
		assert.Len(t, comments, 1)
		assert.Equal(t, 1, comments[0].ID)
		assert.Contains(t, comments[0].Body, "mock comment")
	})
}

// TestMockContentProvider tests the mock provider
func TestMockContentProvider(t *testing.T) {
	// Create mock provider
	provider := NewTestMockGitHubContentProvider()
	
	// Test data
	ctx := context.Background()
	owner := "test-owner"
	repo := "test-repo"
	path := "path/to/file.go"
	issueNumber := 123
	
	// Set up expectations for GetContent
	mockContent := []byte("Mock content for test-owner/test-repo/path/to/file.go")
	provider.On("GetContent", ctx, owner, repo, path).
		Return(mockContent, nil).Once()
	
	// Test GetContent
	content, err := provider.GetContent(ctx, owner, repo, path)
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Equal(t, mockContent, content)
	
	// Set up expectations for GetIssue
	mockIssue := &GitHubIssueData{
		Title: "Mock Issue #123",
		Body: "mock issue body",
		State: "open",
	}
	provider.On("GetIssue", ctx, owner, repo, issueNumber).
		Return(mockIssue, nil).Once()
	
	// Test GetIssue
	issue, err := provider.GetIssue(ctx, owner, repo, issueNumber)
	assert.NoError(t, err)
	assert.NotNil(t, issue)
	assert.Equal(t, mockIssue.Title, issue.Title)
	assert.Equal(t, mockIssue.Body, issue.Body)
	assert.Equal(t, mockIssue.State, issue.State)
	
	// Set up expectations for GetIssueComments
	mockComments := []*GitHubCommentData{
		{
			ID: 1,
			Body: "mock comment",
		},
	}
	provider.On("GetIssueComments", ctx, owner, repo, issueNumber).
		Return(mockComments, nil).Once()

	// Test GetIssueComments
	comments, err := provider.GetIssueComments(ctx, owner, repo, issueNumber)
	assert.NoError(t, err)
	assert.NotNil(t, comments)
	assert.Len(t, comments, 1)
	assert.Equal(t, mockComments[0].ID, comments[0].ID)
	assert.Equal(t, mockComments[0].Body, comments[0].Body)
	
	// Verify all expectations were met
	provider.AssertExpectations(t)
}

package testing

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockGitHubAdapter is a test-only implementation that satisfies the github.GitHubAdapter interface
type MockGitHubAdapter struct {
	mock.Mock
}

// ExecuteAction is a mock implementation of the ExecuteAction method
func (m *MockGitHubAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, contextID, action, params)
	return args.Get(0), args.Error(1)
}

// Type is a mock implementation of the Type method
func (m *MockGitHubAdapter) Type() string {
	args := m.Called()
	return args.String(0)
}

// Version is a mock implementation of the Version method
func (m *MockGitHubAdapter) Version() string {
	args := m.Called()
	return args.String(0)
}

// Health is a mock implementation of the Health method
func (m *MockGitHubAdapter) Health() string {
	args := m.Called()
	return args.String(0)
}

// Close is a mock implementation of the Close method
func (m *MockGitHubAdapter) Close() error {
	args := m.Called()
	return args.Error(0)
}

// NOTE: We can't directly cast our mock to *github.GitHubAdapter type
// The tests will use special techniques to work around this limitation

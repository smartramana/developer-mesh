// Package mocks provides mock implementations for testing
package mocks

import (
	"context"
	"fmt"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/core"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// MockContextManager is a mock implementation of the ContextManagerInterface
type MockContextManager struct {
	// ContextMap holds a map of context ID to context for testing
	ContextMap map[string]*models.Context
}

// NewMockContextManager creates a new MockContextManager
func NewMockContextManager() *MockContextManager {
	return &MockContextManager{
		ContextMap: make(map[string]*models.Context),
	}
}

// CreateContext creates a new context
func (m *MockContextManager) CreateContext(ctx context.Context, context *models.Context) (*models.Context, error) {
	if context.ID == "" {
		context.ID = "mock-context-id"
	}
	m.ContextMap[context.ID] = context
	return context, nil
}

// GetContext retrieves a context by ID
func (m *MockContextManager) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	if context, ok := m.ContextMap[contextID]; ok {
		return context, nil
	}
	return nil, fmt.Errorf("context not found: %s", contextID)
}

// UpdateContext updates an existing context
func (m *MockContextManager) UpdateContext(ctx context.Context, contextID string, context *models.Context, options *models.ContextUpdateOptions) (*models.Context, error) {
	if _, ok := m.ContextMap[contextID]; !ok {
		return nil, fmt.Errorf("context not found: %s", contextID)
	}
	m.ContextMap[contextID] = context
	return context, nil
}

// DeleteContext deletes a context by ID
func (m *MockContextManager) DeleteContext(ctx context.Context, contextID string) error {
	if _, ok := m.ContextMap[contextID]; !ok {
		return fmt.Errorf("context not found: %s", contextID)
	}
	delete(m.ContextMap, contextID)
	return nil
}

// ListContexts lists all contexts with optional filtering
func (m *MockContextManager) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*models.Context, error) {
	var contexts []*models.Context
	for _, context := range m.ContextMap {
		contexts = append(contexts, context)
	}
	return contexts, nil
}

// SearchInContext searches for items within a context
func (m *MockContextManager) SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error) {
	// Mock implementation returns empty results
	return []models.ContextItem{}, nil
}

// SummarizeContext creates a summary of a context
func (m *MockContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	return "This is a mock context summary", nil
}

// Ensure MockContextManager implements ContextManagerInterface
var _ core.ContextManagerInterface = &MockContextManager{}

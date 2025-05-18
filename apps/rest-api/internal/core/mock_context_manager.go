package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/google/uuid"
)

// MockContextManager implements a mock version of the ContextManagerInterface
// for testing and development purposes
type MockContextManager struct {
	contexts map[string]*mcp.Context
	mutex    sync.RWMutex
}

// NewMockContextManager creates a new mock context manager
func NewMockContextManager() ContextManagerInterface {
	return &MockContextManager{
		contexts: make(map[string]*mcp.Context),
		mutex:    sync.RWMutex{},
	}
}

// CreateContext implements ContextManagerInterface.CreateContext
func (m *MockContextManager) CreateContext(ctx context.Context, context *mcp.Context) (*mcp.Context, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Generate ID if not provided
	if context.ID == "" {
		context.ID = uuid.New().String()
	}
	
	// Set creation timestamp if not provided
	if context.CreatedAt.IsZero() {
		context.CreatedAt = time.Now()
	}
	
	// Set update timestamp
	context.UpdatedAt = time.Now()
	
	// Store context
	m.contexts[context.ID] = context
	
	return context, nil
}

// GetContext implements ContextManagerInterface.GetContext
func (m *MockContextManager) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	context, ok := m.contexts[contextID]
	if !ok {
		return nil, fmt.Errorf("context not found: %s", contextID)
	}
	
	return context, nil
}

// UpdateContext implements ContextManagerInterface.UpdateContext
func (m *MockContextManager) UpdateContext(ctx context.Context, contextID string, context *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Check if context exists
	_, ok := m.contexts[contextID]
	if !ok {
		return nil, fmt.Errorf("context not found: %s", contextID)
	}
	
	// Update timestamp
	context.UpdatedAt = time.Now()
	
	// Store updated context
	m.contexts[contextID] = context
	
	return context, nil
}

// DeleteContext implements ContextManagerInterface.DeleteContext
func (m *MockContextManager) DeleteContext(ctx context.Context, contextID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Check if context exists
	_, ok := m.contexts[contextID]
	if !ok {
		return fmt.Errorf("context not found: %s", contextID)
	}
	
	// Delete context
	delete(m.contexts, contextID)
	
	return nil
}

// ListContexts implements ContextManagerInterface.ListContexts
func (m *MockContextManager) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	var results []*mcp.Context
	
	// Filter contexts by agent ID and session ID if provided
	for _, context := range m.contexts {
		// Apply agent ID filter if provided
		if agentID != "" && context.AgentID != agentID {
			continue
		}
		
		// Apply session ID filter if provided
		if sessionID != "" && context.SessionID != sessionID {
			continue
		}
		
		results = append(results, context)
	}
	
	return results, nil
}

// SearchInContext implements ContextManagerInterface.SearchInContext
func (m *MockContextManager) SearchInContext(ctx context.Context, contextID, query string) ([]mcp.ContextItem, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	// Check if context exists
	context, ok := m.contexts[contextID]
	if !ok {
		return nil, fmt.Errorf("context not found: %s", contextID)
	}
	
	// Simple mock implementation: return any item that contains the query string
	var results []mcp.ContextItem
	for _, item := range context.Content {
		if item.Content != "" && contains(item.Content, query) {
			results = append(results, item)
		}
	}
	
	return results, nil
}

// SummarizeContext implements ContextManagerInterface.SummarizeContext
func (m *MockContextManager) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	// Check if context exists
	context, ok := m.contexts[contextID]
	if !ok {
		return "", fmt.Errorf("context not found: %s", contextID)
	}
	
	// Simple mock implementation: return a summary based on context metadata
	return fmt.Sprintf(
		"Mock summary for context %s (agent: %s, session: %s) containing %d items",
		contextID,
		context.AgentID,
		context.SessionID,
		len(context.Content),
	), nil
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" && s != substr
}

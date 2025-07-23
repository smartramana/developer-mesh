package proxies

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// MockContextRepository provides a temporary implementation of the ContextRepository
// interface while the mcp package is being integrated
type MockContextRepository struct {
	logger observability.Logger
}

// NewMockContextRepository creates a new mock context repository
func NewMockContextRepository(logger observability.Logger) repository.ContextRepository {
	if logger == nil {
		logger = observability.NewLogger("mock-context-repository")
	}

	return &MockContextRepository{
		logger: logger,
	}
}

// Create creates a new context
func (m *MockContextRepository) Create(ctx context.Context, contextObj *repository.Context) error {
	m.logger.Debug("Mock context creation", map[string]interface{}{
		"context_id": contextObj.ID,
	})

	// Just update timestamps
	now := time.Now().Unix()
	contextObj.CreatedAt = now
	contextObj.UpdatedAt = now

	return nil
}

// Get retrieves a context by ID
func (m *MockContextRepository) Get(ctx context.Context, id string) (*repository.Context, error) {
	m.logger.Debug("Mock context get", map[string]interface{}{
		"context_id": id,
	})

	// Return a mock context
	return &repository.Context{
		ID:         id,
		Name:       fmt.Sprintf("Mock Context %s", id),
		AgentID:    "mock-agent",
		SessionID:  "mock-session",
		Status:     "active",
		Properties: map[string]interface{}{"mock": true},
		CreatedAt:  time.Now().Unix() - 3600, // 1 hour ago
		UpdatedAt:  time.Now().Unix(),
	}, nil
}

// Update updates an existing context
func (m *MockContextRepository) Update(ctx context.Context, contextObj *repository.Context) error {
	m.logger.Debug("Mock context update", map[string]interface{}{
		"context_id": contextObj.ID,
	})

	// Just update the timestamp
	contextObj.UpdatedAt = time.Now().Unix()

	return nil
}

// Delete deletes a context by ID
func (m *MockContextRepository) Delete(ctx context.Context, id string) error {
	m.logger.Debug("Mock context delete", map[string]interface{}{
		"context_id": id,
	})

	return nil
}

// List lists contexts with optional filtering
func (m *MockContextRepository) List(ctx context.Context, filter map[string]interface{}) ([]*repository.Context, error) {
	m.logger.Debug("Mock context list", map[string]interface{}{
		"filter": filter,
	})

	// Return a single mock context
	mockContext := &repository.Context{
		ID:         "mock-context-id",
		Name:       "Mock Context",
		AgentID:    "mock-agent",
		SessionID:  "mock-session",
		Status:     "active",
		Properties: map[string]interface{}{"mock": true},
		CreatedAt:  time.Now().Unix() - 3600, // 1 hour ago
		UpdatedAt:  time.Now().Unix(),
	}

	return []*repository.Context{mockContext}, nil
}

// Search searches for text within a context
func (m *MockContextRepository) Search(ctx context.Context, contextID, query string) ([]repository.ContextItem, error) {
	m.logger.Debug("Mock context search", map[string]interface{}{
		"context_id": contextID,
		"query":      query,
	})

	// Return a mock context item
	mockItem := repository.ContextItem{
		ID:        "mock-item-id",
		ContextID: contextID,
		Content:   fmt.Sprintf("Mock content matching query: %s", query),
		Type:      "text",
		Score:     0.95,
		Metadata:  map[string]interface{}{"mock": true},
	}

	return []repository.ContextItem{mockItem}, nil
}

// Summarize generates a summary of a context
func (m *MockContextRepository) Summarize(ctx context.Context, contextID string) (string, error) {
	m.logger.Debug("Mock context summarize", map[string]interface{}{
		"context_id": contextID,
	})

	return fmt.Sprintf("This is a mock summary of context %s", contextID), nil
}

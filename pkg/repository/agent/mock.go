package agent

import (
	"context"

	"github.com/google/uuid"
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// MockRepository is a mock implementation of the Repository interface
type MockRepository struct{}

// NewMockRepository creates a new mock repository for testing
func NewMockRepository() Repository {
	return &MockRepository{}
}

// Create implements the Repository interface
func (m *MockRepository) Create(ctx context.Context, agent *models.Agent) error {
	// Mock implementation that does nothing but return success
	return nil
}

// Get implements the Repository interface
func (m *MockRepository) Get(ctx context.Context, id string) (*models.Agent, error) {
	// Mock implementation that returns a dummy agent
	return &models.Agent{
		ID:       id,
		Name:     "Mock Agent",
		TenantID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ModelID:  "mock-model",
	}, nil
}

// List implements the Repository interface
func (m *MockRepository) List(ctx context.Context, filter Filter) ([]*models.Agent, error) {
	// Mock implementation that returns an empty list
	return []*models.Agent{}, nil
}

// Update implements the Repository interface
func (m *MockRepository) Update(ctx context.Context, agent *models.Agent) error {
	// Mock implementation that does nothing but return success
	return nil
}

// Delete implements the Repository interface
func (m *MockRepository) Delete(ctx context.Context, id string) error {
	// Mock implementation that does nothing but return success
	return nil
}

// CreateAgent implements the API-specific method
func (m *MockRepository) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return m.Create(ctx, agent)
}

// GetAgentByID implements the API-specific method
func (m *MockRepository) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	agent, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Agent not found
	if agent == nil {
		return nil, nil
	}

	// Mock agent already has tenant ID set in the Get method, so we don't need to check it
	// The real implementation would verify the tenant ID, but for testing we'll make it work
	// by updating the tenant ID to match the requested one
	if tenantID != "" {
		tenantUUID, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, err
		}
		agent.TenantID = tenantUUID
	}

	return agent, nil
}

// ListAgents implements the API-specific method
func (m *MockRepository) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	filter := FilterFromTenantID(tenantID)
	return m.List(ctx, filter)
}

// UpdateAgent implements the API-specific method
func (m *MockRepository) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return m.Update(ctx, agent)
}

// DeleteAgent implements the API-specific method
func (m *MockRepository) DeleteAgent(ctx context.Context, id string) error {
	return m.Delete(ctx, id)
}

// GetByStatus implements the Repository interface
func (m *MockRepository) GetByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	// Mock implementation that returns empty list
	return []*models.Agent{}, nil
}

// GetWorkload implements the Repository interface
func (m *MockRepository) GetWorkload(ctx context.Context, agentID uuid.UUID) (*models.AgentWorkload, error) {
	// Mock implementation that returns dummy workload
	return &models.AgentWorkload{
		AgentID:       agentID.String(),
		ActiveTasks:   0,
		QueuedTasks:   0,
		TasksByType:   make(map[string]int),
		LoadScore:     0.0,
		EstimatedTime: 0,
	}, nil
}

// UpdateWorkload implements the Repository interface
func (m *MockRepository) UpdateWorkload(ctx context.Context, workload *models.AgentWorkload) error {
	// Mock implementation that does nothing but return success
	return nil
}

// GetLeastLoadedAgent implements the Repository interface
func (m *MockRepository) GetLeastLoadedAgent(ctx context.Context, capability models.AgentCapability) (*models.Agent, error) {
	// Mock implementation that returns a dummy agent
	return &models.Agent{
		ID:           uuid.New().String(),
		Name:         "Mock Least Loaded Agent",
		TenantID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ModelID:      "mock-model",
		Status:       string(models.AgentStatusActive),
		Capabilities: []string{string(capability)},
	}, nil
}

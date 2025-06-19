// Package repository provides a bridge to the new agent package
package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/agent"
	"github.com/jmoiron/sqlx"
)

// Using type alias to reference the interface defined in interfaces.go
// to avoid redeclaration errors in the codebase.
// This maintains compatibility with existing code while consolidating interface definitions.

// NewAgentRepositoryAdapter creates a new agent repository using the adapter pattern
func NewAgentRepositoryAdapter(db any) AgentRepository {
	// Handle different database types
	switch typedDB := db.(type) {
	case *sqlx.DB:
		// Use the new agent repository directly
		return agent.NewRepository(typedDB)
	case *sql.DB:
		// Convert sql.DB to sqlx.DB
		sqlxDB := sqlx.NewDb(typedDB, "postgres")
		return agent.NewRepository(sqlxDB)
	default:
		// Return mock implementation for other cases
		return &mockAgentRepository{}
	}
}

// mockAgentRepository provides a simple implementation for testing
type mockAgentRepository struct {
	agents map[string]*models.Agent
}

// Create implements the Create method for mockAgentRepository
func (m *mockAgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	if m.agents == nil {
		m.agents = make(map[string]*models.Agent)
	}
	m.agents[agent.ID] = agent
	return nil
}

// Get implements the Get method for mockAgentRepository
func (m *mockAgentRepository) Get(ctx context.Context, id string) (*models.Agent, error) {
	if m.agents == nil {
		return nil, nil
	}
	return m.agents[id], nil
}

// List implements the List method for mockAgentRepository
func (m *mockAgentRepository) List(ctx context.Context, filter agent.Filter) ([]*models.Agent, error) {
	var result []*models.Agent

	if m.agents == nil {
		return result, nil
	}

	for _, agent := range m.agents {
		matches := true
		for key, value := range filter {
			switch key {
			case "tenant_id":
				tenantStr, ok := value.(string)
				if !ok {
					matches = false
					break
				}
				tenantUUID, err := uuid.Parse(tenantStr)
				if err != nil || agent.TenantID != tenantUUID {
					matches = false
				}
			case "id":
				if agent.ID != value.(string) {
					matches = false
				}
			}
		}

		if matches {
			result = append(result, agent)
		}
	}

	return result, nil
}

// Update implements the Update method for mockAgentRepository
func (m *mockAgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	if m.agents == nil {
		m.agents = make(map[string]*models.Agent)
	}
	m.agents[agent.ID] = agent
	return nil
}

// Delete implements the Delete method for mockAgentRepository
func (m *mockAgentRepository) Delete(ctx context.Context, id string) error {
	if m.agents != nil {
		delete(m.agents, id)
	}
	return nil
}

// CreateAgent implements the API-specific method
func (m *mockAgentRepository) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return m.Create(ctx, agent)
}

// GetAgentByID implements the API-specific method
func (m *mockAgentRepository) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	agent, _ := m.Get(ctx, id)
	if agent != nil && tenantID != "" {
		tenantUUID, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, err
		}
		if agent.TenantID != tenantUUID {
			return nil, nil
		}
	}
	return agent, nil
}

// ListAgents implements the API-specific method
func (m *mockAgentRepository) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	filter := agent.FilterFromTenantID(tenantID)
	return m.List(ctx, filter)
}

// UpdateAgent implements the API-specific method
func (m *mockAgentRepository) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return m.Update(ctx, agent)
}

// DeleteAgent implements the API-specific method
func (m *mockAgentRepository) DeleteAgent(ctx context.Context, id string) error {
	return m.Delete(ctx, id)
}

// GetByStatus implements the Repository interface
func (m *mockAgentRepository) GetByStatus(ctx context.Context, status models.AgentStatus) ([]*models.Agent, error) {
	var result []*models.Agent
	if m.agents == nil {
		return result, nil
	}
	
	for _, agent := range m.agents {
		if agent.Status == string(status) {
			result = append(result, agent)
		}
	}
	return result, nil
}

// GetWorkload implements the Repository interface
func (m *mockAgentRepository) GetWorkload(ctx context.Context, agentID uuid.UUID) (*models.AgentWorkload, error) {
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
func (m *mockAgentRepository) UpdateWorkload(ctx context.Context, workload *models.AgentWorkload) error {
	// Mock implementation - do nothing
	return nil
}

// GetLeastLoadedAgent implements the Repository interface
func (m *mockAgentRepository) GetLeastLoadedAgent(ctx context.Context, capability models.AgentCapability) (*models.Agent, error) {
	// Return first active agent in mock
	if m.agents == nil {
		return nil, nil
	}
	
	for _, agent := range m.agents {
		if agent.Status == string(models.AgentStatusActive) {
			return agent, nil
		}
	}
	return nil, nil
}

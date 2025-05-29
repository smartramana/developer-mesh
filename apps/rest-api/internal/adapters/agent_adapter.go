// Package adapters provides compatibility adapters for the API code
package adapters

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// AgentAdapter implements the legacy AgentRepository interface
// but delegates to the new repository.AgentRepository interface
type AgentAdapter struct {
	repo repository.AgentRepository
}

// NewAgentAdapter creates a new AgentAdapter
func NewAgentAdapter(repo repository.AgentRepository) *AgentAdapter {
	return &AgentAdapter{repo: repo}
}

// CreateAgent creates a new agent - adapter method that calls Create
func (a *AgentAdapter) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return a.repo.Create(ctx, agent)
}

// GetAgentByID retrieves an agent by ID and tenant ID
// Adapter method that calls Get and then checks the tenant ID
func (a *AgentAdapter) GetAgentByID(ctx context.Context, tenantID, id string) (*models.Agent, error) {
	agent, err := a.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// If tenantID is provided, verify that the agent belongs to the tenant
	if tenantID != "" && agent.TenantID != tenantID {
		return nil, nil // Not found for this tenant
	}

	return agent, nil
}

// ListAgents lists agents for a tenant - adapter method that calls List with tenant filter
func (a *AgentAdapter) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	filter := map[string]any{
		"tenant_id": tenantID,
	}
	return a.repo.List(ctx, filter)
}

// UpdateAgent updates an agent - adapter method that calls Update
func (a *AgentAdapter) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return a.repo.Update(ctx, agent)
}

// DeleteAgent deletes an agent - adapter method that calls Delete
func (a *AgentAdapter) DeleteAgent(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

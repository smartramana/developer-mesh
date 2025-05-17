package repository

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// The methods needed by the API code for AgentRepository
func (a *agentRepositoryAdapter) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return a.Create(ctx, agent)
}

func (a *agentRepositoryAdapter) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	// In a real implementation, we would use tenantID for access control
	return a.Get(ctx, id)
}

func (a *agentRepositoryAdapter) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	// Convert tenantID to a filter map for the underlying implementation
	filter := map[string]interface{}{"tenant_id": tenantID}
	return a.List(ctx, filter)
}

func (a *agentRepositoryAdapter) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return a.Update(ctx, agent)
}

func (a *agentRepositoryAdapter) DeleteAgent(ctx context.Context, id string) error {
	return a.Delete(ctx, id)
}

// Implementing the core repository interface methods
func (a *agentRepositoryAdapter) Create(ctx context.Context, agent *models.Agent) error {
	// Stub implementation - would actually insert into the database
	return nil
}

func (a *agentRepositoryAdapter) Get(ctx context.Context, id string) (*models.Agent, error) {
	// Stub implementation - would actually query the database
	return &models.Agent{ID: id}, nil
}

func (a *agentRepositoryAdapter) List(ctx context.Context, filter map[string]interface{}) ([]*models.Agent, error) {
	// Stub implementation - would actually query the database
	return []*models.Agent{}, nil
}

func (a *agentRepositoryAdapter) Update(ctx context.Context, agent *models.Agent) error {
	// Stub implementation - would actually update the database
	return nil
}

func (a *agentRepositoryAdapter) Delete(ctx context.Context, id string) error {
	// Stub implementation - would actually delete from the database
	return nil
}

// The methods needed by the API code for ModelRepository
func (m *modelRepositoryAdapter) CreateModel(ctx context.Context, model *models.Model) error {
	return m.Create(ctx, model)
}

func (m *modelRepositoryAdapter) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	// In a real implementation, we would use tenantID for access control
	return m.Get(ctx, id)
}

func (m *modelRepositoryAdapter) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	// Convert tenantID to a filter map for the underlying implementation
	filter := map[string]interface{}{"tenant_id": tenantID}
	return m.List(ctx, filter)
}

func (m *modelRepositoryAdapter) UpdateModel(ctx context.Context, model *models.Model) error {
	return m.Update(ctx, model)
}

func (m *modelRepositoryAdapter) DeleteModel(ctx context.Context, id string) error {
	return m.Delete(ctx, id)
}

// Implementing the core repository interface methods
func (m *modelRepositoryAdapter) Create(ctx context.Context, model *models.Model) error {
	// Stub implementation - would actually insert into the database
	return nil
}

func (m *modelRepositoryAdapter) Get(ctx context.Context, id string) (*models.Model, error) {
	// Stub implementation - would actually query the database
	return &models.Model{ID: id}, nil
}

func (m *modelRepositoryAdapter) List(ctx context.Context, filter map[string]interface{}) ([]*models.Model, error) {
	// Stub implementation - would actually query the database
	return []*models.Model{}, nil
}

func (m *modelRepositoryAdapter) Update(ctx context.Context, model *models.Model) error {
	// Stub implementation - would actually update the database
	return nil
}

func (m *modelRepositoryAdapter) Delete(ctx context.Context, id string) error {
	// Stub implementation - would actually delete from the database
	return nil
}

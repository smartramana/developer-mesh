package repository

import (
	"context"
	"database/sql"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/agent"
	"github.com/S-Corkum/devops-mcp/pkg/repository/model"
	"github.com/jmoiron/sqlx"
)

// LegacyAgentAdapter adapts the new agent.Repository to the API expectations
type LegacyAgentAdapter struct {
	repo agent.Repository
}

// NewLegacyAgentAdapter creates a new adapter for the agent repository
func NewLegacyAgentAdapter(db interface{}) AgentRepository {
	var sqlxDB *sqlx.DB
	
	switch typedDB := db.(type) {
	case *sqlx.DB:
		sqlxDB = typedDB
	case *sql.DB:
		sqlxDB = sqlx.NewDb(typedDB, "postgres")
	default:
		// For testing scenarios, we can create a mock repository
		return &LegacyAgentAdapter{repo: agent.NewMockRepository()}
	}
	
	return &LegacyAgentAdapter{repo: agent.NewRepository(sqlxDB)}
}

// The methods needed by the API code for AgentRepository
func (a *LegacyAgentAdapter) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return a.Create(ctx, agent)
}

func (a *LegacyAgentAdapter) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	// In a real implementation, we would use tenantID for access control
	return a.Get(ctx, id)
}

func (a *LegacyAgentAdapter) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	// Convert tenantID to a filter for the underlying implementation
	filter := agent.FilterFromTenantID(tenantID)
	return a.List(ctx, filter)
}

func (a *LegacyAgentAdapter) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return a.Update(ctx, agent)
}

func (a *LegacyAgentAdapter) DeleteAgent(ctx context.Context, id string) error {
	return a.Delete(ctx, id)
}

// Implementing the core repository interface methods
func (a *LegacyAgentAdapter) Create(ctx context.Context, agent *models.Agent) error {
	return a.repo.Create(ctx, agent)
}

func (a *LegacyAgentAdapter) Get(ctx context.Context, id string) (*models.Agent, error) {
	return a.repo.Get(ctx, id)
}

func (a *LegacyAgentAdapter) List(ctx context.Context, filter agent.Filter) ([]*models.Agent, error) {
	return a.repo.List(ctx, filter)
}

func (a *LegacyAgentAdapter) Update(ctx context.Context, agent *models.Agent) error {
	return a.repo.Update(ctx, agent)
}

func (a *LegacyAgentAdapter) Delete(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

// LegacyModelAdapter adapts the new model.Repository to the API expectations
type LegacyModelAdapter struct {
	repo model.Repository
}

// NewLegacyModelAdapter creates a new adapter for the model repository
func NewLegacyModelAdapter(db interface{}) ModelRepository {
	var sqlxDB *sqlx.DB
	
	switch typedDB := db.(type) {
	case *sqlx.DB:
		sqlxDB = typedDB
	case *sql.DB:
		sqlxDB = sqlx.NewDb(typedDB, "postgres")
	default:
		// For testing scenarios, we can create a mock repository
		return &LegacyModelAdapter{repo: model.NewMockRepository()}
	}
	
	return &LegacyModelAdapter{repo: model.NewRepository(sqlxDB)}
}

// The methods needed by the API code for ModelRepository
func (m *LegacyModelAdapter) CreateModel(ctx context.Context, model *models.Model) error {
	return m.Create(ctx, model)
}

func (m *LegacyModelAdapter) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	// In a real implementation, we would use tenantID for access control
	return m.Get(ctx, id)
}

func (m *LegacyModelAdapter) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	// Convert tenantID to a filter for the underlying implementation
	filter := model.FilterFromTenantID(tenantID)
	return m.List(ctx, filter)
}

func (m *LegacyModelAdapter) UpdateModel(ctx context.Context, model *models.Model) error {
	return m.Update(ctx, model)
}

func (m *LegacyModelAdapter) DeleteModel(ctx context.Context, id string) error {
	return m.Delete(ctx, id)
}

// Implementing the core repository interface methods
func (m *LegacyModelAdapter) Create(ctx context.Context, model *models.Model) error {
	return m.repo.Create(ctx, model)
}

func (m *LegacyModelAdapter) Get(ctx context.Context, id string) (*models.Model, error) {
	return m.repo.Get(ctx, id)
}

func (m *LegacyModelAdapter) List(ctx context.Context, filter model.Filter) ([]*models.Model, error) {
	return m.repo.List(ctx, filter)
}

func (m *LegacyModelAdapter) Update(ctx context.Context, model *models.Model) error {
	return m.repo.Update(ctx, model)
}

func (m *LegacyModelAdapter) Delete(ctx context.Context, id string) error {
	return m.repo.Delete(ctx, id)
}

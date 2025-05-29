package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/agent"
	"github.com/jmoiron/sqlx"
)

// AgentRepositoryImpl implements AgentRepository
type AgentRepositoryImpl struct {
	db *sqlx.DB
}

// NewAgentRepositoryLegacy creates a new AgentRepository instance using the legacy implementation
// This avoids naming conflicts with the NewAgentRepository function defined in agent_bridge.go
func NewAgentRepositoryLegacy(db *sql.DB) AgentRepository {
	// Convert the *sql.DB to *sqlx.DB
	dbx := sqlx.NewDb(db, "postgres")
	return &AgentRepositoryImpl{db: dbx}
}

// API-specific methods required by agent_api.go

// CreateAgent implements AgentRepository.CreateAgent
func (r *AgentRepositoryImpl) CreateAgent(ctx context.Context, agent *models.Agent) error {
	// Delegate to the core Create method
	return r.Create(ctx, agent)
}

// GetAgentByID implements AgentRepository.GetAgentByID
func (r *AgentRepositoryImpl) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	// Get the agent first
	agent, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify tenant access if required
	if tenantID != "" && agent.TenantID != tenantID {
		return nil, fmt.Errorf("agent does not belong to tenant: %s", tenantID)
	}

	return agent, nil
}

// ListAgents implements AgentRepository.ListAgents
func (r *AgentRepositoryImpl) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	// Create filter based on tenantID
	filter := agent.FilterFromTenantID(tenantID)

	// Delegate to the core List method
	return r.List(ctx, filter)
}

// UpdateAgent implements AgentRepository.UpdateAgent
func (r *AgentRepositoryImpl) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	// Delegate to the core Update method
	return r.Update(ctx, agent)
}

// DeleteAgent implements AgentRepository.DeleteAgent
func (r *AgentRepositoryImpl) DeleteAgent(ctx context.Context, id string) error {
	// Delegate to the core Delete method
	return r.Delete(ctx, id)
}

// Create implements AgentRepository.Create
func (r *AgentRepositoryImpl) Create(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	query := `INSERT INTO agents (id, name, tenant_id, model_id, created_at, updated_at)
              VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	_, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.Name,
		agent.TenantID,
		agent.ModelID,
	)

	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return nil
}

// Get implements AgentRepository.Get
func (r *AgentRepositoryImpl) Get(ctx context.Context, id string) (*models.Agent, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}

	query := `SELECT id, name, tenant_id, model_id, created_at, updated_at
              FROM agents WHERE id = $1`

	var agent models.Agent
	err := r.db.GetContext(ctx, &agent, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("agent not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return &agent, nil
}

// List implements AgentRepository.List
func (r *AgentRepositoryImpl) List(ctx context.Context, filter agent.Filter) ([]*models.Agent, error) {
	query := `SELECT id, name, tenant_id, model_id, created_at, updated_at FROM agents`

	// Apply filters
	var whereClause string
	var args []any
	argIndex := 1

	if filter != nil {
		for k, v := range filter {
			if whereClause == "" {
				whereClause = " WHERE "
			} else {
				whereClause += " AND "
			}
			whereClause += fmt.Sprintf("%s = $%d", k, argIndex)
			args = append(args, v)
			argIndex++
		}
	}

	query += whereClause + " ORDER BY name"

	var agents []*models.Agent
	err := r.db.SelectContext(ctx, &agents, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return agents, nil
}

// Update implements AgentRepository.Update
func (r *AgentRepositoryImpl) Update(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	query := `UPDATE agents
              SET name = $2, tenant_id = $3, model_id = $4, updated_at = CURRENT_TIMESTAMP
              WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.Name,
		agent.TenantID,
		agent.ModelID,
	)

	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", agent.ID)
	}

	return nil
}

// Delete implements AgentRepository.Delete
func (r *AgentRepositoryImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	query := `DELETE FROM agents WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}

	return nil
}

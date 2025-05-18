// Package agent provides interfaces and implementations for agent entities
package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/jmoiron/sqlx"
)

// RepositoryImpl implements the Repository interface for agents
type RepositoryImpl struct {
	db *sqlx.DB
}

// NewRepository creates a new agent repository
func NewRepository(db *sqlx.DB) Repository {
	return &RepositoryImpl{
		db: db,
	}
}

// Create creates a new agent
func (r *RepositoryImpl) Create(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}
	
	query := `INSERT INTO agents (id, name, tenant_id, model_id)
			  VALUES ($1, $2, $3, $4)`
	
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

// Get retrieves an agent by ID
func (r *RepositoryImpl) Get(ctx context.Context, id string) (*models.Agent, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}
	
	query := `SELECT id, name, tenant_id, model_id
			  FROM agents WHERE id = $1`
	
	var agent models.Agent
	err := r.db.GetContext(ctx, &agent, query, id)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	
	return &agent, nil
}

// List retrieves agents based on filter criteria
func (r *RepositoryImpl) List(ctx context.Context, filter map[string]interface{}) ([]*models.Agent, error) {
	baseQuery := `SELECT id, name, tenant_id, model_id FROM agents`
	
	// Build the WHERE clause based on filters
	whereClause := ""
	var args []interface{}
	argCount := 1
	
	if filter != nil {
		for key, value := range filter {
			if whereClause == "" {
				whereClause = " WHERE"
			} else {
				whereClause += " AND"
			}
			
			whereClause += fmt.Sprintf(" %s = $%d", key, argCount)
			args = append(args, value)
			argCount++
		}
	}
	
	// Order by name as a default sort
	query := baseQuery + whereClause + " ORDER BY name ASC"
	
	var agents []*models.Agent
	err := r.db.SelectContext(ctx, &agents, query, args...)
	
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	
	return agents, nil
}

// Update updates an existing agent
func (r *RepositoryImpl) Update(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}
	
	if agent.ID == "" {
		return errors.New("agent ID cannot be empty")
	}
	
	query := `UPDATE agents 
			  SET name = $2, tenant_id = $3, model_id = $4
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
		return errors.New("agent not found")
	}
	
	return nil
}

// Delete deletes an agent by ID
func (r *RepositoryImpl) Delete(ctx context.Context, id string) error {
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
		return errors.New("agent not found")
	}
	
	return nil
}

// CreateAgent implements the API-specific method
func (r *RepositoryImpl) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return r.Create(ctx, agent)
}

// GetAgentByID implements the API-specific method
func (r *RepositoryImpl) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	// Get agent by ID first
	agent, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// If found, verify tenant ID matches
	if agent != nil && agent.TenantID != tenantID {
		return nil, errors.New("agent not found for tenant")
	}
	
	return agent, nil
}

// ListAgents implements the API-specific method
func (r *RepositoryImpl) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	filter := FilterFromTenantID(tenantID)
	return r.List(ctx, filter)
}

// UpdateAgent implements the API-specific method
func (r *RepositoryImpl) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return r.Update(ctx, agent)
}

// DeleteAgent implements the API-specific method
func (r *RepositoryImpl) DeleteAgent(ctx context.Context, id string) error {
	return r.Delete(ctx, id)
}

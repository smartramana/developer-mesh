package repository

import (
	"context"
	"database/sql"
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

type AgentRepository interface {
	CreateAgent(ctx context.Context, agent *models.Agent) error
	ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error)
	UpdateAgent(ctx context.Context, agent *models.Agent) error
	GetAgentByID(ctx context.Context, tenantID, agentID string) (*models.Agent, error)
}

type agentRepository struct {
	db *sql.DB
}

func NewAgentRepository(db *sql.DB) AgentRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) CreateAgent(ctx context.Context, agent *models.Agent) error {
	query := `INSERT INTO mcp.agents (id, tenant_id, name, model_id) VALUES ($1, $2, $3, $4)`
	_, err := r.db.ExecContext(ctx, query, agent.ID, agent.TenantID, agent.Name, agent.ModelID)
	return err
}

func (r *agentRepository) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	query := `SELECT id, tenant_id, name, model_id FROM mcp.agents WHERE tenant_id = $1`
	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []*models.Agent
	for rows.Next() {
		agent := &models.Agent{}
		if err := rows.Scan(&agent.ID, &agent.TenantID, &agent.Name, &agent.ModelID); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

func (r *agentRepository) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	query := `UPDATE mcp.agents SET name = $1, model_id = $2 WHERE id = $3 AND tenant_id = $4`
	_, err := r.db.ExecContext(ctx, query, agent.Name, agent.ModelID, agent.ID, agent.TenantID)
	return err
}

func (r *agentRepository) GetAgentByID(ctx context.Context, tenantID, agentID string) (*models.Agent, error) {
	query := `SELECT id, tenant_id, name, model_id FROM mcp.agents WHERE id = $1 AND tenant_id = $2`
	row := r.db.QueryRowContext(ctx, query, agentID, tenantID)
	agent := &models.Agent{}
	if err := row.Scan(&agent.ID, &agent.TenantID, &agent.Name, &agent.ModelID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return agent, nil
}

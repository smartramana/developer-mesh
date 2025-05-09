package repository

import (
	"context"
	"database/sql"
	"github.com/S-Corkum/devops-mcp/pkg/models"
)

type ModelRepository interface {
	CreateModel(ctx context.Context, model *models.Model) error
	ListModels(ctx context.Context, tenantID string) ([]*models.Model, error)
	UpdateModel(ctx context.Context, model *models.Model) error
	GetModelByID(ctx context.Context, tenantID, modelID string) (*models.Model, error)
}

type modelRepository struct {
	db *sql.DB
}

func NewModelRepository(db *sql.DB) ModelRepository {
	return &modelRepository{db: db}
}

func (r *modelRepository) CreateModel(ctx context.Context, model *models.Model) error {
	query := `INSERT INTO mcp.models (id, tenant_id, name) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, model.ID, model.TenantID, model.Name)
	return err
}

func (r *modelRepository) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	query := `SELECT id, tenant_id, name FROM mcp.models WHERE tenant_id = $1`
	rows, err := r.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var modelsList []*models.Model
	for rows.Next() {
		model := &models.Model{}
		if err := rows.Scan(&model.ID, &model.TenantID, &model.Name); err != nil {
			return nil, err
		}
		modelsList = append(modelsList, model)
	}
	return modelsList, nil
}

func (r *modelRepository) UpdateModel(ctx context.Context, model *models.Model) error {
	query := `UPDATE mcp.models SET name = $1 WHERE id = $2 AND tenant_id = $3`
	_, err := r.db.ExecContext(ctx, query, model.Name, model.ID, model.TenantID)
	return err
}

func (r *modelRepository) GetModelByID(ctx context.Context, tenantID, modelID string) (*models.Model, error) {
	query := `SELECT id, tenant_id, name FROM mcp.models WHERE id = $1 AND tenant_id = $2`
	row := r.db.QueryRowContext(ctx, query, modelID, tenantID)
	model := &models.Model{}
	if err := row.Scan(&model.ID, &model.TenantID, &model.Name); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return model, nil
}

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/repository/model"
	"github.com/jmoiron/sqlx"
)

// ModelRepositoryImpl implements ModelRepository
type ModelRepositoryImpl struct {
	db *sqlx.DB
}

// NewModelRepository creates a new ModelRepository instance
func NewModelRepository(db *sql.DB) ModelRepository {
	// Convert the *sql.DB to *sqlx.DB
	dbx := sqlx.NewDb(db, "postgres")
	// Use the model.NewRepository which handles database detection
	return model.NewRepository(dbx)
}

// API-specific methods required by model_api.go

// CreateModel implements ModelRepository.CreateModel
func (r *ModelRepositoryImpl) CreateModel(ctx context.Context, model *models.Model) error {
	// Delegate to the core Create method
	return r.Create(ctx, model)
}

// GetModelByID implements ModelRepository.GetModelByID
func (r *ModelRepositoryImpl) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	// Get the model first
	model, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify tenant access if required
	if tenantID != "" && model.TenantID != tenantID {
		return nil, fmt.Errorf("model does not belong to tenant: %s", tenantID)
	}

	return model, nil
}

// ListModels implements ModelRepository.ListModels
func (r *ModelRepositoryImpl) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	// Create filter based on tenantID
	filter := model.FilterFromTenantID(tenantID)

	// Delegate to the core List method
	return r.List(ctx, filter)
}

// UpdateModel implements ModelRepository.UpdateModel
func (r *ModelRepositoryImpl) UpdateModel(ctx context.Context, model *models.Model) error {
	// Delegate to the core Update method
	return r.Update(ctx, model)
}

// DeleteModel implements ModelRepository.DeleteModel
func (r *ModelRepositoryImpl) DeleteModel(ctx context.Context, id string) error {
	// Delegate to the core Delete method
	return r.Delete(ctx, id)
}

// Create implements ModelRepository.Create
func (r *ModelRepositoryImpl) Create(ctx context.Context, model *models.Model) error {
	if model == nil {
		return errors.New("model cannot be nil")
	}

	query := `INSERT INTO mcp.models (id, name, tenant_id, created_at, updated_at)
              VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`

	_, err := r.db.ExecContext(ctx, query,
		model.ID,
		model.Name,
		model.TenantID,
	)

	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}

	return nil
}

// Get implements ModelRepository.Get
func (r *ModelRepositoryImpl) Get(ctx context.Context, id string) (*models.Model, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}

	query := `SELECT id, name, tenant_id, created_at, updated_at
              FROM mcp.models WHERE id = $1`

	var model models.Model
	err := r.db.GetContext(ctx, &model, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("model not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	return &model, nil
}

// List implements ModelRepository.List
func (r *ModelRepositoryImpl) List(ctx context.Context, filter model.Filter) ([]*models.Model, error) {
	query := `SELECT id, name, tenant_id, created_at, updated_at FROM mcp.models`

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

	var models []*models.Model
	err := r.db.SelectContext(ctx, &models, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	return models, nil
}

// Update implements ModelRepository.Update
func (r *ModelRepositoryImpl) Update(ctx context.Context, model *models.Model) error {
	if model == nil {
		return errors.New("model cannot be nil")
	}

	query := `UPDATE mcp.models
              SET name = $2, tenant_id = $3, updated_at = CURRENT_TIMESTAMP
              WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		model.ID,
		model.Name,
		model.TenantID,
	)

	if err != nil {
		return fmt.Errorf("failed to update model: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("model not found: %s", model.ID)
	}

	return nil
}

// Delete implements ModelRepository.Delete
func (r *ModelRepositoryImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}

	query := `DELETE FROM mcp.models WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("model not found: %s", id)
	}

	return nil
}

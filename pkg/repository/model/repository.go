// Package model provides interfaces and implementations for model entities
package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/jmoiron/sqlx"
)

// RepositoryImpl implements the Repository interface for models
type RepositoryImpl struct {
	db *sqlx.DB
}

// NewRepository creates a new model repository
func NewRepository(db *sqlx.DB) Repository {
	return &RepositoryImpl{
		db: db,
	}
}

// Create creates a new model
func (r *RepositoryImpl) Create(ctx context.Context, model *models.Model) error {
	if model == nil {
		return errors.New("model cannot be nil")
	}
	
	query := `INSERT INTO models (id, name, tenant_id)
			  VALUES ($1, $2, $3)`
	
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

// Get retrieves a model by ID
func (r *RepositoryImpl) Get(ctx context.Context, id string) (*models.Model, error) {
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}
	
	query := `SELECT id, name, tenant_id
			  FROM models WHERE id = $1`
	
	var model models.Model
	err := r.db.GetContext(ctx, &model, query, id)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get model: %w", err)
	}
	
	return &model, nil
}

// List retrieves models based on filter criteria
func (r *RepositoryImpl) List(ctx context.Context, filter Filter) ([]*models.Model, error) {
	baseQuery := `SELECT id, name, tenant_id FROM models`
	
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
	
	var models []*models.Model
	err := r.db.SelectContext(ctx, &models, query, args...)
	
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	
	return models, nil
}

// Update updates an existing model
func (r *RepositoryImpl) Update(ctx context.Context, model *models.Model) error {
	if model == nil {
		return errors.New("model cannot be nil")
	}
	
	if model.ID == "" {
		return errors.New("model ID cannot be empty")
	}
	
	query := `UPDATE models 
			  SET name = $2, tenant_id = $3
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
		return errors.New("model not found")
	}
	
	return nil
}

// Delete deletes a model by ID
func (r *RepositoryImpl) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id cannot be empty")
	}
	
	query := `DELETE FROM models WHERE id = $1`
	
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return errors.New("model not found")
	}
	
	return nil
}

// CreateModel implements the API-specific method
func (r *RepositoryImpl) CreateModel(ctx context.Context, model *models.Model) error {
	return r.Create(ctx, model)
}

// GetModelByID implements the API-specific method
func (r *RepositoryImpl) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	// Get model by ID first
	model, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// If found, verify tenant ID matches
	if model != nil && model.TenantID != tenantID {
		return nil, errors.New("model not found for tenant")
	}
	
	return model, nil
}

// ListModels implements the API-specific method
func (r *RepositoryImpl) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	filter := FilterFromTenantID(tenantID)
	return r.List(ctx, filter)
}

// UpdateModel implements the API-specific method
func (r *RepositoryImpl) UpdateModel(ctx context.Context, model *models.Model) error {
	return r.Update(ctx, model)
}

// DeleteModel implements the API-specific method
func (r *RepositoryImpl) DeleteModel(ctx context.Context, id string) error {
	return r.Delete(ctx, id)
}

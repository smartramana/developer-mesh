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
	tableName string
}

// isSQLite determines if the database is SQLite
func isSQLite(db *sqlx.DB) bool {
	// Try SQLite detection by querying for version
	var version string
	err := db.QueryRow("SELECT sqlite_version()").Scan(&version)
	return err == nil && version != ""
}

// NewRepository creates a new model repository
func NewRepository(db *sqlx.DB) Repository {
	// Determine the appropriate table name with schema prefix if needed
	tableName := "models"
	
	// Check if we're using SQLite by trying a SQLite-specific pragma
	var sqliteVersion string
	err := db.QueryRow("PRAGMA database_list").Scan(&sqliteVersion)
	if err == nil {
		// SQLite detected, use no schema prefix and log for debugging
		fmt.Println("SQLite detected in model repository, using table name: models")
		tableName = "models"
	} else {
		// Try another SQLite detection method
		err = db.QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion)
		if err == nil {
			// SQLite detected via version
			fmt.Println("SQLite detected in model repository via version, using table name: models")
			tableName = "models"
		} else {
			// Not SQLite, try PostgreSQL detection
			var pgVersion string
			err = db.QueryRow("SELECT version()").Scan(&pgVersion)
			if err == nil && len(pgVersion) > 10 && pgVersion[:10] == "PostgreSQL" {
				// PostgreSQL detected, use schema prefix
				fmt.Println("PostgreSQL detected in model repository, using table name: mcp.models")
				tableName = "mcp.models"
			} else {
				// Unknown database type, default to models
				fmt.Println("Unknown database type in model repository, defaulting to table name: models")
				tableName = "models"
			}
		}
	}
	
	return &RepositoryImpl{
		db: db,
		tableName: tableName,
	}
}

// Create creates a new model
func (r *RepositoryImpl) Create(ctx context.Context, model *models.Model) error {
	if model == nil {
		return errors.New("model cannot be nil")
	}
	
	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}
	
	// Use appropriate placeholders based on database type
	var placeholders string
	if isSQLite(r.db) {
		placeholders = "?, ?, ?"
	} else {
		placeholders = "$1, $2, $3"
	}
	
	query := fmt.Sprintf("INSERT INTO %s (id, name, tenant_id) VALUES (%s)", 
		r.tableName, placeholders)
	
	// Use transaction if available
	var err error
	if ok && tx != nil {
		_, err = tx.ExecContext(ctx, query,
			model.ID,
			model.Name,
			model.TenantID,
		)
	} else {
		_, err = r.db.ExecContext(ctx, query,
			model.ID,
			model.Name,
			model.TenantID,
		)
	}
	
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
	
	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}
	
	// Use appropriate placeholder based on database type
	var placeholder string
	if isSQLite(r.db) {
		placeholder = "?"
	} else {
		placeholder = "$1"
	}
	
	query := fmt.Sprintf("SELECT id, name, tenant_id FROM %s WHERE id = %s", 
		r.tableName, placeholder)
	
	var model models.Model
	var err error
	
	// Use transaction if available
	if ok && tx != nil {
		err = tx.GetContext(ctx, &model, query, id)
	} else {
		err = r.db.GetContext(ctx, &model, query, id)
	}
	
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
	baseQuery := fmt.Sprintf("SELECT id, name, tenant_id FROM %s", r.tableName)
	
	// Build the WHERE clause based on filters
	whereClause := ""
	var args []interface{}
	
	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}
	
	if filter != nil {
		argCount := 1
		for key, value := range filter {
			if whereClause == "" {
				whereClause = " WHERE"
			} else {
				whereClause += " AND"
			}
			
			// For SQLite, always use ? without numbering
			if isSQLite(r.db) {
				whereClause += fmt.Sprintf(" %s = ?", key)
			} else {
				whereClause += fmt.Sprintf(" %s = $%d", key, argCount)
			}
			
			args = append(args, value)
			argCount++
		}
	}
	
	// Order by name as a default sort
	query := baseQuery + whereClause + " ORDER BY name ASC"
	
	var models []*models.Model
	var err error
	
	// Use transaction if available
	if ok && tx != nil {
		err = tx.SelectContext(ctx, &models, query, args...)
	} else {
		err = r.db.SelectContext(ctx, &models, query, args...)
	}
	
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
	
	// Check if we need to use a specific transaction from context
	// First try with string key
	tx, ok := ctx.Value("tx").(*sqlx.Tx)
	// If not found, try other common transaction keys
	if !ok || tx == nil {
		tx, ok = ctx.Value("TransactionKey").(*sqlx.Tx)
	}
	
	// Choose the appropriate placeholders based on database type
	var placeholders string
	if isSQLite(r.db) {
		placeholders = "name = ?, tenant_id = ? WHERE id = ?"
	} else {
		placeholders = "name = $2, tenant_id = $3 WHERE id = $1"
	}
	
	query := fmt.Sprintf("UPDATE %s SET %s", r.tableName, placeholders)
	
	var args []interface{}
	if isSQLite(r.db) {
		// SQLite uses placeholders in order of appearance
		args = []interface{}{model.Name, model.TenantID, model.ID}
	} else {
		// PostgreSQL uses numbered placeholders, maintain same order as original
		args = []interface{}{model.ID, model.Name, model.TenantID}
	}
	
	var result sql.Result
	var err error
	
	// Use transaction if available
	if ok && tx != nil {
		result, err = tx.ExecContext(ctx, query, args...)
	} else {
		result, err = r.db.ExecContext(ctx, query, args...)
	}
	
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
	
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.tableName)
	
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

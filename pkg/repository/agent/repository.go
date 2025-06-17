// Package agent provides interfaces and implementations for agent entities
package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/jmoiron/sqlx"
)

// RepositoryImpl implements the Repository interface for agents
type RepositoryImpl struct {
	db        *sqlx.DB
	tableName string
}

// isSQLite determines if the database is SQLite
func isSQLite(db *sqlx.DB) bool {
	// Try SQLite detection by querying for version
	var version string
	err := db.QueryRow("SELECT sqlite_version()").Scan(&version)
	return err == nil && version != ""
}

// NewRepository creates a new agent repository
func NewRepository(db *sqlx.DB) Repository {
	// Determine the appropriate table name with schema prefix if needed
	var tableName string

	// Try SQLite detection first (most common in tests)
	var sqliteVersion string
	err := db.QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion)
	if err == nil && sqliteVersion != "" {
		// SQLite detected, use no schema prefix
		tableName = "agents"
	} else {
		// Try PostgreSQL detection
		var pgVersion string
		err = db.QueryRow("SELECT version()").Scan(&pgVersion)
		if err == nil && len(pgVersion) > 10 && pgVersion[:10] == "PostgreSQL" {
			// PostgreSQL detected, use schema prefix
			tableName = "mcp.agents"
		} else {
			// Check the driver name as fallback
			driverName := db.DriverName()
			switch driverName {
			case "sqlite3":
				tableName = "agents"
			case "postgres", "pgx":
				tableName = "mcp.agents"
			default:
				// Default to no schema for unknown databases
				tableName = "agents"
			}
		}
	}

	return &RepositoryImpl{
		db:        db,
		tableName: tableName,
	}
}

// Create creates a new agent
func (r *RepositoryImpl) Create(ctx context.Context, agent *models.Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	// Validate required fields
	if agent.Name == "" {
		return errors.New("agent name cannot be empty")
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
		placeholders = "?, ?, ?, ?"
	} else {
		placeholders = "$1, $2, $3, $4"
	}

	query := fmt.Sprintf("INSERT INTO %s (id, name, tenant_id, model_id) VALUES (%s)",
		r.tableName, placeholders)

	// Use transaction if available
	var err error
	if ok && tx != nil {
		_, err = tx.ExecContext(ctx, query,
			agent.ID,
			agent.Name,
			agent.TenantID,
			agent.ModelID,
		)
	} else {
		_, err = r.db.ExecContext(ctx, query,
			agent.ID,
			agent.Name,
			agent.TenantID,
			agent.ModelID,
		)
	}

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

	query := fmt.Sprintf("SELECT id, name, tenant_id, model_id FROM %s WHERE id = %s",
		r.tableName, placeholder)

	var agent models.Agent
	var err error

	// Use transaction if available
	if ok && tx != nil {
		err = tx.GetContext(ctx, &agent, query, id)
	} else {
		err = r.db.GetContext(ctx, &agent, query, id)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return &agent, nil
}

// List retrieves agents based on filter criteria
func (r *RepositoryImpl) List(ctx context.Context, filter Filter) ([]*models.Agent, error) {
	baseQuery := fmt.Sprintf("SELECT id, name, tenant_id, model_id FROM %s", r.tableName)

	// Build the WHERE clause based on filters
	whereClause := ""
	var args []any

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

	var agents []*models.Agent
	var err error

	// Use transaction if available
	if ok && tx != nil {
		err = tx.SelectContext(ctx, &agents, query, args...)
	} else {
		err = r.db.SelectContext(ctx, &agents, query, args...)
	}

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
		placeholders = "name = ?, tenant_id = ?, model_id = ? WHERE id = ?"
	} else {
		placeholders = "name = $2, tenant_id = $3, model_id = $4 WHERE id = $1"
	}

	query := fmt.Sprintf("UPDATE %s SET %s", r.tableName, placeholders)

	var args []any
	if isSQLite(r.db) {
		// SQLite uses placeholders in order of appearance
		args = []any{agent.Name, agent.TenantID, agent.ModelID, agent.ID}
	} else {
		// PostgreSQL uses numbered placeholders, maintain same order as original
		args = []any{agent.ID, agent.Name, agent.TenantID, agent.ModelID}
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

	query := fmt.Sprintf("DELETE FROM %s WHERE id = %s", r.tableName, placeholder)

	var result sql.Result
	var err error

	// Use transaction if available
	if ok && tx != nil {
		result, err = tx.ExecContext(ctx, query, id)
	} else {
		result, err = r.db.ExecContext(ctx, query, id)
	}

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
	if agent != nil && tenantID != "" {
		tenantUUID, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, err
		}
		if agent.TenantID != tenantUUID {
			return nil, errors.New("agent not found for tenant")
		}
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

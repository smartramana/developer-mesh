package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// RequiredTable represents a database table that must exist
type RequiredTable struct {
	Schema      string
	TableName   string
	Description string
}

// TableHealthChecker validates required database tables exist
type TableHealthChecker struct {
	db             *sqlx.DB
	requiredTables []RequiredTable
}

// NewTableHealthChecker creates a new table health checker
func NewTableHealthChecker(db *sqlx.DB) *TableHealthChecker {
	return &TableHealthChecker{
		db: db,
		requiredTables: []RequiredTable{
			// Core MCP tables
			{Schema: "mcp", TableName: "agents", Description: "Agent configurations"},
			{Schema: "mcp", TableName: "models", Description: "Model definitions"},
			{Schema: "mcp", TableName: "contexts", Description: "Context storage"},
			{Schema: "mcp", TableName: "context_items", Description: "Context items"},
			{Schema: "mcp", TableName: "events", Description: "Event sourcing"},
			{Schema: "mcp", TableName: "api_keys", Description: "API key storage"},
			{Schema: "mcp", TableName: "users", Description: "User accounts"},
			{Schema: "mcp", TableName: "tenant_config", Description: "Tenant configurations"},

			// Tool tables
			{Schema: "mcp", TableName: "tool_configurations", Description: "Dynamic tool configs"},
			{Schema: "mcp", TableName: "tool_discovery_sessions", Description: "Tool discovery tracking"},
			{Schema: "mcp", TableName: "tool_discovery_patterns", Description: "Discovery patterns"},
			{Schema: "mcp", TableName: "tool_executions", Description: "Tool execution history"},
			{Schema: "mcp", TableName: "tool_health_checks", Description: "Health check results"},

			// Workflow tables
			{Schema: "mcp", TableName: "workflows", Description: "Workflow definitions"},
			{Schema: "mcp", TableName: "workflow_executions", Description: "Workflow runs"},
			{Schema: "mcp", TableName: "tasks", Description: "Task management"},
			{Schema: "mcp", TableName: "task_delegations", Description: "Task assignments"},

			// Webhook tables
			{Schema: "mcp", TableName: "webhook_configs", Description: "Webhook configurations"},
			{Schema: "mcp", TableName: "webhook_dlq", Description: "Dead letter queue"},

			// Embedding tables
			{Schema: "mcp", TableName: "embeddings", Description: "Vector embeddings"},
			{Schema: "mcp", TableName: "embedding_models", Description: "Embedding model configs"},
			{Schema: "mcp", TableName: "embedding_cache", Description: "Embedding cache"},

			// Audit tables
			{Schema: "mcp", TableName: "audit_log", Description: "Audit trail"},
			{Schema: "mcp", TableName: "api_key_usage", Description: "API key usage tracking"},
		},
	}
}

// TableCheckResult represents the result of a table check
type TableCheckResult struct {
	Schema      string    `json:"schema"`
	TableName   string    `json:"table_name"`
	Exists      bool      `json:"exists"`
	Error       string    `json:"error,omitempty"`
	CheckedAt   time.Time `json:"checked_at"`
	Description string    `json:"description"`
}

// DatabaseHealthResult represents overall database health
type DatabaseHealthResult struct {
	Healthy       bool               `json:"healthy"`
	TotalTables   int                `json:"total_tables"`
	MissingTables int                `json:"missing_tables"`
	Tables        []TableCheckResult `json:"tables"`
	CheckedAt     time.Time          `json:"checked_at"`
	Errors        []string           `json:"errors,omitempty"`
}

// CheckRequiredTables validates all required tables exist
func (t *TableHealthChecker) CheckRequiredTables(ctx context.Context) (*DatabaseHealthResult, error) {
	result := &DatabaseHealthResult{
		Healthy:     true,
		TotalTables: len(t.requiredTables),
		Tables:      make([]TableCheckResult, 0, len(t.requiredTables)),
		CheckedAt:   time.Now(),
		Errors:      []string{},
	}

	// Query to check if table exists
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.tables 
			WHERE table_schema = $1 
			AND table_name = $2
		)
	`

	missingTables := []string{}

	for _, table := range t.requiredTables {
		checkResult := TableCheckResult{
			Schema:      table.Schema,
			TableName:   table.TableName,
			Description: table.Description,
			CheckedAt:   time.Now(),
		}

		var exists bool
		err := t.db.QueryRowContext(ctx, query, table.Schema, table.TableName).Scan(&exists)
		if err != nil {
			checkResult.Error = err.Error()
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to check %s.%s: %v", table.Schema, table.TableName, err))
		} else {
			checkResult.Exists = exists
			if !exists {
				missingTables = append(missingTables, fmt.Sprintf("%s.%s", table.Schema, table.TableName))
				result.MissingTables++
			}
		}

		result.Tables = append(result.Tables, checkResult)
	}

	// Set overall health status
	if result.MissingTables > 0 {
		result.Healthy = false
		result.Errors = append(result.Errors, fmt.Sprintf("Missing tables: %s", strings.Join(missingTables, ", ")))
	}

	return result, nil
}

// CheckDatabaseConnectivity verifies basic database connectivity
func (t *TableHealthChecker) CheckDatabaseConnectivity(ctx context.Context) error {
	// Set a timeout for the connectivity check
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Simple query to verify connectivity
	var result int
	err := t.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database connectivity check failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected result from database connectivity check")
	}

	return nil
}

// CheckSchemaExists verifies the MCP schema exists
func (t *TableHealthChecker) CheckSchemaExists(ctx context.Context) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.schemata 
			WHERE schema_name = 'mcp'
		)
	`

	var exists bool
	err := t.db.QueryRowContext(ctx, query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check schema existence: %w", err)
	}

	return exists, nil
}

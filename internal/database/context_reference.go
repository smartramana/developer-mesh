package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/jmoiron/sqlx"
)

// ContextReference is a lightweight reference to a full context stored in S3
type ContextReference struct {
	ID          string    `db:"id"`
	AgentID     string    `db:"agent_id"`
	ModelID     string    `db:"model_id"`
	SessionID   string    `db:"session_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	ExpiresAt   time.Time `db:"expires_at"`
	TokenCount  int       `db:"token_count"`
	MessageCount int      `db:"message_count"`
	StoragePath string    `db:"storage_path"`
}

// CreateContextReference creates a new context reference in the database
func (d *Database) CreateContextReference(ctx context.Context, contextData *mcp.Context) error {
	query := `
		INSERT INTO mcp.context_references (
			id, agent_id, model_id, session_id, created_at, updated_at, expires_at, 
			token_count, message_count, storage_path
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`
	
	// Generate storage path (will be used by S3 provider)
	storagePath := fmt.Sprintf("contexts/%s.json", contextData.ID)
	
	_, err := d.db.ExecContext(
		ctx,
		query,
		contextData.ID,
		contextData.AgentID,
		contextData.ModelID,
		contextData.SessionID,
		contextData.CreatedAt,
		contextData.UpdatedAt,
		contextData.ExpiresAt,
		contextData.CurrentTokens,
		len(contextData.Content),
		storagePath,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create context reference: %w", err)
	}
	
	return nil
}

// GetContextReference retrieves a context reference by ID
func (d *Database) GetContextReference(ctx context.Context, contextID string) (*ContextReference, error) {
	query := `
		SELECT 
			id, agent_id, model_id, session_id, created_at, updated_at, expires_at, 
			token_count, message_count, storage_path
		FROM mcp.context_references
		WHERE id = $1
	`
	
	var ref ContextReference
	err := d.db.GetContext(ctx, &ref, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context reference: %w", err)
	}
	
	return &ref, nil
}

// UpdateContextReference updates an existing context reference
func (d *Database) UpdateContextReference(ctx context.Context, contextData *mcp.Context) error {
	query := `
		UPDATE mcp.context_references
		SET
			agent_id = $2,
			model_id = $3,
			session_id = $4,
			updated_at = $5,
			expires_at = $6,
			token_count = $7,
			message_count = $8
		WHERE id = $1
	`
	
	_, err := d.db.ExecContext(
		ctx,
		query,
		contextData.ID,
		contextData.AgentID,
		contextData.ModelID,
		contextData.SessionID,
		contextData.UpdatedAt,
		contextData.ExpiresAt,
		contextData.CurrentTokens,
		len(contextData.Content),
	)
	
	if err != nil {
		return fmt.Errorf("failed to update context reference: %w", err)
	}
	
	return nil
}

// DeleteContextReference deletes a context reference
func (d *Database) DeleteContextReference(ctx context.Context, contextID string) error {
	query := `DELETE FROM mcp.context_references WHERE id = $1`
	
	_, err := d.db.ExecContext(ctx, query, contextID)
	if err != nil {
		return fmt.Errorf("failed to delete context reference: %w", err)
	}
	
	return nil
}

// ListContextReferences lists context references
func (d *Database) ListContextReferences(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*ContextReference, error) {
	// Construct the base query
	baseQuery := `
		SELECT 
			id, agent_id, model_id, session_id, created_at, updated_at, expires_at, 
			token_count, message_count, storage_path
		FROM mcp.context_references
		WHERE 1=1
	`
	
	// Construct where clause and arguments
	var whereClause []string
	var args []interface{}
	argPos := 1
	
	if agentID != "" {
		whereClause = append(whereClause, fmt.Sprintf("agent_id = $%d", argPos))
		args = append(args, agentID)
		argPos++
	}
	
	if sessionID != "" {
		whereClause = append(whereClause, fmt.Sprintf("session_id = $%d", argPos))
		args = append(args, sessionID)
		argPos++
	}
	
	// Add options filtering
	if options != nil {
		// Example: Filter by model_id
		if modelID, ok := options["model_id"].(string); ok && modelID != "" {
			whereClause = append(whereClause, fmt.Sprintf("model_id = $%d", argPos))
			args = append(args, modelID)
			argPos++
		}
		
		// Example: Filter by created_after timestamp
		if createdAfter, ok := options["created_after"].(time.Time); ok && !createdAfter.IsZero() {
			whereClause = append(whereClause, fmt.Sprintf("created_at > $%d", argPos))
			args = append(args, createdAfter)
			argPos++
		}
	}
	
	// Add where clause to query
	query := baseQuery
	if len(whereClause) > 0 {
		query += " AND " + strings.Join(whereClause, " AND ")
	}
	
	// Add order and limit
	query += " ORDER BY created_at DESC"
	if limit, ok := options["limit"].(int); ok && limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	
	// Execute query
	var references []*ContextReference
	err := d.db.SelectContext(ctx, &references, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list context references: %w", err)
	}
	
	return references, nil
}

// CreateContextReferenceTable creates the context_references table if it doesn't exist
func (d *Database) CreateContextReferenceTable(ctx context.Context) error {
	// Create schema if it doesn't exist
	_, err := d.db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS mcp")
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	
	// Create context_references table
	query := `
		CREATE TABLE IF NOT EXISTS mcp.context_references (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			session_id TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE,
			token_count INTEGER NOT NULL DEFAULT 0,
			message_count INTEGER NOT NULL DEFAULT 0,
			storage_path TEXT NOT NULL
		)
	`
	
	// Execute the CREATE TABLE query
	_, err = d.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create context_references table: %w", err)
	}
	
	// Create indexes in separate statements (PostgreSQL style)
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_context_agent_id ON mcp.context_references (agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_context_session_id ON mcp.context_references (session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_context_created_at ON mcp.context_references (created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_context_expires_at ON mcp.context_references (expires_at)`,
	}
	
	// Execute each CREATE INDEX query
	for _, indexQuery := range indexes {
		_, err = d.db.ExecContext(ctx, indexQuery)
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}
	
	return nil
}

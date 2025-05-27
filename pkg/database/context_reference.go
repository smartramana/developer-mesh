package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/jmoiron/sqlx"
)

// ContextReference represents a reference to a context stored in S3 or another storage
type ContextReference struct {
	ID           string    `db:"id" json:"id"`
	AgentID      string    `db:"agent_id" json:"agent_id"`
	ModelID      string    `db:"model_id" json:"model_id"`
	SessionID    string    `db:"session_id" json:"session_id"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
	ExpiresAt    time.Time `db:"expires_at" json:"expires_at"`
	TokenCount   int       `db:"token_count" json:"token_count"`
	MessageCount int       `db:"message_count" json:"message_count"`
	StoragePath  string    `db:"storage_path" json:"storage_path"`
}

// CreateContextReferenceTable creates the context_references table if it doesn't exist
func (d *Database) CreateContextReferenceTable(ctx context.Context) error {
	// Create schema if not exists
	_, err := d.db.ExecContext(ctx, `
		CREATE SCHEMA IF NOT EXISTS mcp
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create context_references table
	_, err = d.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mcp.context_references (
			id VARCHAR(255) PRIMARY KEY,
			agent_id VARCHAR(255) NOT NULL,
			model_id VARCHAR(255) NOT NULL,
			session_id VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE,
			token_count INTEGER NOT NULL DEFAULT 0,
			message_count INTEGER NOT NULL DEFAULT 0,
			storage_path VARCHAR(1024) NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create context_references table: %w", err)
	}

	// Create indices
	_, err = d.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_context_agent_id ON mcp.context_references(agent_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create agent_id index: %w", err)
	}

	_, err = d.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_context_session_id ON mcp.context_references(session_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create session_id index: %w", err)
	}

	_, err = d.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_context_created_at ON mcp.context_references(created_at)
	`)
	if err != nil {
		return fmt.Errorf("failed to create created_at index: %w", err)
	}

	_, err = d.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_context_expires_at ON mcp.context_references(expires_at)
	`)
	if err != nil {
		return fmt.Errorf("failed to create expires_at index: %w", err)
	}

	return nil
}

// CreateContextReference creates a new context reference
func (d *Database) CreateContextReference(ctx context.Context, context *models.Context) error {
	// Generate storage path
	storagePath := fmt.Sprintf("contexts/%s.json", context.ID)

	// Insert context reference
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO mcp.context_references (
			id, agent_id, model_id, session_id, created_at, updated_at, expires_at, 
			token_count, message_count, storage_path
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`,
		context.ID,
		context.AgentID,
		context.ModelID,
		context.SessionID,
		context.CreatedAt,
		context.UpdatedAt,
		context.ExpiresAt,
		context.CurrentTokens,
		len(context.Content),
		storagePath,
	)

	if err != nil {
		return fmt.Errorf("failed to create context reference: %w", err)
	}

	return nil
}

// GetContextReference retrieves a context reference by ID
func (d *Database) GetContextReference(ctx context.Context, id string) (*ContextReference, error) {
	var ref ContextReference
	err := d.db.GetContext(ctx, &ref, `
		SELECT 
			id, agent_id, model_id, session_id, created_at, updated_at, expires_at, 
			token_count, message_count, storage_path
		FROM mcp.context_references
		WHERE id = $1
	`, id)

	if err != nil {
		return nil, fmt.Errorf("failed to get context reference: %w", err)
	}

	return &ref, nil
}

// UpdateContextReference updates an existing context reference
func (d *Database) UpdateContextReference(ctx context.Context, context *models.Context) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE mcp.context_references SET
			agent_id = $2,
			model_id = $3,
			session_id = $4,
			updated_at = $5,
			expires_at = $6,
			token_count = $7,
			message_count = $8
		WHERE id = $1
	`,
		context.ID,
		context.AgentID,
		context.ModelID,
		context.SessionID,
		context.UpdatedAt,
		context.ExpiresAt,
		context.CurrentTokens,
		len(context.Content),
	)

	if err != nil {
		return fmt.Errorf("failed to update context reference: %w", err)
	}

	return nil
}

// DeleteContextReference deletes a context reference by ID
func (d *Database) DeleteContextReference(ctx context.Context, id string) error {
	_, err := d.db.ExecContext(ctx, `
		DELETE FROM mcp.context_references
		WHERE id = $1
	`, id)

	if err != nil {
		return fmt.Errorf("failed to delete context reference: %w", err)
	}

	return nil
}

// ListContextReferences lists context references filtered by the given criteria
func (d *Database) ListContextReferences(ctx context.Context, agentID, sessionID string, options map[string]any) ([]*ContextReference, error) {
	// Build the query dynamically
	query := strings.Builder{}
	query.WriteString(`
		SELECT 
			id, agent_id, model_id, session_id, created_at, updated_at, expires_at, 
			token_count, message_count, storage_path
		FROM mcp.context_references
		WHERE 1=1
	`)

	// Create a slice to hold the query arguments
	var args []any
	var argIndex int = 0

	// Add agentID filter if provided
	if agentID != "" {
		argIndex++
		query.WriteString(fmt.Sprintf(" AND agent_id = $%d", argIndex))
		args = append(args, agentID)
	}

	// Add sessionID filter if provided
	if sessionID != "" {
		argIndex++
		query.WriteString(fmt.Sprintf(" AND session_id = $%d", argIndex))
		args = append(args, sessionID)
	}

	// Add options filters if provided
	if options != nil {
		// Add modelID filter
		if modelID, ok := options["model_id"]; ok {
			argIndex++
			query.WriteString(fmt.Sprintf(" AND model_id = $%d", argIndex))
			args = append(args, modelID)
		}

		// Add createdAfter filter
		if createdAfter, ok := options["created_after"]; ok {
			if ca, ok := createdAfter.(time.Time); ok {
				argIndex++
				query.WriteString(fmt.Sprintf(" AND created_at > $%d", argIndex))
				args = append(args, ca)
			}
		}

		// Add createdBefore filter
		if createdBefore, ok := options["created_before"]; ok {
			if cb, ok := createdBefore.(time.Time); ok {
				argIndex++
				query.WriteString(fmt.Sprintf(" AND created_at < $%d", argIndex))
				args = append(args, cb)
			}
		}

		// Add updatedAfter filter
		if updatedAfter, ok := options["updated_after"]; ok {
			if ua, ok := updatedAfter.(time.Time); ok {
				argIndex++
				query.WriteString(fmt.Sprintf(" AND updated_at > $%d", argIndex))
				args = append(args, ua)
			}
		}

		// Add expiresAfter filter
		if expiresAfter, ok := options["expires_after"]; ok {
			if ea, ok := expiresAfter.(time.Time); ok {
				argIndex++
				query.WriteString(fmt.Sprintf(" AND expires_at > $%d", argIndex))
				args = append(args, ea)
			}
		}

		// Add expiresBefore filter
		if expiresBefore, ok := options["expires_before"]; ok {
			if eb, ok := expiresBefore.(time.Time); ok {
				argIndex++
				query.WriteString(fmt.Sprintf(" AND expires_at < $%d", argIndex))
				args = append(args, eb)
			}
		}
	}

	// Add ordering
	query.WriteString(" ORDER BY created_at DESC")

	// Add limit if provided
	if options != nil {
		if limit, ok := options["limit"]; ok {
			if l, ok := limit.(int); ok && l > 0 {
				argIndex++
				query.WriteString(fmt.Sprintf(" LIMIT $%d", argIndex))
				args = append(args, l)
			}
		}
	}

	// Execute the query
	rows, err := d.db.QueryxContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list context references: %w", err)
	}
	defer rows.Close()

	// Process the results
	var refs []*ContextReference
	for rows.Next() {
		var ref ContextReference
		if err := rows.StructScan(&ref); err != nil {
			return nil, fmt.Errorf("failed to scan context reference: %w", err)
		}
		refs = append(refs, &ref)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over context references: %w", err)
	}

	return refs, nil
}

// Transaction is a helper function for running a transaction
func (d *Database) RunTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after Rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("error rolling back transaction: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

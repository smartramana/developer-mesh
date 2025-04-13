package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// CreateContext creates a new context in the database
func (d *Database) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	query := `
		INSERT INTO mcp.contexts 
		(id, agent_id, model_id, session_id, content, metadata, current_tokens, max_tokens, created_at, updated_at, expires_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	
	// Convert content to JSON
	contentJSON, err := json.Marshal(contextData.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}
	
	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(contextData.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	// Execute query
	_, err = d.db.ExecContext(
		ctx,
		query,
		contextData.ID,
		contextData.AgentID,
		contextData.ModelID,
		contextData.SessionID,
		contentJSON,
		metadataJSON,
		contextData.CurrentTokens,
		contextData.MaxTokens,
		contextData.CreatedAt,
		contextData.UpdatedAt,
		contextData.ExpiresAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create context: %w", err)
	}
	
	return nil
}

// GetContext retrieves a context from the database
func (d *Database) GetContext(ctx context.Context, id string) (*mcp.Context, error) {
	query := `
		SELECT 
			id, agent_id, model_id, session_id, content, metadata, 
			current_tokens, max_tokens, created_at, updated_at, expires_at
		FROM mcp.contexts
		WHERE id = $1
	`
	
	var contextData mcp.Context
	var contentJSON, metadataJSON []byte
	
	err := d.db.QueryRowxContext(ctx, query, id).Scan(
		&contextData.ID,
		&contextData.AgentID,
		&contextData.ModelID,
		&contextData.SessionID,
		&contentJSON,
		&metadataJSON,
		&contextData.CurrentTokens,
		&contextData.MaxTokens,
		&contextData.CreatedAt,
		&contextData.UpdatedAt,
		&contextData.ExpiresAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}
	
	// Parse content
	if err := json.Unmarshal(contentJSON, &contextData.Content); err != nil {
		return nil, fmt.Errorf("failed to unmarshal content: %w", err)
	}
	
	// Parse metadata
	if err := json.Unmarshal(metadataJSON, &contextData.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	
	return &contextData, nil
}

// UpdateContext updates a context in the database
func (d *Database) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	query := `
		UPDATE mcp.contexts
		SET 
			agent_id = $2,
			model_id = $3,
			session_id = $4,
			content = $5,
			metadata = $6,
			current_tokens = $7,
			max_tokens = $8,
			updated_at = $9,
			expires_at = $10
		WHERE id = $1
	`
	
	// Convert content to JSON
	contentJSON, err := json.Marshal(contextData.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}
	
	// Convert metadata to JSON
	metadataJSON, err := json.Marshal(contextData.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	// Execute query
	result, err := d.db.ExecContext(
		ctx,
		query,
		contextData.ID,
		contextData.AgentID,
		contextData.ModelID,
		contextData.SessionID,
		contentJSON,
		metadataJSON,
		contextData.CurrentTokens,
		contextData.MaxTokens,
		time.Now(),
		contextData.ExpiresAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update context: %w", err)
	}
	
	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("context not found: %s", contextData.ID)
	}
	
	return nil
}

// DeleteContext deletes a context from the database
func (d *Database) DeleteContext(ctx context.Context, id string) error {
	query := `DELETE FROM mcp.contexts WHERE id = $1`
	
	result, err := d.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}
	
	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("context not found: %s", id)
	}
	
	return nil
}

// ListContexts lists contexts from the database with optional filtering
func (d *Database) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	query := `
		SELECT
			id, agent_id, model_id, session_id, content, metadata,
			current_tokens, max_tokens, created_at, updated_at, expires_at
		FROM mcp.contexts
		WHERE 1 = 1
	`
	args := []interface{}{}
	
	// Add filters
	if agentID != "" {
		query += " AND agent_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, agentID)
	}
	
	if sessionID != "" {
		query += " AND session_id = $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, sessionID)
	}
	
	// Add ordering
	query += " ORDER BY created_at DESC"
	
	// Add limit if specified
	if limit, ok := options["limit"].(int); ok && limit > 0 {
		query += " LIMIT $" + fmt.Sprintf("%d", len(args)+1)
		args = append(args, limit)
	}
	
	// Execute query
	rows, err := d.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts: %w", err)
	}
	defer rows.Close()
	
	// Parse results
	var contexts []*mcp.Context
	for rows.Next() {
		var contextData mcp.Context
		var contentJSON, metadataJSON []byte
		
		err := rows.Scan(
			&contextData.ID,
			&contextData.AgentID,
			&contextData.ModelID,
			&contextData.SessionID,
			&contentJSON,
			&metadataJSON,
			&contextData.CurrentTokens,
			&contextData.MaxTokens,
			&contextData.CreatedAt,
			&contextData.UpdatedAt,
			&contextData.ExpiresAt,
		)
		
		if err != nil {
			return nil, fmt.Errorf("failed to scan context: %w", err)
		}
		
		// Parse content
		if err := json.Unmarshal(contentJSON, &contextData.Content); err != nil {
			return nil, fmt.Errorf("failed to unmarshal content: %w", err)
		}
		
		// Parse metadata
		if err := json.Unmarshal(metadataJSON, &contextData.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
		
		contexts = append(contexts, &contextData)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contexts: %w", err)
	}
	
	return contexts, nil
}

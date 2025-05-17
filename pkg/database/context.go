package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/jmoiron/sqlx"
)

// Tx represents a database transaction
type Tx struct {
	tx *sqlx.Tx
}

// CreateContext creates a new context in the database
func (db *Database) CreateContext(ctx context.Context, contextData *mcp.Context) error {
	return db.Transaction(ctx, func(sqlxTx *sqlx.Tx) error {
		tx := &Tx{tx: sqlxTx}
		return db.createContext(ctx, tx, contextData)
	})
}

// createContext is the internal implementation to create a context within a transaction
func (db *Database) createContext(ctx context.Context, tx *Tx, contextData *mcp.Context) error {
	// Serialize metadata to JSON, handling nil/empty cases
	var metadataJSON []byte
	var err error
	if contextData.Metadata == nil || len(contextData.Metadata) == 0 {
		metadataJSON = []byte("{}")
	} else {
		metadataJSON, err = json.Marshal(contextData.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		// Final check to avoid empty string being sent to PostgreSQL
		if string(metadataJSON) == "" || string(metadataJSON) == "null" {
			metadataJSON = []byte("{}")
		}
	}
	
	// Insert context record
	_, err = tx.tx.ExecContext(ctx, `
		INSERT INTO mcp.contexts (
			id, agent_id, model_id, session_id, current_tokens, max_tokens,
			metadata, created_at, updated_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`,
		contextData.ID,
		contextData.AgentID,
		contextData.ModelID,
		contextData.SessionID,
		contextData.CurrentTokens,
		contextData.MaxTokens,
		metadataJSON,
		contextData.CreatedAt,
		contextData.UpdatedAt,
		contextData.ExpiresAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to insert context: %w", err)
	}
	
	// Insert context items
	for _, item := range contextData.Content {
		if err := db.createContextItem(ctx, tx, contextData.ID, &item); err != nil {
			return err
		}
	}
	
	return nil
}

// createContextItem creates a context item in the database
func (db *Database) createContextItem(ctx context.Context, tx *Tx, contextID string, item *mcp.ContextItem) error {
	// Generate ID if not provided
	itemID := item.ID
	if itemID == "" {
		itemID = fmt.Sprintf("item-%s", NewUUID())
		item.ID = itemID
	}
	
	// Serialize item metadata to JSON, handling nil/empty cases
	var metadataJSON []byte
	var err error
	if item.Metadata == nil || len(item.Metadata) == 0 {
		metadataJSON = []byte("{}")
	} else {
		metadataJSON, err = json.Marshal(item.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal item metadata: %w", err)
		}
		// Final check to avoid empty string being sent to PostgreSQL
		if string(metadataJSON) == "" || string(metadataJSON) == "null" {
			metadataJSON = []byte("{}")
		}
	}
	
	// Insert context item
	_, err = tx.tx.ExecContext(ctx, `
		INSERT INTO mcp.context_items (
			id, context_id, role, content, tokens, timestamp, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
	`,
		itemID,
		contextID,
		item.Role,
		item.Content,
		item.Tokens,
		item.Timestamp,
		metadataJSON,
	)
	
	if err != nil {
		return fmt.Errorf("failed to insert context item: %w", err)
	}
	
	return nil
}

// GetContext retrieves a context from the database
func (db *Database) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	var contextData *mcp.Context
	
	err := db.Transaction(ctx, func(sqlxTx *sqlx.Tx) error {
		tx := &Tx{tx: sqlxTx}
		var err error
		contextData, err = db.getContext(ctx, tx, contextID)
		return err
	})
	
	return contextData, err
}

// getContext is the internal implementation to retrieve a context within a transaction
func (db *Database) getContext(ctx context.Context, tx *Tx, contextID string) (*mcp.Context, error) {
	// Get context metadata
	var (
		agentID       string
		modelID       string
		sessionID     sql.NullString
		currentTokens int
		maxTokens     int
		metadata      []byte
		createdAt     time.Time
		updatedAt     time.Time
		expiresAt     sql.NullTime
	)
	
	err := tx.tx.QueryRowContext(ctx, `
		SELECT agent_id, model_id, session_id, current_tokens, max_tokens,
		       metadata, created_at, updated_at, expires_at
		FROM mcp.contexts
		WHERE id = $1
	`, contextID).Scan(
		&agentID,
		&modelID,
		&sessionID,
		&currentTokens,
		&maxTokens,
		&metadata,
		&createdAt,
		&updatedAt,
		&expiresAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("context not found: %s", contextID)
		}
		return nil, fmt.Errorf("failed to get context: %w", err)
	}
	
	// Parse metadata
	var metadataMap map[string]interface{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &metadataMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	
	// Create context object
	contextData := &mcp.Context{
		ID:            contextID,
		AgentID:       agentID,
		ModelID:       modelID,
		CurrentTokens: currentTokens,
		MaxTokens:     maxTokens,
		Metadata:      metadataMap,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		Content:       []mcp.ContextItem{},
	}
	
	if sessionID.Valid {
		contextData.SessionID = sessionID.String
	}
	
	if expiresAt.Valid {
		contextData.ExpiresAt = expiresAt.Time
	}
	
	// Get context items
	rows, err := tx.tx.QueryContext(ctx, `
		SELECT id, role, content, tokens, timestamp, metadata
		FROM mcp.context_items
		WHERE context_id = $1
		ORDER BY timestamp ASC
	`, contextID)
	
	if err != nil {
		return nil, fmt.Errorf("failed to query context items: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var (
			itemID        string
			role          string
			content       string
			tokens        int
			timestamp     time.Time
			itemMetadata  []byte
		)
		
		if err := rows.Scan(&itemID, &role, &content, &tokens, &timestamp, &itemMetadata); err != nil {
			return nil, fmt.Errorf("failed to scan context item: %w", err)
		}
		
		// Parse item metadata
		var itemMetadataMap map[string]interface{}
		if len(itemMetadata) > 0 {
			if err := json.Unmarshal(itemMetadata, &itemMetadataMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal item metadata: %w", err)
			}
		}
		
		// Add item to context
		contextData.Content = append(contextData.Content, mcp.ContextItem{
			ID:        itemID,
			Role:      role,
			Content:   content,
			Tokens:    tokens,
			Timestamp: timestamp,
			Metadata:  itemMetadataMap,
		})
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over context items: %w", err)
	}
	
	return contextData, nil
}

// UpdateContext updates a context in the database
func (db *Database) UpdateContext(ctx context.Context, contextData *mcp.Context) error {
	return db.Transaction(ctx, func(sqlxTx *sqlx.Tx) error {
		tx := &Tx{tx: sqlxTx}
		return db.updateContext(ctx, tx, contextData)
	})
}

// updateContext is the internal implementation to update a context within a transaction
func (db *Database) updateContext(ctx context.Context, tx *Tx, contextData *mcp.Context) error {
	// Serialize metadata to JSON, handling nil/empty cases
	var metadataJSON []byte
	var err error
	if contextData.Metadata == nil || len(contextData.Metadata) == 0 {
		metadataJSON = []byte("{}")
	} else {
		metadataJSON, err = json.Marshal(contextData.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		// Final check to avoid empty string being sent to PostgreSQL
		if string(metadataJSON) == "" || string(metadataJSON) == "null" {
			metadataJSON = []byte("{}")
		}
	}
	
	// Update context record
	_, err = tx.tx.ExecContext(ctx, `
		UPDATE mcp.contexts
		SET agent_id = $1, model_id = $2, session_id = $3, current_tokens = $4,
		    max_tokens = $5, metadata = $6, updated_at = $7, expires_at = $8
		WHERE id = $9
	`,
		contextData.AgentID,
		contextData.ModelID,
		contextData.SessionID,
		contextData.CurrentTokens,
		contextData.MaxTokens,
		metadataJSON,
		contextData.UpdatedAt,
		contextData.ExpiresAt,
		contextData.ID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update context: %w", err)
	}
	
	// Delete existing context items
	_, err = tx.tx.ExecContext(ctx, `
		DELETE FROM mcp.context_items
		WHERE context_id = $1
	`, contextData.ID)
	
	if err != nil {
		return fmt.Errorf("failed to delete context items: %w", err)
	}
	
	// Insert updated context items
	for _, item := range contextData.Content {
		if err := db.createContextItem(ctx, tx, contextData.ID, &item); err != nil {
			return err
		}
	}
	
	return nil
}

// DeleteContext deletes a context from the database
func (db *Database) DeleteContext(ctx context.Context, contextID string) error {
	return db.Transaction(ctx, func(sqlxTx *sqlx.Tx) error {
		tx := &Tx{tx: sqlxTx}
		return db.deleteContext(ctx, tx, contextID)
	})
}

// deleteContext is the internal implementation to delete a context within a transaction
func (db *Database) deleteContext(ctx context.Context, tx *Tx, contextID string) error {
	// Delete context (will cascade to items)
	_, err := tx.tx.ExecContext(ctx, `
		DELETE FROM mcp.contexts
		WHERE id = $1
	`, contextID)
	
	if err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}
	
	return nil
}

// ListContexts lists contexts for an agent
func (db *Database) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	var contexts []*mcp.Context
	
	err := db.Transaction(ctx, func(sqlxTx *sqlx.Tx) error {
		tx := &Tx{tx: sqlxTx}
		var err error
		contexts, err = db.listContexts(ctx, tx, agentID, sessionID, options)
		return err
	})
	
	return contexts, err
}

// listContexts is the internal implementation to list contexts within a transaction
func (db *Database) listContexts(ctx context.Context, tx *Tx, agentID string, sessionID string, options map[string]interface{}) ([]*mcp.Context, error) {
	query := `
		SELECT id, agent_id, model_id, session_id, current_tokens, max_tokens,
		       metadata, created_at, updated_at, expires_at
		FROM mcp.contexts
		WHERE agent_id = $1
	`
	
	args := []interface{}{agentID}
	argIndex := 2
	
	if sessionID != "" {
		query += fmt.Sprintf(" AND session_id = $%d", argIndex)
		args = append(args, sessionID)
		argIndex++
	}
	
	// Add limit if provided
	if options != nil {
		if limit, ok := options["limit"].(int); ok && limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", argIndex)
			args = append(args, limit)
			argIndex++
		}
	}
	
	// Add order by
	query += " ORDER BY updated_at DESC"
	
	// Query contexts
	rows, err := tx.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query contexts: %w", err)
	}
	defer rows.Close()
	
	// Process results
	var contexts []*mcp.Context
	
	for rows.Next() {
		var (
			id            string
			agentID       string
			modelID       string
			sessionIDVal  sql.NullString
			currentTokens int
			maxTokens     int
			metadata      []byte
			createdAt     time.Time
			updatedAt     time.Time
			expiresAt     sql.NullTime
		)
		
		if err := rows.Scan(
			&id,
			&agentID,
			&modelID,
			&sessionIDVal,
			&currentTokens,
			&maxTokens,
			&metadata,
			&createdAt,
			&updatedAt,
			&expiresAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan context: %w", err)
		}
		
		// Parse metadata
		var metadataMap map[string]interface{}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &metadataMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}
		
		// Create context object
		contextData := &mcp.Context{
			ID:            id,
			AgentID:       agentID,
			ModelID:       modelID,
			CurrentTokens: currentTokens,
			MaxTokens:     maxTokens,
			Metadata:      metadataMap,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
			Content:       []mcp.ContextItem{}, // Empty content for listing
		}
		
		if sessionIDVal.Valid {
			contextData.SessionID = sessionIDVal.String
		}
		
		if expiresAt.Valid {
			contextData.ExpiresAt = expiresAt.Time
		}
		
		contexts = append(contexts, contextData)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over contexts: %w", err)
	}
	
	return contexts, nil
}

// SearchContexts searches for contexts based on a text query
func (db *Database) SearchContexts(ctx context.Context, agentID string, query string, limit int) ([]*mcp.Context, error) {
	var contexts []*mcp.Context
	
	err := db.Transaction(ctx, func(sqlxTx *sqlx.Tx) error {
		tx := &Tx{tx: sqlxTx}
		var err error
		contexts, err = db.searchContexts(ctx, tx, agentID, query, limit)
		return err
	})
	
	return contexts, err
}

// searchContexts is the internal implementation to search contexts within a transaction
func (db *Database) searchContexts(ctx context.Context, tx *Tx, agentID string, query string, limit int) ([]*mcp.Context, error) {
	// Simple text search implementation
	// In a production environment, consider using PostgreSQL's full-text search capabilities
	searchQuery := `
		SELECT DISTINCT c.id
		FROM mcp.contexts c
		JOIN mcp.context_items ci ON c.id = ci.context_id
		WHERE c.agent_id = $1
		AND (
			ci.content ILIKE $2
			OR c.metadata::text ILIKE $2
			OR ci.metadata::text ILIKE $2
		)
	`
	
	if limit > 0 {
		searchQuery += " LIMIT $3"
	}
	
	args := []interface{}{agentID, "%" + query + "%"}
	if limit > 0 {
		args = append(args, limit)
	}
	
	// Query matching context IDs
	rows, err := tx.tx.QueryContext(ctx, searchQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search contexts: %w", err)
	}
	defer rows.Close()
	
	// Collect matching context IDs
	var contextIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan context ID: %w", err)
		}
		contextIDs = append(contextIDs, id)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over context IDs: %w", err)
	}
	
	// Get context details for matching IDs
	var contexts []*mcp.Context
	for _, id := range contextIDs {
		contextData, err := db.getContext(ctx, tx, id)
		if err != nil {
			// Log error but continue
			fmt.Printf("Error fetching context %s: %v\n", id, err)
			continue
		}
		contexts = append(contexts, contextData)
	}
	
	return contexts, nil
}

// Create necessary database migrations for context tables
func (db *Database) ensureContextTables(ctx context.Context) error {
	// Check if mcp schema exists
	var exists bool
	err := db.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.schemata WHERE schema_name = 'mcp'
		)
	`).Scan(&exists)
	
	if err != nil {
		return fmt.Errorf("failed to check if schema exists: %w", err)
	}
	
	// Create schema if it doesn't exist
	if !exists {
		_, err := db.db.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS mcp`)
		if err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
	}
	
	// Create contexts table if it doesn't exist
	_, err = db.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mcp.contexts (
			id VARCHAR(36) PRIMARY KEY,
			agent_id VARCHAR(255) NOT NULL,
			model_id VARCHAR(255) NOT NULL,
			session_id VARCHAR(255),
			current_tokens INTEGER NOT NULL DEFAULT 0,
			max_tokens INTEGER NOT NULL DEFAULT 4000,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE
		)
	`)
	
	if err != nil {
		return fmt.Errorf("failed to create contexts table: %w", err)
	}
	
	// Create context_items table if it doesn't exist
	_, err = db.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mcp.context_items (
			id VARCHAR(36) PRIMARY KEY,
			context_id VARCHAR(36) NOT NULL,
			role VARCHAR(50) NOT NULL,
			content TEXT NOT NULL,
			tokens INTEGER NOT NULL,
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
			metadata JSONB,
			FOREIGN KEY (context_id) REFERENCES mcp.contexts(id) ON DELETE CASCADE
		)
	`)
	
	if err != nil {
		return fmt.Errorf("failed to create context_items table: %w", err)
	}
	
	// Create indexes if they don't exist
	_, err = db.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_contexts_agent_id ON mcp.contexts(agent_id);
		CREATE INDEX IF NOT EXISTS idx_contexts_session_id ON mcp.contexts(session_id);
		CREATE INDEX IF NOT EXISTS idx_context_items_context_id ON mcp.context_items(context_id);
	`)
	
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	
	return nil
}

// NewUUID generates a new UUID
func NewUUID() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		timeBasedRandomBytes(4),
		timeBasedRandomBytes(2),
		timeBasedRandomBytes(2),
		timeBasedRandomBytes(2),
		timeBasedRandomBytes(6),
	)
}

// timeBasedRandomBytes generates random bytes with time-based seed
func timeBasedRandomBytes(n int) []byte {
	// Simple implementation for illustration
	// In a production environment, use a proper UUID library
	timestamp := time.Now().UnixNano()
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = byte((timestamp >> (8 * i)) & 0xff)
	}
	return result
}

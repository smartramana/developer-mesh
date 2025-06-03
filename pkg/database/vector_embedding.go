package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Embedding represents a vector embedding in the database
type Embedding struct {
	ID          string         `json:"id" db:"id"`
	Vector      []float32      `json:"vector" db:"vector"`
	Dimensions  int            `json:"dimensions" db:"dimensions"`
	ModelID     string         `json:"model_id" db:"model_id"`
	ContentType string         `json:"content_type" db:"content_type"`
	ContentID   string         `json:"content_id" db:"content_id"`
	Namespace   string         `json:"namespace" db:"namespace"`
	ContextID   string         `json:"context_id" db:"context_id"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
	Metadata    map[string]any `json:"metadata" db:"metadata"`
	Similarity  float64        `json:"similarity" db:"similarity"`
}

// EnsureSchema ensures the vector database schema exists
func (vdb *VectorDatabase) EnsureSchema(ctx context.Context) error {
	// This is just a wrapper around Initialize for compatibility
	return vdb.Initialize(ctx)
}

// StoreEmbedding stores an embedding vector in the database
func (vdb *VectorDatabase) StoreEmbedding(ctx context.Context, tx *sqlx.Tx, embedding *Embedding) error {
	if err := vdb.Initialize(ctx); err != nil {
		return err
	}

	if embedding == nil {
		return fmt.Errorf("embedding cannot be nil")
	}

	// Create a managed transaction if one wasn't provided
	managedTx := tx == nil
	if managedTx {
		var err error
		tx, err = vdb.vectorDB.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() {
			if err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					vdb.logger.Warn("Failed to rollback transaction", map[string]any{"error": rbErr, "original_error": err})
				}
			} else if managedTx {
				if cmErr := tx.Commit(); cmErr != nil {
					vdb.logger.Error("Failed to commit transaction", map[string]any{"error": cmErr})
				}
			}
		}()
	}

	// Ensure we have a valid ID
	if embedding.ID == "" {
		// Generate a UUID for the embedding
		var id string
		err := tx.QueryRowContext(ctx, "SELECT gen_random_uuid()::text").Scan(&id)
		if err != nil {
			return fmt.Errorf("failed to generate UUID: %w", err)
		}
		embedding.ID = id
	}

	// Set timestamps if not provided
	now := time.Now().UTC()
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = now
	}
	if embedding.UpdatedAt.IsZero() {
		embedding.UpdatedAt = now
	}

	// Convert the vector to a PostgreSQL vector format
	vectorStr, err := vdb.CreateVector(ctx, embedding.Vector)
	if err != nil {
		return fmt.Errorf("failed to create vector: %w", err)
	}

	// Store the embedding in the database
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mcp.embeddings (
			id, vector, dimensions, model_id, content_type, 
			content_id, namespace, context_id, created_at, updated_at, metadata
		) VALUES (
			$1, $2::vector, $3, $4, $5, 
			$6, $7, $8, $9, $10, $11
		)
		ON CONFLICT (id) DO UPDATE SET
			vector = $2::vector,
			dimensions = $3,
			model_id = $4,
			content_type = $5,
			content_id = $6,
			namespace = $7,
			context_id = $8,
			updated_at = $10,
			metadata = $11
	`,
		embedding.ID,
		vectorStr,
		embedding.Dimensions,
		embedding.ModelID,
		embedding.ContentType,
		embedding.ContentID,
		embedding.Namespace,
		embedding.ContextID,
		embedding.CreatedAt,
		embedding.UpdatedAt,
		embedding.Metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	return nil
}

// SearchEmbeddings searches for similar embeddings in the vector database
func (vdb *VectorDatabase) SearchEmbeddings(
	ctx context.Context,
	tx *sqlx.Tx,
	queryVector []float32,
	contextID string,
	modelID string,
	limit int,
	similarityThreshold float64,
) ([]*Embedding, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return nil, err
	}

	// Create a managed transaction if one wasn't provided
	managedTx := tx == nil
	if managedTx {
		var err error
		tx, err = vdb.vectorDB.BeginTxx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() {
			if managedTx {
				if rbErr := tx.Rollback(); rbErr != nil {
					vdb.logger.Warn("Failed to rollback transaction", map[string]any{"error": rbErr})
				}
			}
		}()
	}

	// Create the vector string
	vectorStr, err := vdb.CreateVector(ctx, queryVector)
	if err != nil {
		return nil, fmt.Errorf("failed to create query vector: %w", err)
	}

	// Build the query
	query := `
		SELECT 
			e.id, 
			e.vector, 
			e.dimensions, 
			e.model_id, 
			e.content_type, 
			e.content_id, 
			e.namespace, 
			e.context_id, 
			e.created_at, 
			e.updated_at, 
			e.metadata,
			1 - (e.vector <=> $1::vector) as similarity
		FROM 
			mcp.embeddings e
		WHERE 
			1 - (e.vector <=> $1::vector) >= $4
	`

	params := []any{vectorStr, similarityThreshold}
	paramCount := 2

	// Add optional filters
	if contextID != "" {
		paramCount++
		query += fmt.Sprintf(" AND e.context_id = $%d", paramCount)
		params = append(params, contextID)
	}

	if modelID != "" {
		paramCount++
		query += fmt.Sprintf(" AND e.model_id = $%d", paramCount)
		params = append(params, modelID)
	}

	// Add ordering and limit
	query += " ORDER BY similarity DESC"

	if limit > 0 {
		paramCount++
		query += fmt.Sprintf(" LIMIT $%d", paramCount)
		params = append(params, limit)
	}

	// Execute the query
	rows, err := tx.QueryxContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			vdb.logger.Warn("Failed to close rows", map[string]any{"error": err})
		}
	}()

	// Process results
	var results []*Embedding
	for rows.Next() {
		var emb Embedding
		if err := rows.StructScan(&emb); err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %w", err)
		}
		results = append(results, &emb)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over embeddings: %w", err)
	}

	// If we created a transaction, commit it
	if managedTx {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return results, nil
}

// GetEmbeddingByID retrieves an embedding by ID
func (vdb *VectorDatabase) GetEmbeddingByID(ctx context.Context, tx *sqlx.Tx, id string) (*Embedding, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return nil, err
	}

	// Create a managed transaction if one wasn't provided
	managedTx := tx == nil
	if managedTx {
		var err error
		tx, err = vdb.vectorDB.BeginTxx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() {
			if managedTx {
				if rbErr := tx.Rollback(); rbErr != nil {
					vdb.logger.Warn("Failed to rollback transaction", map[string]any{"error": rbErr})
				}
			}
		}()
	}

	var emb Embedding
	err := tx.QueryRowxContext(ctx, `
		SELECT 
			id, 
			vector, 
			dimensions, 
			model_id, 
			content_type, 
			content_id, 
			namespace, 
			context_id, 
			created_at, 
			updated_at, 
			metadata
		FROM 
			mcp.embeddings
		WHERE 
			id = $1
	`, id).StructScan(&emb)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("embedding not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	// If we created a transaction, commit it
	if managedTx {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return &emb, nil
}

// DeleteEmbedding deletes an embedding from the database
func (vdb *VectorDatabase) DeleteEmbedding(ctx context.Context, tx *sqlx.Tx, id string) error {
	if err := vdb.Initialize(ctx); err != nil {
		return err
	}

	// Create a managed transaction if one wasn't provided
	managedTx := tx == nil
	if managedTx {
		var err error
		tx, err = vdb.vectorDB.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() {
			if err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					vdb.logger.Warn("Failed to rollback transaction", map[string]any{"error": rbErr, "original_error": err})
				}
			} else if managedTx {
				if cmErr := tx.Commit(); cmErr != nil {
					vdb.logger.Error("Failed to commit transaction", map[string]any{"error": cmErr})
				}
			}
		}()
	}

	_, err := tx.ExecContext(ctx, `
		DELETE FROM mcp.embeddings
		WHERE id = $1
	`, id)

	if err != nil {
		return fmt.Errorf("failed to delete embedding: %w", err)
	}

	return nil
}

// BatchDeleteEmbeddings deletes multiple embeddings matching criteria
func (vdb *VectorDatabase) BatchDeleteEmbeddings(
	ctx context.Context,
	tx *sqlx.Tx,
	contentType string,
	contentID string,
	contextID string,
) (int, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return 0, err
	}

	// Create a managed transaction if one wasn't provided
	managedTx := tx == nil
	if managedTx {
		var err error
		tx, err = vdb.vectorDB.BeginTxx(ctx, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() {
			if err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					vdb.logger.Warn("Failed to rollback transaction", map[string]any{"error": rbErr, "original_error": err})
				}
			} else if managedTx {
				if cmErr := tx.Commit(); cmErr != nil {
					vdb.logger.Error("Failed to commit transaction", map[string]any{"error": cmErr})
				}
			}
		}()
	}

	// Build the delete query
	query := `DELETE FROM mcp.embeddings WHERE 1=1`
	var params []any
	paramCount := 0

	// Add filters
	if contentType != "" {
		paramCount++
		query += fmt.Sprintf(" AND content_type = $%d", paramCount)
		params = append(params, contentType)
	}

	if contentID != "" {
		paramCount++
		query += fmt.Sprintf(" AND content_id = $%d", paramCount)
		params = append(params, contentID)
	}

	if contextID != "" {
		paramCount++
		query += fmt.Sprintf(" AND context_id = $%d", paramCount)
		params = append(params, contextID)
	}

	// Execute the delete
	result, err := tx.ExecContext(ctx, query, params...)
	if err != nil {
		return 0, fmt.Errorf("failed to batch delete embeddings: %w", err)
	}

	// Get the number of rows affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// EmbeddingRepository provides an interface for storing and retrieving embeddings
type EmbeddingRepository struct {
	db *sqlx.DB
}

// Embedding represents a vector embedding for a piece of text
type Embedding struct {
	ID              string    `db:"id" json:"id"`
	ContextID       string    `db:"context_id" json:"context_id"`
	ContentIndex    int       `db:"content_index" json:"content_index"`
	Text            string    `db:"text" json:"text"`
	Embedding       []float32 `db:"-" json:"embedding"`  // Handled specially due to pgvector type
	EmbeddingString string    `db:"embedding" json:"-"`  // Used for database operations
	VectorDimensions int       `db:"vector_dimensions" json:"vector_dimensions"`
	ModelID         string    `db:"model_id" json:"model_id"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

// NewEmbeddingRepository creates a new repository for vector embeddings
func NewEmbeddingRepository(db *sqlx.DB) *EmbeddingRepository {
	return &EmbeddingRepository{
		db: db,
	}
}

// StoreEmbedding stores a vector embedding for a piece of text
func (r *EmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	// Convert the embedding to a pgvector string format
	// Format: [0.1,0.2,0.3,...,0.n]
	var vectorStr strings.Builder
	vectorStr.WriteString("[")
	for i, v := range embedding.Embedding {
		if i > 0 {
			vectorStr.WriteString(",")
		}
		vectorStr.WriteString(fmt.Sprintf("%f", v))
	}
	vectorStr.WriteString("]")

	// Set the vector dimensions
	dimensions := len(embedding.Embedding)

	// Insert the embedding into the database
	query := `
        INSERT INTO mcp.embeddings
        (context_id, content_index, text, embedding, vector_dimensions, model_id)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
    `

	// Execute the query
	var id string
	err := r.db.QueryRowContext(
		ctx,
		query,
		embedding.ContextID,
		embedding.ContentIndex,
		embedding.Text,
		vectorStr.String(),
		dimensions,
		embedding.ModelID,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	// Set the ID on the embedding
	embedding.ID = id

	return nil
}

// SearchEmbeddings searches for embeddings similar to the query vector
func (r *EmbeddingRepository) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error) {
	// Convert query vector to pgvector string format
	var vectorStr strings.Builder
	vectorStr.WriteString("[")
	for i, v := range queryVector {
		if i > 0 {
			vectorStr.WriteString(",")
		}
		vectorStr.WriteString(fmt.Sprintf("%f", v))
	}
	vectorStr.WriteString("]")

	// Get the vector dimensions
	dimensions := len(queryVector)

	// Create the query
	// Note: <-> is the cosine distance operator in pgvector
	query := `
        SELECT id, context_id, content_index, text, embedding::text as embedding, 
               vector_dimensions, model_id, created_at
        FROM mcp.embeddings
        WHERE context_id = $1 AND vector_dimensions = $2
        ORDER BY embedding <-> $3
        LIMIT $4
    `

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, contextID, dimensions, vectorStr.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()

	// Process the results
	embeddings := make([]*Embedding, 0)
	for rows.Next() {
		embedding := &Embedding{}
		if err := rows.Scan(
			&embedding.ID,
			&embedding.ContextID,
			&embedding.ContentIndex,
			&embedding.Text,
			&embedding.EmbeddingString,
			&embedding.VectorDimensions,
			&embedding.ModelID,
			&embedding.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding row: %w", err)
		}

		// Convert embedding string back to float32 array
		// Format from database is typically {0.1,0.2,0.3}
		// Remove the curly braces and parse
		embStr := embedding.EmbeddingString
		embStr = strings.TrimPrefix(embStr, "{")
		embStr = strings.TrimSuffix(embStr, "}")

		// Split by comma and convert to float32
		if embStr != "" {
			parts := strings.Split(embStr, ",")
			embedding.Embedding = make([]float32, len(parts))
			for i, p := range parts {
				var f float32
				if _, err := fmt.Sscanf(p, "%f", &f); err != nil {
					return nil, fmt.Errorf("failed to parse embedding value '%s': %w", p, err)
				}
				embedding.Embedding[i] = f
			}
		}

		embeddings = append(embeddings, embedding)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over embedding rows: %w", err)
	}

	return embeddings, nil
}

// GetContextEmbeddings gets all embeddings for a context
func (r *EmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	// Create the query
	query := `
        SELECT id, context_id, content_index, text, embedding::text as embedding, 
               vector_dimensions, model_id, created_at
        FROM mcp.embeddings
        WHERE context_id = $1
    `

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context embeddings: %w", err)
	}
	defer rows.Close()

	// Process the results
	embeddings := make([]*Embedding, 0)
	for rows.Next() {
		embedding := &Embedding{}
		if err := rows.Scan(
			&embedding.ID,
			&embedding.ContextID,
			&embedding.ContentIndex,
			&embedding.Text,
			&embedding.EmbeddingString,
			&embedding.VectorDimensions,
			&embedding.ModelID,
			&embedding.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding row: %w", err)
		}

		// Convert embedding string back to float32 array
		// Format from database is typically {0.1,0.2,0.3}
		// Remove the curly braces and parse
		embStr := embedding.EmbeddingString
		embStr = strings.TrimPrefix(embStr, "{")
		embStr = strings.TrimSuffix(embStr, "}")

		// Split by comma and convert to float32
		if embStr != "" {
			parts := strings.Split(embStr, ",")
			embedding.Embedding = make([]float32, len(parts))
			for i, p := range parts {
				var f float32
				if _, err := fmt.Sscanf(p, "%f", &f); err != nil {
					return nil, fmt.Errorf("failed to parse embedding value '%s': %w", p, err)
				}
				embedding.Embedding[i] = f
			}
		}

		embeddings = append(embeddings, embedding)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over embedding rows: %w", err)
	}

	return embeddings, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (r *EmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	// Create the query
	query := "DELETE FROM mcp.embeddings WHERE context_id = $1"

	// Execute the query
	_, err := r.db.ExecContext(ctx, query, contextID)
	if err != nil {
		return fmt.Errorf("failed to delete context embeddings: %w", err)
	}

	return nil
}

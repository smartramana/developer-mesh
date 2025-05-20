package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/common"
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
	Similarity      float64   `db:"similarity" json:"similarity,omitempty"` // Optional similarity score
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
	vectorStr := common.FormatVectorForPgVector(embedding.Embedding)

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
		vectorStr,
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
func (r *EmbeddingRepository) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*Embedding, error) {
	// Convert query vector to pgvector string format
	vectorStr := common.FormatVectorForPgVector(queryVector)

	// Get the vector dimensions
	dimensions := len(queryVector)

	// Create the query
	// Note: <-> is the cosine distance operator in pgvector
	// We add similarity calculation and filtering by model ID
	query := `
        SELECT id, context_id, content_index, text, embedding::text as embedding,
               vector_dimensions, model_id, created_at, 
               (1 - (embedding <-> $3)) as similarity
        FROM mcp.embeddings
        WHERE context_id = $1 AND vector_dimensions = $2 AND model_id = $4
          AND (1 - (embedding <-> $3)) >= $5
        ORDER BY embedding <-> $3
        LIMIT $6
    `

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, contextID, dimensions, vectorStr, modelID, similarityThreshold, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()

	// Process the results
	embeddings := make([]*Embedding, 0)
	for rows.Next() {
		embedding := &Embedding{}
		var similarity float64
		
		if err := rows.Scan(
			&embedding.ID,
			&embedding.ContextID,
			&embedding.ContentIndex,
			&embedding.Text,
			&embedding.EmbeddingString,
			&embedding.VectorDimensions,
			&embedding.ModelID,
			&embedding.CreatedAt,
			&similarity,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding row: %w", err)
		}

		// Set the similarity value
		embedding.Similarity = similarity

		// Convert embedding string back to float32 array
		embeddingArray, err := common.ParseVectorFromPgVector(embedding.EmbeddingString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedding vector: %w", err)
		}
		embedding.Embedding = embeddingArray
		
		embeddings = append(embeddings, embedding)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over embedding rows: %w", err)
	}

	return embeddings, nil
}

// Simplified version of SearchEmbeddings that doesn't require model ID for backward compatibility
func (r *EmbeddingRepository) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error) {
	// Convert query vector to pgvector string format
	vectorStr := common.FormatVectorForPgVector(queryVector)

	// Get the vector dimensions
	dimensions := len(queryVector)

	// Create the query
	query := `
        SELECT id, context_id, content_index, text, embedding::text as embedding, 
               vector_dimensions, model_id, created_at
        FROM mcp.embeddings
        WHERE context_id = $1 AND vector_dimensions = $2
        ORDER BY embedding <-> $3
        LIMIT $4
    `

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, contextID, dimensions, vectorStr, limit)
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
		embeddingArray, err := common.ParseVectorFromPgVector(embedding.EmbeddingString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedding vector: %w", err)
		}
		embedding.Embedding = embeddingArray

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
		embeddingArray, err := common.ParseVectorFromPgVector(embedding.EmbeddingString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedding vector: %w", err)
		}
		embedding.Embedding = embeddingArray

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

// GetEmbeddingsByModel retrieves all embeddings for a specific model in a context
func (r *EmbeddingRepository) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error) {
	// Create the query
	query := `
        SELECT id, context_id, content_index, text, embedding::text as embedding, 
               vector_dimensions, model_id, created_at
        FROM mcp.embeddings
        WHERE context_id = $1 AND model_id = $2
        ORDER BY content_index
    `

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, contextID, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get embeddings by model: %w", err)
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
		embeddingArray, err := common.ParseVectorFromPgVector(embedding.EmbeddingString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedding vector: %w", err)
		}
		embedding.Embedding = embeddingArray

		embeddings = append(embeddings, embedding)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over embedding rows: %w", err)
	}

	return embeddings, nil
}

// GetSupportedModels gets a list of all model IDs that have embeddings
func (r *EmbeddingRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	// Create the query to get distinct model IDs
	query := `
        SELECT DISTINCT model_id
        FROM mcp.embeddings
        ORDER BY model_id
    `

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get supported models: %w", err)
	}
	defer rows.Close()

	// Process the results
	models := make([]string, 0)
	for rows.Next() {
		var modelID string
		if err := rows.Scan(&modelID); err != nil {
			return nil, fmt.Errorf("failed to scan model ID: %w", err)
		}
		models = append(models, modelID)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over model rows: %w", err)
	}

	return models, nil
}

// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
func (r *EmbeddingRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	// Create the query
	query := "DELETE FROM mcp.embeddings WHERE context_id = $1 AND model_id = $2"

	// Execute the query
	_, err := r.db.ExecContext(ctx, query, contextID, modelID)
	if err != nil {
		return fmt.Errorf("failed to delete model embeddings: %w", err)
	}

	return nil
}

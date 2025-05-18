package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// EmbeddingRepositoryImpl implements VectorAPIRepository
type EmbeddingRepositoryImpl struct {
	db *sqlx.DB
}

// NewEmbeddingRepository creates a new EmbeddingRepository instance
func NewEmbeddingRepository(db *sqlx.DB) VectorAPIRepository {
	return &EmbeddingRepositoryImpl{db: db}
}

// StoreEmbedding implements VectorAPIRepository.StoreEmbedding
func (r *EmbeddingRepositoryImpl) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	if embedding == nil {
		return errors.New("embedding cannot be nil")
	}

	// Ensure we have a timestamp
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = time.Now()
	}

	query := `INSERT INTO embeddings (id, context_id, content_index, text, embedding, model_id, created_at, metadata)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
              ON CONFLICT (id) DO UPDATE SET
              context_id = $2, content_index = $3, text = $4, embedding = $5, model_id = $6, metadata = $8`

	_, err := r.db.ExecContext(ctx, query,
		embedding.ID,
		embedding.ContextID,
		embedding.ContentIndex,
		embedding.Text,
		embedding.Embedding,
		embedding.ModelID,
		embedding.CreatedAt,
		embedding.Metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}

	return nil
}

// SearchEmbeddings implements VectorAPIRepository.SearchEmbeddings
func (r *EmbeddingRepositoryImpl) SearchEmbeddings(
	ctx context.Context,
	queryVector []float32,
	contextID string,
	modelID string,
	limit int,
	similarityThreshold float64,
) ([]*Embedding, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector cannot be empty")
	}

	if limit <= 0 {
		limit = 10 // Default limit
	}

	// Build query based on parameters
	query := `SELECT id, context_id, content_index, text, embedding, model_id, created_at, metadata FROM embeddings`
	
	whereClause := ""
	var args []interface{}
	argIndex := 1

	// Add context filter if provided
	if contextID != "" {
		whereClause = " WHERE context_id = $1"
		args = append(args, contextID)
		argIndex++
	}

	// Add model filter if provided
	if modelID != "" {
		if whereClause == "" {
			whereClause = " WHERE model_id = $" + fmt.Sprintf("%d", argIndex)
		} else {
			whereClause += " AND model_id = $" + fmt.Sprintf("%d", argIndex)
		}
		args = append(args, modelID)
		argIndex++
	}

	// Order by similarity to query vector (simplified version)
	// In a real implementation, this would use vector similarity functions like cosine similarity
	// but for compatibility we'll use a simplified approach
	query += whereClause + " ORDER BY id LIMIT $" + fmt.Sprintf("%d", argIndex)
	args = append(args, limit)

	var embeddings []*Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}

	// In a real implementation, we'd calculate similarity here and filter by threshold
	// For now, we'll just return all results
	return embeddings, nil
}

// SearchEmbeddings_Legacy implements VectorAPIRepository.SearchEmbeddings_Legacy
func (r *EmbeddingRepositoryImpl) SearchEmbeddings_Legacy(
	ctx context.Context,
	queryVector []float32,
	contextID string,
	limit int,
) ([]*Embedding, error) {
	// Legacy method delegates to the new method with default values
	return r.SearchEmbeddings(ctx, queryVector, contextID, "", limit, 0.0)
}

// GetContextEmbeddings implements VectorAPIRepository.GetContextEmbeddings
func (r *EmbeddingRepositoryImpl) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	if contextID == "" {
		return nil, errors.New("context ID cannot be empty")
	}

	query := `SELECT id, context_id, content_index, text, embedding, model_id, created_at, metadata
              FROM embeddings WHERE context_id = $1 ORDER BY content_index`

	var embeddings []*Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context embeddings: %w", err)
	}

	return embeddings, nil
}

// DeleteContextEmbeddings implements VectorAPIRepository.DeleteContextEmbeddings
func (r *EmbeddingRepositoryImpl) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	if contextID == "" {
		return errors.New("context ID cannot be empty")
	}

	query := `DELETE FROM embeddings WHERE context_id = $1`

	_, err := r.db.ExecContext(ctx, query, contextID)
	if err != nil {
		return fmt.Errorf("failed to delete context embeddings: %w", err)
	}

	return nil
}

// GetEmbeddingsByModel implements VectorAPIRepository.GetEmbeddingsByModel
func (r *EmbeddingRepositoryImpl) GetEmbeddingsByModel(
	ctx context.Context,
	contextID string,
	modelID string,
) ([]*Embedding, error) {
	if contextID == "" {
		return nil, errors.New("context ID cannot be empty")
	}

	if modelID == "" {
		return nil, errors.New("model ID cannot be empty")
	}

	query := `SELECT id, context_id, content_index, text, embedding, model_id, created_at, metadata
              FROM embeddings WHERE context_id = $1 AND model_id = $2 ORDER BY content_index`

	var embeddings []*Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, contextID, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get embeddings by model: %w", err)
	}

	return embeddings, nil
}

// GetSupportedModels implements VectorAPIRepository.GetSupportedModels
func (r *EmbeddingRepositoryImpl) GetSupportedModels(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT model_id FROM embeddings WHERE model_id IS NOT NULL AND model_id != ''`

	var modelIDs []string
	err := r.db.SelectContext(ctx, &modelIDs, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get supported models: %w", err)
	}

	return modelIDs, nil
}

// DeleteModelEmbeddings implements VectorAPIRepository.DeleteModelEmbeddings
func (r *EmbeddingRepositoryImpl) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	if contextID == "" {
		return errors.New("context ID cannot be empty")
	}

	if modelID == "" {
		return errors.New("model ID cannot be empty")
	}

	query := `DELETE FROM embeddings WHERE context_id = $1 AND model_id = $2`

	_, err := r.db.ExecContext(ctx, query, contextID, modelID)
	if err != nil {
		return fmt.Errorf("failed to delete model embeddings: %w", err)
	}

	return nil
}

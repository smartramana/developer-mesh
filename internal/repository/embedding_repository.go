package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Embedding represents a vector embedding stored in the database
type Embedding struct {
	ID               string     `db:"id" json:"id"`
	ContextID        string     `db:"context_id" json:"context_id"`
	ContentIndex     int        `db:"content_index" json:"content_index"`
	Text             string     `db:"text" json:"text"`
	Embedding        []float32  `db:"-" json:"embedding"`
	VectorDimensions int        `db:"vector_dimensions" json:"vector_dimensions"`
	ModelID          string     `db:"model_id" json:"model_id"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	Similarity       float64    `db:"similarity,omitempty" json:"similarity,omitempty"`
}

// EmbeddingRepository handles operations for vector embeddings
type EmbeddingRepository struct {
	db     *sqlx.DB
	logger *observability.Logger
}

// NewEmbeddingRepository creates a new embedding repository
func NewEmbeddingRepository(db *sqlx.DB) *EmbeddingRepository {
	return &EmbeddingRepository{
		db:     db,
		logger: observability.NewLogger("embedding_repository"),
	}
}

// StoreEmbedding stores a vector embedding in the database
func (r *EmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	// Generate ID if not provided
	if embedding.ID == "" {
		embedding.ID = uuid.New().String()
	}
	
	// Set timestamp if not provided
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = time.Now()
	}
	
	// Calculate vector dimensions if not provided
	if embedding.VectorDimensions == 0 {
		embedding.VectorDimensions = len(embedding.Embedding)
	}
	
	// Convert embedding array to PostgreSQL vector format
	vectorStr := "["
	for i, val := range embedding.Embedding {
		if i > 0 {
			vectorStr += ","
		}
		vectorStr += fmt.Sprintf("%f", val)
	}
	vectorStr += "]"
	
	// Insert embedding
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO mcp.embeddings (
			id, context_id, content_index, text, 
			embedding, vector_dimensions, model_id, created_at
		) VALUES (
			$1, $2, $3, $4, $5::vector, $6, $7, $8
		) RETURNING id
	`,
		embedding.ID,
		embedding.ContextID,
		embedding.ContentIndex,
		embedding.Text,
		vectorStr,
		embedding.VectorDimensions,
		embedding.ModelID,
		embedding.CreatedAt,
	).Scan(&id)
	
	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}
	
	// Update ID in case it was generated on the database side
	embedding.ID = id
	
	return nil
}

// SearchEmbeddings searches for similar embeddings within a context
// Uses model_id to ensure only embeddings from the same model are compared
func (r *EmbeddingRepository) SearchEmbeddings(
	ctx context.Context, 
	queryEmbedding []float32, 
	contextID string, 
	modelID string,
	limit int,
	threshold float64,
) ([]*Embedding, error) {
	// Validate inputs
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding cannot be empty")
	}
	
	if contextID == "" {
		return nil, fmt.Errorf("context ID is required")
	}
	
	if modelID == "" {
		return nil, fmt.Errorf("model ID is required")
	}
	
	// Default limit if not provided
	if limit <= 0 {
		limit = 10
	}
	
	// Default threshold if not provided
	if threshold <= 0 {
		threshold = 0.7
	}
	
	// Calculate dimensions
	dimensions := len(queryEmbedding)
	
	// Convert query embedding to PostgreSQL vector format
	vectorStr := "["
	for i, val := range queryEmbedding {
		if i > 0 {
			vectorStr += ","
		}
		vectorStr += fmt.Sprintf("%f", val)
	}
	vectorStr += "]"
	
	// Query similar embeddings
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, context_id, content_index, text, embedding::text as embedding, 
		       vector_dimensions, model_id, created_at,
		       (1 - (embedding <=> $3::vector))::float as similarity
		FROM mcp.embeddings 
		WHERE context_id = $1 
		AND vector_dimensions = $2 
		AND model_id = $4
		AND (1 - (embedding <=> $3::vector))::float >= $5
		ORDER BY embedding <=> $3::vector 
		LIMIT $6
	`,
		contextID,
		dimensions,
		vectorStr,
		modelID,
		threshold,
		limit,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()
	
	// Process results
	var results []*Embedding
	
	for rows.Next() {
		var (
			id              string
			contextID       string
			contentIndex    int
			text            string
			embeddingStr    string
			dimensions      int
			modelID         string
			createdAt       time.Time
			similarity      float64
		)
		
		if err := rows.Scan(
			&id, &contextID, &contentIndex, &text, &embeddingStr, 
			&dimensions, &modelID, &createdAt, &similarity,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding result: %w", err)
		}
		
		// Parse embedding vector from string
		embedding, err := parseEmbeddingVector(embeddingStr)
		if err != nil {
			r.logger.Error("Failed to parse embedding vector", map[string]interface{}{
				"error": err.Error(),
				"id":    id,
			})
			continue
		}
		
		// Add to results
		results = append(results, &Embedding{
			ID:               id,
			ContextID:        contextID,
			ContentIndex:     contentIndex,
			Text:             text,
			Embedding:        embedding,
			VectorDimensions: dimensions,
			ModelID:          modelID,
			CreatedAt:        createdAt,
			Similarity:       similarity,
		})
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over embedding results: %w", err)
	}
	
	return results, nil
}

// GetContextEmbeddings gets all embeddings for a context
func (r *EmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	// Query embeddings for the context
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, context_id, content_index, text, embedding::text as embedding, 
		       vector_dimensions, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1
		ORDER BY content_index
	`, contextID)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get context embeddings: %w", err)
	}
	defer rows.Close()
	
	// Process results
	var results []*Embedding
	
	for rows.Next() {
		var (
			id              string
			contextID       string
			contentIndex    int
			text            string
			embeddingStr    string
			dimensions      int
			modelID         string
			createdAt       time.Time
		)
		
		if err := rows.Scan(
			&id, &contextID, &contentIndex, &text, &embeddingStr, 
			&dimensions, &modelID, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %w", err)
		}
		
		// Parse embedding vector from string
		embedding, err := parseEmbeddingVector(embeddingStr)
		if err != nil {
			r.logger.Error("Failed to parse embedding vector", map[string]interface{}{
				"error": err.Error(),
				"id":    id,
			})
			continue
		}
		
		// Add to results
		results = append(results, &Embedding{
			ID:               id,
			ContextID:        contextID,
			ContentIndex:     contentIndex,
			Text:             text,
			Embedding:        embedding,
			VectorDimensions: dimensions,
			ModelID:          modelID,
			CreatedAt:        createdAt,
		})
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over embeddings: %w", err)
	}
	
	return results, nil
}

// GetEmbeddingsByModel gets all embeddings for a context filtered by model ID
func (r *EmbeddingRepository) GetEmbeddingsByModel(
	ctx context.Context, 
	contextID string, 
	modelID string,
) ([]*Embedding, error) {
	// Query embeddings for the context and model
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, context_id, content_index, text, embedding::text as embedding, 
		       vector_dimensions, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1 AND model_id = $2
		ORDER BY content_index
	`, contextID, modelID)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get context embeddings by model: %w", err)
	}
	defer rows.Close()
	
	// Process results
	var results []*Embedding
	
	for rows.Next() {
		var (
			id              string
			contextID       string
			contentIndex    int
			text            string
			embeddingStr    string
			dimensions      int
			modelID         string
			createdAt       time.Time
		)
		
		if err := rows.Scan(
			&id, &contextID, &contentIndex, &text, &embeddingStr, 
			&dimensions, &modelID, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan embedding: %w", err)
		}
		
		// Parse embedding vector from string
		embedding, err := parseEmbeddingVector(embeddingStr)
		if err != nil {
			r.logger.Error("Failed to parse embedding vector", map[string]interface{}{
				"error": err.Error(),
				"id":    id,
			})
			continue
		}
		
		// Add to results
		results = append(results, &Embedding{
			ID:               id,
			ContextID:        contextID,
			ContentIndex:     contentIndex,
			Text:             text,
			Embedding:        embedding,
			VectorDimensions: dimensions,
			ModelID:          modelID,
			CreatedAt:        createdAt,
		})
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over embeddings: %w", err)
	}
	
	return results, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (r *EmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	// Delete embeddings for the context
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM mcp.embeddings
		WHERE context_id = $1
	`, contextID)
	
	if err != nil {
		return fmt.Errorf("failed to delete context embeddings: %w", err)
	}
	
	return nil
}

// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
func (r *EmbeddingRepository) DeleteModelEmbeddings(
	ctx context.Context, 
	contextID string, 
	modelID string,
) error {
	// Delete embeddings for the context and model
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM mcp.embeddings
		WHERE context_id = $1 AND model_id = $2
	`, contextID, modelID)
	
	if err != nil {
		return fmt.Errorf("failed to delete model embeddings: %w", err)
	}
	
	return nil
}

// GetSupportedModels returns a list of embedding models used in the database
func (r *EmbeddingRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	// Query distinct model IDs
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT model_id
		FROM mcp.embeddings
		ORDER BY model_id
	`)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get supported models: %w", err)
	}
	defer rows.Close()
	
	// Process results
	var models []string
	
	for rows.Next() {
		var modelID string
		
		if err := rows.Scan(&modelID); err != nil {
			return nil, fmt.Errorf("failed to scan model ID: %w", err)
		}
		
		models = append(models, modelID)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over model IDs: %w", err)
	}
	
	return models, nil
}

// parseEmbeddingVector parses a PostgreSQL vector string into a float32 slice
func parseEmbeddingVector(vectorStr string) ([]float32, error) {
	// Remove brackets
	vectorStr = strings.TrimPrefix(vectorStr, "[")
	vectorStr = strings.TrimSuffix(vectorStr, "]")
	
	// Handle empty vector
	if vectorStr == "" {
		return []float32{}, nil
	}
	
	// Split components
	components := strings.Split(vectorStr, ",")
	
	// Parse components
	embedding := make([]float32, len(components))
	
	for i, comp := range components {
		var value float64
		if _, err := fmt.Sscanf(comp, "%f", &value); err != nil {
			return nil, fmt.Errorf("failed to parse embedding component: %w", err)
		}
		embedding[i] = float32(value)
	}
	
	return embedding, nil
}

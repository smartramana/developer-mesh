package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"github.com/jmoiron/sqlx"
)

// EmbeddingRepository handles vector storage and retrieval
type EmbeddingRepository struct {
	db *sqlx.DB
}

// NewEmbeddingRepository creates a new embedding repository
func NewEmbeddingRepository(db *sqlx.DB) *EmbeddingRepository {
	return &EmbeddingRepository{
		db: db,
	}
}

// Embedding represents a vector embedding of text
type Embedding struct {
	ID          string    `db:"id" json:"id"`
	ContextID   string    `db:"context_id" json:"context_id"`
	ContentIndex int       `db:"content_index" json:"content_index"`
	Text        string    `db:"text" json:"text"`
	Embedding   []float32 `db:"embedding" json:"embedding"`
	ModelID     string    `db:"model_id" json:"model_id"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// StoreEmbedding stores a vector embedding for a context item
func (r *EmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	query := `
		INSERT INTO mcp.embeddings (context_id, content_index, text, embedding, model_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	
	// Convert the Go slice to a PostgreSQL vector
	vectorStr := fmt.Sprintf("[%s]", strings.Join(floatSliceToStrings(embedding.Embedding), ","))
	
	var id string
	err := r.db.GetContext(ctx, &id, query, 
		embedding.ContextID, 
		embedding.ContentIndex,
		embedding.Text,
		vectorStr,
		embedding.ModelID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}
	
	embedding.ID = id
	return nil
}

// SearchEmbeddings searches for similar embeddings using vector similarity
func (r *EmbeddingRepository) SearchEmbeddings(
	ctx context.Context, 
	queryVector []float32, 
	contextID string, 
	limit int,
) ([]Embedding, error) {
	query := `
		SELECT id, context_id, content_index, text, embedding, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1
		ORDER BY embedding <-> $2
		LIMIT $3
	`
	
	// Convert the query vector to a PostgreSQL vector
	vectorStr := fmt.Sprintf("[%s]", strings.Join(floatSliceToStrings(queryVector), ","))
	
	var embeddings []Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, contextID, vectorStr, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	
	return embeddings, nil
}

// GetContextEmbeddings retrieves all embeddings for a context
func (r *EmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]Embedding, error) {
	query := `
		SELECT id, context_id, content_index, text, embedding, model_id, created_at
		FROM mcp.embeddings
		WHERE context_id = $1
	`
	
	var embeddings []Embedding
	err := r.db.SelectContext(ctx, &embeddings, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context embeddings: %w", err)
	}
	
	return embeddings, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (r *EmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	query := `
		DELETE FROM mcp.embeddings
		WHERE context_id = $1
	`
	
	_, err := r.db.ExecContext(ctx, query, contextID)
	if err != nil {
		return fmt.Errorf("failed to delete context embeddings: %w", err)
	}
	
	return nil
}

// Helper function to convert float slice to string slice
func floatSliceToStrings(floats []float32) []string {
	strings := make([]string, len(floats))
	for i, f := range floats {
		strings[i] = fmt.Sprintf("%f", f)
	}
	return strings
}

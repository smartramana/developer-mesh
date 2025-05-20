// Package vector provides interfaces and types for vector embeddings
package vector

import (
	"context"
	"time"
)

// Embedding represents a vector embedding with its metadata
type Embedding struct {
	ID           string                 `json:"id" db:"id"`
	ContextID    string                 `json:"context_id" db:"context_id"`
	ContentIndex int                    `json:"content_index" db:"content_index"`
	Text         string                 `json:"text" db:"text"`
	Embedding    []float32              `json:"embedding" db:"embedding"`
	ModelID      string                 `json:"model_id" db:"model_id"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	Metadata     map[string]interface{} `json:"metadata" db:"metadata"`
}

// Repository defines the interface for vector embedding operations
type Repository interface {
	// StoreEmbedding stores a vector embedding
	StoreEmbedding(ctx context.Context, embedding *Embedding) error

	// SearchEmbeddings performs a vector search with various filter options
	SearchEmbeddings(
		ctx context.Context,
		queryVector []float32,
		contextID string,
		modelID string,
		limit int,
		similarityThreshold float64,
	) ([]*Embedding, error)

	// SearchEmbeddings_Legacy performs a legacy vector search
	SearchEmbeddings_Legacy(
		ctx context.Context,
		queryVector []float32,
		contextID string,
		limit int,
	) ([]*Embedding, error)

	// GetContextEmbeddings retrieves all embeddings for a context
	GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)

	// DeleteContextEmbeddings deletes all embeddings for a context
	DeleteContextEmbeddings(ctx context.Context, contextID string) error

	// GetEmbeddingsByModel retrieves all embeddings for a context and model
	GetEmbeddingsByModel(
		ctx context.Context,
		contextID string,
		modelID string,
	) ([]*Embedding, error)

	// GetSupportedModels returns a list of models with embeddings
	GetSupportedModels(ctx context.Context) ([]string, error)

	// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
	DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error
}

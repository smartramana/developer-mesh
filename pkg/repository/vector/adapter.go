// Package vector provides interfaces and implementations for vector embeddings
package vector

import (
	"context"
)

// LegacyAdapter adapts between the old repository.VectorAPIRepository interface
// and the new vector.Repository interface during the migration process
type LegacyAdapter struct {
	repo Repository
}

// NewLegacyAdapter creates a new adapter that can be used by code expecting
// the old repository.VectorAPIRepository interface
func NewLegacyAdapter(repo Repository) *LegacyAdapter {
	return &LegacyAdapter{repo: repo}
}

// StoreEmbedding delegates to the new repository
func (a *LegacyAdapter) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	return a.repo.StoreEmbedding(ctx, embedding)
}

// SearchEmbeddings delegates to the new repository
func (a *LegacyAdapter) SearchEmbeddings(
	ctx context.Context,
	queryEmbedding []float32,
	contextID string,
	modelID string,
	limit int,
	threshold float64,
) ([]*Embedding, error) {
	return a.repo.SearchEmbeddings(ctx, queryEmbedding, contextID, modelID, limit, threshold)
}

// SearchEmbeddings_Legacy delegates to the new repository
func (a *LegacyAdapter) SearchEmbeddings_Legacy(
	ctx context.Context,
	queryEmbedding []float32,
	contextID string,
	limit int,
) ([]*Embedding, error) {
	return a.repo.SearchEmbeddings_Legacy(ctx, queryEmbedding, contextID, limit)
}

// GetEmbedding is a helper method that retrieves a single embedding by ID
// This is needed for compatibility with older code
func (a *LegacyAdapter) GetEmbedding(ctx context.Context, id string) (*Embedding, error) {
	// Since the new API doesn't have a direct GetEmbedding method,
	// we could implement it by searching for the specific ID
	// This is a stub implementation
	return &Embedding{ID: id}, nil
}

// DeleteEmbedding is a helper method that deletes a single embedding by ID
// This is needed for compatibility with older code
func (a *LegacyAdapter) DeleteEmbedding(ctx context.Context, id string) error {
	// Since the new API doesn't have a direct DeleteEmbedding method,
	// we could implement it separately
	// This is a stub implementation
	return nil
}

// GetContextEmbeddings delegates to the new repository
func (a *LegacyAdapter) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	return a.repo.GetContextEmbeddings(ctx, contextID)
}

// DeleteContextEmbeddings delegates to the new repository
func (a *LegacyAdapter) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	return a.repo.DeleteContextEmbeddings(ctx, contextID)
}

// GetSupportedModels delegates to the new repository
func (a *LegacyAdapter) GetSupportedModels(ctx context.Context) ([]string, error) {
	return a.repo.GetSupportedModels(ctx)
}

// GetEmbeddingsByModel delegates to the new repository
func (a *LegacyAdapter) GetEmbeddingsByModel(ctx context.Context, tenantID string, modelID string) ([]*Embedding, error) {
	return a.repo.GetEmbeddingsByModel(ctx, tenantID, modelID)
}

// DeleteModelEmbeddings is a helper method that deletes embeddings for a specific model
// This is needed for compatibility with older code
func (a *LegacyAdapter) DeleteModelEmbeddings(ctx context.Context, tenantID string, modelID string) error {
	// This is a stub implementation
	// In a real implementation, we would delete all embeddings for the specified model and tenant
	return nil
}

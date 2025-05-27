// Package repository provides factory methods for creating repository instances
package repository

import (
	"context"
	"database/sql"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	pkgrepo "github.com/S-Corkum/devops-mcp/pkg/repository"
)

// NewAgentRepository creates a new agent repository using the adapter pattern
func NewAgentRepository(db *sql.DB) AgentRepository {
	// Create the core repository
	coreRepo := pkgrepo.NewAgentRepositoryAdapter(db)

	// Wrap it with an adapter that implements the expected interface
	return &AgentRepositoryAdapter{
		repo: coreRepo,
	}
}

// NewModelRepository creates a new model repository using the adapter pattern
func NewModelRepository(db *sql.DB) ModelRepository {
	// Create the core repository
	coreRepo := pkgrepo.NewModelRepository(db)

	// Wrap it with an adapter that implements the expected interface
	return &ModelRepositoryAdapter{
		repo: coreRepo,
	}
}

// NewVectorRepository creates a new vector repository adapter
func NewVectorRepository(db *sql.DB) VectorAPIRepository {
	// Create the core repository using the adapter pattern
	coreRepo := pkgrepo.NewEmbeddingAdapter(db)

	// Wrap it with an adapter that implements the expected interface
	return &VectorRepositoryAdapter{
		repo: coreRepo,
	}
}

// AgentRepositoryAdapter adapts the pkg.repository.AgentRepository to the internal AgentRepository interface
type AgentRepositoryAdapter struct {
	repo pkgrepo.AgentRepository
}

// CreateAgent creates a new agent
func (a *AgentRepositoryAdapter) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return a.repo.Create(ctx, agent)
}

// GetAgentByID retrieves an agent by ID and tenant ID
func (a *AgentRepositoryAdapter) GetAgentByID(ctx context.Context, tenantID, id string) (*models.Agent, error) {
	// First get the agent by ID
	agent, err := a.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify this agent belongs to the requested tenant
	if agent != nil && agent.TenantID != tenantID {
		return nil, nil // Not found for this tenant
	}

	return agent, nil
}

// ListAgents retrieves all agents for a given tenant
func (a *AgentRepositoryAdapter) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	// Create a filter map for the tenant ID
	filter := map[string]interface{}{
		"tenant_id": tenantID,
	}
	return a.repo.List(ctx, filter)
}

// UpdateAgent updates an existing agent
func (a *AgentRepositoryAdapter) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	return a.repo.Update(ctx, agent)
}

// DeleteAgent deletes an agent by ID
func (a *AgentRepositoryAdapter) DeleteAgent(ctx context.Context, id string) error {
	return a.repo.Delete(ctx, id)
}

// ModelRepositoryAdapter adapts the pkg.repository.ModelRepository to the internal ModelRepository interface
type ModelRepositoryAdapter struct {
	repo pkgrepo.ModelRepository
}

// CreateModel creates a new model
func (m *ModelRepositoryAdapter) CreateModel(ctx context.Context, model *models.Model) error {
	return m.repo.Create(ctx, model)
}

// GetModelByID retrieves a model by ID and tenant ID
func (m *ModelRepositoryAdapter) GetModelByID(ctx context.Context, tenantID, id string) (*models.Model, error) {
	// First get the model by ID
	model, err := m.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Verify this model belongs to the requested tenant
	if model != nil && model.TenantID != tenantID {
		return nil, nil // Not found for this tenant
	}

	return model, nil
}

// ListModels retrieves all models for a given tenant
func (m *ModelRepositoryAdapter) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	filter := map[string]interface{}{
		"tenant_id": tenantID,
	}
	return m.repo.List(ctx, filter)
}

// UpdateModel updates an existing model
func (m *ModelRepositoryAdapter) UpdateModel(ctx context.Context, model *models.Model) error {
	return m.repo.Update(ctx, model)
}

// DeleteModel deletes a model by ID
func (m *ModelRepositoryAdapter) DeleteModel(ctx context.Context, id string) error {
	return m.repo.Delete(ctx, id)
}

// VectorRepositoryAdapter adapts the pkg repository embedding implementation to our VectorAPIRepository interface
type VectorRepositoryAdapter struct {
	repo pkgrepo.VectorAPIRepository
}

// StoreEmbedding stores a vector embedding
func (v *VectorRepositoryAdapter) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	// Convert to pkg repository embedding using the adapter pattern
	pkgEmb := &pkgrepo.Embedding{
		ID:           embedding.ID,
		ContextID:    embedding.ContextID,
		ContentIndex: embedding.ContentIndex,
		Text:         embedding.Text,
		Embedding:    embedding.Embedding,
		ModelID:      embedding.ModelID,
	}
	return v.repo.StoreEmbedding(ctx, pkgEmb)
}

// SearchEmbeddings performs a vector search with various filter options
func (v *VectorRepositoryAdapter) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*Embedding, error) {
	// Call the underlying repository
	pkgResults, err := v.repo.SearchEmbeddings(ctx, queryVector, contextID, modelID, limit, similarityThreshold)
	if err != nil {
		return nil, err
	}

	// Convert to internal embeddings using the adapter pattern
	results := make([]*Embedding, len(pkgResults))
	for i, pkgEmb := range pkgResults {
		if pkgEmb == nil {
			continue
		}
		results[i] = &Embedding{
			ID:           pkgEmb.ID,
			ContextID:    pkgEmb.ContextID,
			ContentIndex: pkgEmb.ContentIndex,
			Text:         pkgEmb.Text,
			Embedding:    pkgEmb.Embedding,
			ModelID:      pkgEmb.ModelID,
		}
	}
	return results, nil
}

// SearchEmbeddings_Legacy performs a legacy vector search
func (v *VectorRepositoryAdapter) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error) {
	// Call the underlying repository
	pkgResults, err := v.repo.SearchEmbeddings_Legacy(ctx, queryVector, contextID, limit)
	if err != nil {
		return nil, err
	}

	// Convert to internal embeddings using the adapter pattern
	results := make([]*Embedding, len(pkgResults))
	for i, pkgEmb := range pkgResults {
		if pkgEmb == nil {
			continue
		}
		results[i] = &Embedding{
			ID:           pkgEmb.ID,
			ContextID:    pkgEmb.ContextID,
			ContentIndex: pkgEmb.ContentIndex,
			Text:         pkgEmb.Text,
			Embedding:    pkgEmb.Embedding,
			ModelID:      pkgEmb.ModelID,
		}
	}
	return results, nil
}

// GetContextEmbeddings retrieves all embeddings for a context
func (v *VectorRepositoryAdapter) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	// Call the underlying repository
	pkgResults, err := v.repo.GetContextEmbeddings(ctx, contextID)
	if err != nil {
		return nil, err
	}

	// Convert to internal embeddings using the adapter pattern
	results := make([]*Embedding, len(pkgResults))
	for i, pkgEmb := range pkgResults {
		if pkgEmb == nil {
			continue
		}
		results[i] = &Embedding{
			ID:           pkgEmb.ID,
			ContextID:    pkgEmb.ContextID,
			ContentIndex: pkgEmb.ContentIndex,
			Text:         pkgEmb.Text,
			Embedding:    pkgEmb.Embedding,
			ModelID:      pkgEmb.ModelID,
		}
	}
	return results, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (v *VectorRepositoryAdapter) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	return v.repo.DeleteContextEmbeddings(ctx, contextID)
}

// GetEmbeddingsByModel retrieves all embeddings for a context and model
func (v *VectorRepositoryAdapter) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error) {
	// Call the underlying repository
	pkgResults, err := v.repo.GetEmbeddingsByModel(ctx, contextID, modelID)
	if err != nil {
		return nil, err
	}

	// Convert to internal embeddings using the adapter pattern
	results := make([]*Embedding, len(pkgResults))
	for i, pkgEmb := range pkgResults {
		if pkgEmb == nil {
			continue
		}
		results[i] = &Embedding{
			ID:           pkgEmb.ID,
			ContextID:    pkgEmb.ContextID,
			ContentIndex: pkgEmb.ContentIndex,
			Text:         pkgEmb.Text,
			Embedding:    pkgEmb.Embedding,
			ModelID:      pkgEmb.ModelID,
		}
	}
	return results, nil
}

// GetSupportedModels returns a list of models with embeddings
func (v *VectorRepositoryAdapter) GetSupportedModels(ctx context.Context) ([]string, error) {
	return v.repo.GetSupportedModels(ctx)
}

// DeleteModelEmbeddings deletes all embeddings for a specific model in a context
func (v *VectorRepositoryAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	return v.repo.DeleteModelEmbeddings(ctx, contextID, modelID)
}

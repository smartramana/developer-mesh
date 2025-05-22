package proxies

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/client/rest"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/pkg/repository/vector"
)

// VectorAPIProxy implements the embedding repository interface but delegates to the REST API
type VectorAPIProxy struct {
	client  *rest.VectorClient
	logger  observability.Logger
}

// NewVectorAPIProxy creates a new VectorAPIProxy
func NewVectorAPIProxy(factory *rest.Factory, logger observability.Logger) *VectorAPIProxy {
	return &VectorAPIProxy{
		client: factory.Vector(),
		logger: logger,
	}
}

// StoreEmbedding stores a vector embedding by delegating to the REST API
func (p *VectorAPIProxy) StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error {
	p.logger.Debug("Storing embedding via REST API proxy", map[string]interface{}{
		"vector_id": embedding.ID,
		"tenant_id": embedding.ContextID,
	})
	
	// Convert from repository.Embedding to models.Vector for the REST API
	vector := &models.Vector{
		ID:        embedding.ID,
		TenantID:  embedding.ContextID,
		Content:   embedding.Text,
		Embedding: embedding.Embedding,
		Metadata: map[string]interface{}{
			"content_index": embedding.ContentIndex,
			"model_id":     embedding.ModelID,
		},
	}
	
	return p.client.StoreEmbedding(ctx, vector)
}

// SearchEmbeddings searches for similar embeddings by delegating to the REST API
func (p *VectorAPIProxy) SearchEmbeddings(ctx context.Context, queryEmbedding []float32, contextID string, modelID string, limit int, threshold float64) ([]*repository.Embedding, error) {
	p.logger.Debug("Searching embeddings via REST API proxy", map[string]interface{}{
		"context_id": contextID,
		"model_id":   modelID,
		"limit":      limit,
		"threshold":  threshold,
	})
	
	// Get the models.Vector objects from the REST API
	vectors, err := p.client.SearchEmbeddings(ctx, queryEmbedding, contextID, modelID, limit, threshold)
	if err != nil {
		return nil, err
	}
	
	// Convert from models.Vector to repository.Embedding
	embeddings := make([]*repository.Embedding, len(vectors))
	for i, vector := range vectors {
		embeddings[i] = &repository.Embedding{
			ID:           vector.ID,
			ContextID:    vector.TenantID,
			ContentIndex: getContentIndexFromMetadata(vector.Metadata),
			Text:         vector.Content,
			Embedding:    vector.Embedding,
			ModelID:      getModelIDFromMetadata(vector.Metadata),
		}
	}
	
	return embeddings, nil
}

// SearchEmbeddings_Legacy is a legacy method that delegates to the REST API
func (p *VectorAPIProxy) SearchEmbeddings_Legacy(ctx context.Context, queryEmbedding []float32, contextID string, limit int) ([]*repository.Embedding, error) {
	p.logger.Debug("Searching embeddings (legacy) via REST API proxy", map[string]interface{}{
		"context_id": contextID,
		"limit":      limit,
	})
	
	// Get the models.Vector objects from the REST API
	vectors, err := p.client.SearchEmbeddings_Legacy(ctx, queryEmbedding, contextID, limit)
	if err != nil {
		return nil, err
	}
	
	// Convert from models.Vector to repository.Embedding
	embeddings := make([]*repository.Embedding, len(vectors))
	for i, vector := range vectors {
		embeddings[i] = &repository.Embedding{
			ID:           vector.ID,
			ContextID:    vector.TenantID,
			ContentIndex: getContentIndexFromMetadata(vector.Metadata),
			Text:         vector.Content,
			Embedding:    vector.Embedding,
			ModelID:      getModelIDFromMetadata(vector.Metadata),
		}
	}
	
	return embeddings, nil
}

// GetContextEmbeddings retrieves all embeddings for a context by delegating to the REST API
func (p *VectorAPIProxy) GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error) {
	p.logger.Debug("Getting context embeddings via REST API proxy", map[string]interface{}{
		"context_id": contextID,
	})
	
	// Get the models.Vector objects from the REST API
	vectors, err := p.client.GetContextEmbeddings(ctx, contextID)
	if err != nil {
		return nil, err
	}
	
	// Convert from models.Vector to repository.Embedding
	embeddings := make([]*repository.Embedding, len(vectors))
	for i, vector := range vectors {
		embeddings[i] = &repository.Embedding{
			ID:           vector.ID,
			ContextID:    vector.TenantID,
			ContentIndex: getContentIndexFromMetadata(vector.Metadata),
			Text:         vector.Content,
			Embedding:    vector.Embedding,
			ModelID:      getModelIDFromMetadata(vector.Metadata),
		}
	}
	
	return embeddings, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context by delegating to the REST API
func (p *VectorAPIProxy) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	p.logger.Debug("Deleting context embeddings via REST API proxy", map[string]interface{}{
		"context_id": contextID,
	})
	
	return p.client.DeleteContextEmbeddings(ctx, contextID)
}

// GetEmbeddingsByModel retrieves all embeddings for a context and model by delegating to the REST API
func (p *VectorAPIProxy) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error) {
	p.logger.Debug("Getting embeddings by model via REST API proxy", map[string]interface{}{
		"context_id": contextID,
		"model_id":   modelID,
	})
	
	// Get the models.Vector objects from the REST API
	vectors, err := p.client.GetEmbeddingsByModel(ctx, contextID, modelID)
	if err != nil {
		return nil, err
	}
	
	// Convert from models.Vector to repository.Embedding
	embeddings := make([]*repository.Embedding, len(vectors))
	for i, vector := range vectors {
		embeddings[i] = &repository.Embedding{
			ID:           vector.ID,
			ContextID:    vector.TenantID,
			ContentIndex: getContentIndexFromMetadata(vector.Metadata),
			Text:         vector.Content,
			Embedding:    vector.Embedding,
			ModelID:      getModelIDFromMetadata(vector.Metadata),
		}
	}
	
	return embeddings, nil
}

// DeleteModelEmbeddings deletes all embeddings for a context and model by delegating to the REST API
func (p *VectorAPIProxy) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	p.logger.Debug("Deleting model embeddings via REST API proxy", map[string]interface{}{
		"context_id": contextID,
		"model_id":   modelID,
	})
	
	return p.client.DeleteModelEmbeddings(ctx, contextID, modelID)
}

// GetSupportedModels retrieves all supported models by delegating to the REST API
func (p *VectorAPIProxy) GetSupportedModels(ctx context.Context) ([]string, error) {
	p.logger.Debug("Getting supported models via REST API proxy", map[string]interface{}{})
	
	return p.client.GetSupportedModels(ctx)
}

// DeleteEmbedding deletes a single embedding by ID by delegating to the REST API
func (p *VectorAPIProxy) DeleteEmbedding(ctx context.Context, id string) error {
	p.logger.Debug("Deleting embedding via REST API proxy", map[string]interface{}{
		"id": id,
	})
	
	// Delegate to the DeleteEmbedding endpoint in the REST API
	return p.client.DeleteEmbedding(ctx, id)
}

// GetEmbedding retrieves a single embedding by ID by delegating to the REST API
func (p *VectorAPIProxy) GetEmbedding(ctx context.Context, id string) (*repository.Embedding, error) {
	p.logger.Debug("Getting embedding by ID via REST API proxy", map[string]interface{}{
		"id": id,
	})
	
	// Get the models.Vector from the REST API
	vector, err := p.client.GetEmbedding(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Convert from models.Vector to repository.Embedding
	embedding := &repository.Embedding{
		ID:           vector.ID,
		ContextID:    vector.TenantID,
		ContentIndex: getContentIndexFromMetadata(vector.Metadata),
		Text:         vector.Content,
		Embedding:    vector.Embedding,
		ModelID:      getModelIDFromMetadata(vector.Metadata),
	}
	
	return embedding, nil
}

// Helper functions to extract metadata from the Vector model
func getContentIndexFromMetadata(metadata map[string]interface{}) int {
	if metadata == nil {
		return 0
	}
	
	if contentIndex, ok := metadata["content_index"]; ok {
		switch v := contentIndex.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	
	return 0
}

func getModelIDFromMetadata(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	
	if modelID, ok := metadata["model_id"]; ok {
		if modelIDStr, ok := modelID.(string); ok {
			return modelIDStr
		}
	}
	
	return ""
}

// Ensure that VectorAPIProxy implements repository.VectorAPIRepository
var _ repository.VectorAPIRepository = (*VectorAPIProxy)(nil)

// The following methods implement the standard Repository[Embedding] interface

// Create implements Repository[Embedding].Create
func (p *VectorAPIProxy) Create(ctx context.Context, embedding *repository.Embedding) error {
	// Delegate to StoreEmbedding for backward compatibility
	return p.StoreEmbedding(ctx, embedding)
}

// Get implements Repository[Embedding].Get
func (p *VectorAPIProxy) Get(ctx context.Context, id string) (*repository.Embedding, error) {
	// Delegate to GetEmbedding for backward compatibility
	return p.GetEmbedding(ctx, id)
}

// List implements Repository[Embedding].List
func (p *VectorAPIProxy) List(ctx context.Context, filter vector.Filter) ([]*repository.Embedding, error) {
	p.logger.Debug("Listing embeddings via REST API proxy", map[string]interface{}{
		"filter": filter,
	})
	
	// Extract common filter parameters
	var contextID string
	if contextIDVal, ok := filter["context_id"]; ok {
		if contextIDStr, ok := contextIDVal.(string); ok {
			contextID = contextIDStr
		}
	}
	
	// If we have a context ID but no other filters, use GetContextEmbeddings
	if contextID != "" {
		var modelID string
		if modelIDVal, ok := filter["model_id"]; ok {
			if modelIDStr, ok := modelIDVal.(string); ok {
				modelID = modelIDStr
			}
		}
		
		if modelID != "" {
			// If we have both context and model ID, use GetEmbeddingsByModel
			return p.GetEmbeddingsByModel(ctx, contextID, modelID)
		}
		
		// Otherwise just get all embeddings for the context
		return p.GetContextEmbeddings(ctx, contextID)
	}
	
	// If no context ID, we can't list all (not supported in REST API)
	// Return empty list for now
	p.logger.Warn("List without context_id not supported in REST API", nil)
	return []*repository.Embedding{}, nil
}

// Update implements Repository[Embedding].Update
func (p *VectorAPIProxy) Update(ctx context.Context, embedding *repository.Embedding) error {
	// Delegate to StoreEmbedding for backward compatibility
	// The REST API uses upsert semantics for StoreEmbedding
	return p.StoreEmbedding(ctx, embedding)
}

// Delete implements Repository[Embedding].Delete
func (p *VectorAPIProxy) Delete(ctx context.Context, id string) error {
	// Delegate to DeleteEmbedding for backward compatibility
	return p.DeleteEmbedding(ctx, id)
}

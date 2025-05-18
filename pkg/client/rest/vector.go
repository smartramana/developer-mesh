package rest

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// VectorClient provides methods for interacting with the Vector API
type VectorClient struct {
	client *RESTClient
}

// NewVectorClient creates a new Vector API client
func NewVectorClient(client *RESTClient) *VectorClient {
	return &VectorClient{
		client: client,
	}
}

// StoreEmbedding stores a vector embedding
func (c *VectorClient) StoreEmbedding(ctx context.Context, vector *models.Vector) error {
	path := "/api/v1/vectors/store"
	
	// REST API expects specific field names, so create a conversion
	requestBody := map[string]interface{}{
		"context_id":    vector.TenantID,
		"content_index": getContentIndexFromMetadata(vector.Metadata),
		"text":          vector.Content,
		"embedding":     vector.Embedding,
		"model_id":      getModelIDFromMetadata(vector.Metadata),
	}
	
	var response map[string]interface{}
	return c.client.Post(ctx, path, requestBody, &response)
}

// SearchEmbeddings searches for similar embeddings
func (c *VectorClient) SearchEmbeddings(ctx context.Context, queryEmbedding []float32, contextID string, modelID string, limit int, threshold float64) ([]*models.Vector, error) {
	path := "/api/v1/vectors/search"
	
	requestBody := map[string]interface{}{
		"context_id":           contextID,
		"query_embedding":      queryEmbedding,
		"model_id":             modelID,
		"limit":                limit,
		"similarity_threshold": threshold,
	}
	
	var response struct {
		Embeddings []*models.Vector `json:"embeddings"`
	}
	
	if err := c.client.Post(ctx, path, requestBody, &response); err != nil {
		return nil, err
	}
	
	return response.Embeddings, nil
}

// SearchEmbeddings_Legacy is a legacy method that searches for similar embeddings without model ID
func (c *VectorClient) SearchEmbeddings_Legacy(ctx context.Context, queryEmbedding []float32, contextID string, limit int) ([]*models.Vector, error) {
	path := "/api/v1/vectors/search-legacy"
	
	requestBody := map[string]interface{}{
		"context_id":      contextID,
		"query_embedding": queryEmbedding,
		"limit":           limit,
	}
	
	var response struct {
		Embeddings []*models.Vector `json:"embeddings"`
	}
	
	if err := c.client.Post(ctx, path, requestBody, &response); err != nil {
		return nil, err
	}
	
	return response.Embeddings, nil
}

// GetContextEmbeddings retrieves all embeddings for a context
func (c *VectorClient) GetContextEmbeddings(ctx context.Context, contextID string) ([]*models.Vector, error) {
	path := fmt.Sprintf("/api/v1/vectors/context/%s", contextID)
	
	var response struct {
		Embeddings []*models.Vector `json:"embeddings"`
	}
	
	if err := c.client.Get(ctx, path, &response); err != nil {
		return nil, err
	}
	
	return response.Embeddings, nil
}

// DeleteContextEmbeddings deletes all embeddings for a context
func (c *VectorClient) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	path := fmt.Sprintf("/api/v1/vectors/context/%s", contextID)
	
	return c.client.Delete(ctx, path, nil)
}

// GetEmbeddingsByModel retrieves all embeddings for a context and model
func (c *VectorClient) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*models.Vector, error) {
	path := fmt.Sprintf("/api/v1/vectors/context/%s/model/%s", contextID, modelID)
	
	var response struct {
		Embeddings []*models.Vector `json:"embeddings"`
	}
	
	if err := c.client.Get(ctx, path, &response); err != nil {
		return nil, err
	}
	
	return response.Embeddings, nil
}

// DeleteModelEmbeddings deletes all embeddings for a context and model
func (c *VectorClient) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	path := fmt.Sprintf("/api/v1/vectors/context/%s/model/%s", contextID, modelID)
	
	return c.client.Delete(ctx, path, nil)
}

// GetSupportedModels retrieves all supported models
func (c *VectorClient) GetSupportedModels(ctx context.Context) ([]string, error) {
	path := "/api/v1/vectors/models"
	
	var response struct {
		Models []string `json:"models"`
	}
	
	if err := c.client.Get(ctx, path, &response); err != nil {
		return nil, err
	}
	
	return response.Models, nil
}

// DeleteEmbedding deletes a single embedding by ID
func (c *VectorClient) DeleteEmbedding(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/vectors/%s", id)
	return c.client.Delete(ctx, path, nil)
}

// GetEmbedding retrieves a single embedding by ID
func (c *VectorClient) GetEmbedding(ctx context.Context, id string) (*models.Vector, error) {
	path := fmt.Sprintf("/api/v1/vectors/%s", id)
	
	var vector models.Vector
	if err := c.client.Get(ctx, path, &vector); err != nil {
		return nil, err
	}
	
	return &vector, nil
}

// Helper functions to extract metadata
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

package rest

import (
	"context"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/models"
)

// ModelClient provides methods for interacting with the Model API
type ModelClient struct {
	client *RESTClient
}

// NewModelClient creates a new Model API client
func NewModelClient(client *RESTClient) *ModelClient {
	return &ModelClient{
		client: client,
	}
}

// CreateModel creates a new model
func (c *ModelClient) CreateModel(ctx context.Context, model *models.Model) error {
	path := "/api/v1/models"
	
	var response map[string]interface{}
	return c.client.Post(ctx, path, model, &response)
}

// GetModelByID retrieves a model by ID
func (c *ModelClient) GetModelByID(ctx context.Context, id string) (*models.Model, error) {
	path := fmt.Sprintf("/api/v1/models/%s", id)
	
	var model models.Model
	if err := c.client.Get(ctx, path, &model); err != nil {
		return nil, err
	}
	
	return &model, nil
}

// ListModels retrieves all models
func (c *ModelClient) ListModels(ctx context.Context) ([]*models.Model, error) {
	path := "/api/v1/models"
	
	var response struct {
		Models []*models.Model `json:"models"`
	}
	
	if err := c.client.Get(ctx, path, &response); err != nil {
		return nil, err
	}
	
	return response.Models, nil
}

// UpdateModel updates an existing model
func (c *ModelClient) UpdateModel(ctx context.Context, model *models.Model) error {
	path := fmt.Sprintf("/api/v1/models/%s", model.ID)
	
	var response map[string]interface{}
	return c.client.Put(ctx, path, model, &response)
}

// DeleteModel deletes a model by ID
func (c *ModelClient) DeleteModel(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/models/%s", id)
	
	return c.client.Delete(ctx, path, nil)
}

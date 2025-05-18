package proxies

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/client/rest"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// ModelAPIProxy implements the model repository interface but delegates to the REST API
type ModelAPIProxy struct {
	client *rest.ModelClient
	logger observability.Logger
}

// NewModelAPIProxy creates a new ModelAPIProxy
func NewModelAPIProxy(factory *rest.Factory, logger observability.Logger) *ModelAPIProxy {
	return &ModelAPIProxy{
		client: factory.Model(),
		logger: logger,
	}
}

// CreateModel creates a new model by delegating to the REST API
func (p *ModelAPIProxy) CreateModel(ctx context.Context, model *models.Model) error {
	p.logger.Debug("Creating model via REST API proxy", map[string]interface{}{
		"model_id": model.ID,
		"name":     model.Name,
	})
	
	return p.client.CreateModel(ctx, model)
}

// GetModelByID retrieves a model by ID by delegating to the REST API
func (p *ModelAPIProxy) GetModelByID(ctx context.Context, id string, tenantID string) (*models.Model, error) {
	p.logger.Debug("Getting model by ID via REST API proxy", map[string]interface{}{
		"model_id": id,
		"tenant_id": tenantID,
	})
	
	// The REST client's GetModelByID doesn't take a tenantID parameter, but we include it in the interface
	// for consistency with other repository interfaces
	return p.client.GetModelByID(ctx, id)
}

// ListModels retrieves all models by delegating to the REST API
func (p *ModelAPIProxy) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	p.logger.Debug("Listing models via REST API proxy", map[string]interface{}{
		"tenant_id": tenantID,
	})
	
	// The tenantID parameter is ignored in the current implementation
	// but kept for interface compatibility
	return p.client.ListModels(ctx)
}

// UpdateModel updates an existing model by delegating to the REST API
func (p *ModelAPIProxy) UpdateModel(ctx context.Context, model *models.Model) error {
	p.logger.Debug("Updating model via REST API proxy", map[string]interface{}{
		"model_id": model.ID,
		"name":     model.Name,
	})
	
	return p.client.UpdateModel(ctx, model)
}

// DeleteModel deletes a model by ID by delegating to the REST API
func (p *ModelAPIProxy) DeleteModel(ctx context.Context, id string) error {
	p.logger.Debug("Deleting model via REST API proxy", map[string]interface{}{
		"model_id": id,
	})
	
	return p.client.DeleteModel(ctx, id)
}

// Create implements the Create method of the repository.ModelRepository interface
// It delegates to CreateModel for API compatibility
func (p *ModelAPIProxy) Create(ctx context.Context, model *models.Model) error {
	return p.CreateModel(ctx, model)
}

// Get implements the Get method of the repository.ModelRepository interface
// It delegates to GetModelByID for API compatibility
func (p *ModelAPIProxy) Get(ctx context.Context, id string) (*models.Model, error) {
	return p.GetModelByID(ctx, id, "")
}

// List implements the List method of the repository.ModelRepository interface
// It delegates to ListModels for API compatibility
func (p *ModelAPIProxy) List(ctx context.Context, filters map[string]interface{}) ([]*models.Model, error) {
	// Extract tenantID from filters if present
	var tenantID string
	if tenantIDVal, ok := filters["tenant_id"]; ok {
		if tenantIDStr, ok := tenantIDVal.(string); ok {
			tenantID = tenantIDStr
		}
	}
	
	return p.ListModels(ctx, tenantID)
}

// Update implements the Update method of the repository.ModelRepository interface
// It delegates to UpdateModel for API compatibility
func (p *ModelAPIProxy) Update(ctx context.Context, model *models.Model) error {
	return p.UpdateModel(ctx, model)
}

// Delete implements the Delete method of the repository.ModelRepository interface
// It delegates to DeleteModel for API compatibility
func (p *ModelAPIProxy) Delete(ctx context.Context, id string) error {
	return p.DeleteModel(ctx, id)
}

// Ensure that ModelAPIProxy implements repository.ModelRepository
var _ repository.ModelRepository = (*ModelAPIProxy)(nil)

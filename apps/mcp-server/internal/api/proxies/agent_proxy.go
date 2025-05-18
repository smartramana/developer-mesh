package proxies

import (
	"context"

	"github.com/S-Corkum/devops-mcp/pkg/client/rest"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// AgentAPIProxy implements the agent repository interface but delegates to the REST API
type AgentAPIProxy struct {
	client *rest.AgentClient
	logger observability.Logger
}

// NewAgentAPIProxy creates a new AgentAPIProxy
func NewAgentAPIProxy(factory *rest.Factory, logger observability.Logger) *AgentAPIProxy {
	return &AgentAPIProxy{
		client: factory.Agent(),
		logger: logger,
	}
}

// CreateAgent creates a new agent by delegating to the REST API
func (p *AgentAPIProxy) CreateAgent(ctx context.Context, agent *models.Agent) error {
	p.logger.Debug("Creating agent via REST API proxy", map[string]interface{}{
		"agent_id": agent.ID,
		"name":     agent.Name,
	})
	
	return p.client.CreateAgent(ctx, agent)
}

// GetAgentByID retrieves an agent by ID by delegating to the REST API
func (p *AgentAPIProxy) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	p.logger.Debug("Getting agent by ID via REST API proxy", map[string]interface{}{
		"agent_id":  id,
		"tenant_id": tenantID,
	})
	
	return p.client.GetAgentByID(ctx, id, tenantID)
}

// ListAgents retrieves all agents for a tenant by delegating to the REST API
func (p *AgentAPIProxy) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	p.logger.Debug("Listing agents via REST API proxy", map[string]interface{}{
		"tenant_id": tenantID,
	})
	
	return p.client.ListAgents(ctx, tenantID)
}

// UpdateAgent updates an existing agent by delegating to the REST API
func (p *AgentAPIProxy) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	p.logger.Debug("Updating agent via REST API proxy", map[string]interface{}{
		"agent_id": agent.ID,
		"name":     agent.Name,
	})
	
	return p.client.UpdateAgent(ctx, agent)
}

// DeleteAgent deletes an agent by ID by delegating to the REST API
func (p *AgentAPIProxy) DeleteAgent(ctx context.Context, id string) error {
	p.logger.Debug("Deleting agent via REST API proxy", map[string]interface{}{
		"agent_id": id,
	})
	
	return p.client.DeleteAgent(ctx, id)
}

// Create implements the Create method of the repository.AgentRepository interface
// It delegates to CreateAgent for API compatibility
func (p *AgentAPIProxy) Create(ctx context.Context, agent *models.Agent) error {
	return p.CreateAgent(ctx, agent)
}

// Get implements the Get method of the repository.AgentRepository interface
// It delegates to GetAgentByID for API compatibility but only uses the id parameter
// and passes an empty tenant ID since the interface doesn't include it
func (p *AgentAPIProxy) Get(ctx context.Context, id string) (*models.Agent, error) {
	// Using an empty tenantID since the interface doesn't include it
	// The REST client will need to handle this properly
	return p.GetAgentByID(ctx, id, "")
}

// List implements the List method of the repository.AgentRepository interface
// It expects a filter map but extracts the tenantID if present and delegates to ListAgents
func (p *AgentAPIProxy) List(ctx context.Context, filter map[string]interface{}) ([]*models.Agent, error) {
	// Extract tenantID from filter if available
	tenantID := ""
	if val, ok := filter["tenant_id"]; ok {
		if strVal, isStr := val.(string); isStr {
			tenantID = strVal
		}
	}
	return p.ListAgents(ctx, tenantID)
}

// Update implements the Update method of the repository.AgentRepository interface
// It delegates to UpdateAgent for API compatibility
func (p *AgentAPIProxy) Update(ctx context.Context, agent *models.Agent) error {
	return p.UpdateAgent(ctx, agent)
}

// Delete implements the Delete method of the repository.AgentRepository interface
// It delegates to DeleteAgent for API compatibility
func (p *AgentAPIProxy) Delete(ctx context.Context, id string) error {
	return p.DeleteAgent(ctx, id)
}

// Ensure that AgentAPIProxy implements repository.AgentRepository
var _ repository.AgentRepository = (*AgentAPIProxy)(nil)

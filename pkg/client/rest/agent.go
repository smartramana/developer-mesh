package rest

import (
	"context"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// AgentClient provides methods for interacting with the Agent API
type AgentClient struct {
	client *RESTClient
}

// NewAgentClient creates a new Agent API client
func NewAgentClient(client *RESTClient) *AgentClient {
	return &AgentClient{
		client: client,
	}
}

// CreateAgent creates a new agent
func (c *AgentClient) CreateAgent(ctx context.Context, agent *models.Agent) error {
	path := "/api/v1/agents"

	var response map[string]interface{}
	return c.client.Post(ctx, path, agent, &response)
}

// GetAgentByID retrieves an agent by ID
func (c *AgentClient) GetAgentByID(ctx context.Context, id string, tenantID string) (*models.Agent, error) {
	path := fmt.Sprintf("/api/v1/agents/%s", id)

	// Add tenant ID as query parameter
	if tenantID != "" {
		path = fmt.Sprintf("%s?tenant_id=%s", path, tenantID)
	}

	var agent models.Agent
	if err := c.client.Get(ctx, path, &agent); err != nil {
		return nil, err
	}

	return &agent, nil
}

// ListAgents retrieves all agents for a tenant
func (c *AgentClient) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	path := "/api/v1/agents"

	// Add tenant ID as query parameter
	if tenantID != "" {
		path = fmt.Sprintf("%s?tenant_id=%s", path, tenantID)
	}

	var response struct {
		Agents []*models.Agent `json:"agents"`
	}

	if err := c.client.Get(ctx, path, &response); err != nil {
		return nil, err
	}

	return response.Agents, nil
}

// UpdateAgent updates an existing agent
func (c *AgentClient) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	path := fmt.Sprintf("/api/v1/agents/%s", agent.ID)

	var response map[string]interface{}
	return c.client.Put(ctx, path, agent, &response)
}

// DeleteAgent deletes an agent by ID
func (c *AgentClient) DeleteAgent(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/agents/%s", id)

	return c.client.Delete(ctx, path, nil)
}

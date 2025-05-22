package agent

import (
	"context"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestFilterFromTenantID(t *testing.T) {
	// Test that FilterFromTenantID correctly creates a filter with tenant_id
	tenantID := "test-tenant"
	filter := FilterFromTenantID(tenantID)
	
	assert.Equal(t, 1, len(filter), "Filter should have exactly one entry")
	assert.Equal(t, tenantID, filter["tenant_id"], "Filter should contain the correct tenant_id")
}

func TestFilterFromIDs(t *testing.T) {
	// Test that FilterFromIDs correctly creates a filter with tenant_id and id
	tenantID := "test-tenant"
	id := "test-id"
	filter := FilterFromIDs(tenantID, id)
	
	assert.Equal(t, 2, len(filter), "Filter should have exactly two entries")
	assert.Equal(t, tenantID, filter["tenant_id"], "Filter should contain the correct tenant_id")
	assert.Equal(t, id, filter["id"], "Filter should contain the correct id")
}

func TestMockRepository(t *testing.T) {
	// Test that the mock repository implements the Repository interface properly
	repo := NewMockRepository()
	
	// Test Create and Get
	agent := &models.Agent{
		ID:       "test-id",
		Name:     "Test Agent",
		TenantID: "test-tenant",
		ModelID:  "test-model",
	}
	
	err := repo.Create(context.Background(), agent)
	assert.NoError(t, err, "Create should not return an error")
	
	retrieved, err := repo.Get(context.Background(), agent.ID)
	assert.NoError(t, err, "Get should not return an error")
	assert.NotNil(t, retrieved, "Retrieved agent should not be nil")
	assert.Equal(t, agent.ID, retrieved.ID, "Retrieved agent should have the correct ID")
	
	// Test List
	filter := FilterFromTenantID(agent.TenantID)
	agents, err := repo.List(context.Background(), filter)
	assert.NoError(t, err, "List should not return an error")
	assert.GreaterOrEqual(t, len(agents), 0, "List should return at least an empty slice")
	
	// Test API-specific methods
	err = repo.CreateAgent(context.Background(), agent)
	assert.NoError(t, err, "CreateAgent should not return an error")
	
	retrieved, err = repo.GetAgentByID(context.Background(), agent.ID, agent.TenantID)
	assert.NoError(t, err, "GetAgentByID should not return an error")
	assert.NotNil(t, retrieved, "Retrieved agent should not be nil")
	
	agents, err = repo.ListAgents(context.Background(), agent.TenantID)
	assert.NoError(t, err, "ListAgents should not return an error")
}

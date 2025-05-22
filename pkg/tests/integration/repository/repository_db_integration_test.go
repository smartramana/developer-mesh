package repository

import (
	"context"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
	"github.com/S-Corkum/devops-mcp/pkg/repository/agent"
	"github.com/S-Corkum/devops-mcp/pkg/repository/model"
	"github.com/S-Corkum/devops-mcp/pkg/tests/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryDatabaseIntegration(t *testing.T) {
	helper := integration.NewTestHelper(t)
	
	// Create observability components for tests
	logger := observability.NewLogger()
	
	// Create an in-memory test database
	db, err := database.NewTestDatabase()
	require.NoError(t, err)
	require.NotNil(t, db)
	
	// Ensure database is cleaned up after the test
	defer func() {
		err := db.Close()
		if err != nil {
			t.Logf("Error closing test database: %v", err)
		}
	}()
	
	t.Run("AgentRepository and ModelRepository provide data persistence", func(t *testing.T) {
		// Create repository factory 
		factory := repository.NewRepositoryFactory(db, logger)
		require.NotNil(t, factory)
		
		// Create repositories
		agentRepo, err := factory.GetAgentRepository()
		require.NoError(t, err)
		require.NotNil(t, agentRepo)
		
		modelRepo, err := factory.GetModelRepository()
		require.NoError(t, err)
		require.NotNil(t, modelRepo)
		
		// Create test data
		ctx := context.Background()
		tenantID := "test-tenant"
		
		testModel := &models.Model{
			ID:             "model-1",
			TenantID:       tenantID,
			Name:           "Test Model",
			Provider:       "test-provider",
			ModelType:      "test-type",
			ContextLength:  1024,
			MaxTokens:      256,
			Temperature:    0.7,
			TopP:           1.0,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			Active:         true,
			ProviderModel:  "test-model",
			ProviderConfig: map[string]interface{}{"key": "value"},
		}
		
		testAgent := &models.Agent{
			ID:        "agent-1",
			TenantID:  tenantID,
			Name:      "Test Agent",
			ModelID:   testModel.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			SystemPrompt: "You are a helpful agent",
		}
		
		// Test cross-repository operations with proper transaction management
		// 1. First add a model, which agent depends on
		err = modelRepo.Create(ctx, testModel)
		require.NoError(t, err)
		
		// 2. Add an agent that references the model
		err = agentRepo.Create(ctx, testAgent)
		require.NoError(t, err)
		
		// 3. Verify model can be retrieved
		retrievedModel, err := modelRepo.Get(ctx, tenantID, testModel.ID)
		require.NoError(t, err)
		assert.Equal(t, testModel.ID, retrievedModel.ID)
		assert.Equal(t, testModel.Name, retrievedModel.Name)
		assert.Equal(t, testModel.Provider, retrievedModel.Provider)
		
		// 4. Verify agent can be retrieved and has correct model reference
		retrievedAgent, err := agentRepo.Get(ctx, tenantID, testAgent.ID)
		require.NoError(t, err)
		assert.Equal(t, testAgent.ID, retrievedAgent.ID)
		assert.Equal(t, testAgent.Name, retrievedAgent.Name)
		assert.Equal(t, testModel.ID, retrievedAgent.ModelID) // Verify model reference
		
		// 5. Test model update propagates properly
		testModel.Name = "Updated Model Name"
		err = modelRepo.Update(ctx, testModel)
		require.NoError(t, err)
		
		updatedModel, err := modelRepo.Get(ctx, tenantID, testModel.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Model Name", updatedModel.Name)
		
		// 6. Test filters work correctly
		filter := model.NewFilter().WithTenantID(tenantID)
		models, err := modelRepo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, models, 1)
		
		agentFilter := agent.NewFilter().WithTenantID(tenantID)
		agents, err := agentRepo.List(ctx, agentFilter)
		require.NoError(t, err)
		assert.Len(t, agents, 1)
		
		// 7. Test delete operations
		err = agentRepo.Delete(ctx, tenantID, testAgent.ID)
		require.NoError(t, err)
		
		// Verify agent is gone
		_, err = agentRepo.Get(ctx, tenantID, testAgent.ID)
		assert.Error(t, err)
		
		// Delete model
		err = modelRepo.Delete(ctx, tenantID, testModel.ID)
		require.NoError(t, err)
		
		// Verify model is gone
		_, err = modelRepo.Get(ctx, tenantID, testModel.ID)
		assert.Error(t, err)
	})
}

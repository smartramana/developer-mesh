package adapters

import (
	"context"
	"testing"

	"github.com/S-Corkum/devops-mcp/apps/rest-api/internal/repository"

	"github.com/stretchr/testify/assert"
)

func TestMockVectorRepository_DeleteModelEmbeddings(t *testing.T) {
	// Create a new mock repository
	repo := &MockVectorRepository{
		embeddings: make(map[string][]*repository.Embedding),
	}

	// Create test context
	ctx := context.Background()
	contextID := "test-context"
	modelID1 := "model1"
	modelID2 := "model2"

	// Add some test data
	repo.embeddings[contextID] = []*repository.Embedding{
		{
			ID:        "1",
			ContextID: contextID,
			ModelID:   modelID1,
			Text:      "Test text 1",
			Embedding: []float32{0.1, 0.2, 0.3},
		},
		{
			ID:        "2",
			ContextID: contextID,
			ModelID:   modelID1,
			Text:      "Test text 2",
			Embedding: []float32{0.4, 0.5, 0.6},
		},
		{
			ID:        "3",
			ContextID: contextID,
			ModelID:   modelID2,
			Text:      "Test text 3",
			Embedding: []float32{0.7, 0.8, 0.9},
		},
	}

	// Test initial state
	allEmbeddings, err := repo.GetContextEmbeddings(ctx, contextID)
	assert.NoError(t, err)
	assert.Len(t, allEmbeddings, 3)

	model1Embeddings, err := repo.GetEmbeddingsByModel(ctx, contextID, modelID1)
	assert.NoError(t, err)
	assert.Len(t, model1Embeddings, 2)

	model2Embeddings, err := repo.GetEmbeddingsByModel(ctx, contextID, modelID2)
	assert.NoError(t, err)
	assert.Len(t, model2Embeddings, 1)

	// Execute the method under test - delete model1 embeddings
	err = repo.DeleteModelEmbeddings(ctx, contextID, modelID1)
	assert.NoError(t, err)

	// Verify that only model1 embeddings were deleted
	allEmbeddings, err = repo.GetContextEmbeddings(ctx, contextID)
	assert.NoError(t, err)
	assert.Len(t, allEmbeddings, 1) // Only model2 embedding should remain

	model1Embeddings, err = repo.GetEmbeddingsByModel(ctx, contextID, modelID1)
	assert.NoError(t, err)
	assert.Len(t, model1Embeddings, 0) // Should be empty now

	model2Embeddings, err = repo.GetEmbeddingsByModel(ctx, contextID, modelID2)
	assert.NoError(t, err)
	assert.Len(t, model2Embeddings, 1) // Should still have one embedding

	// Test deleting from non-existent context (should not error)
	err = repo.DeleteModelEmbeddings(ctx, "non-existent-context", modelID1)
	assert.NoError(t, err)

	// Test deleting non-existent model (should not error but not change anything)
	err = repo.DeleteModelEmbeddings(ctx, contextID, "non-existent-model")
	assert.NoError(t, err)

	allEmbeddings, err = repo.GetContextEmbeddings(ctx, contextID)
	assert.NoError(t, err)
	assert.Len(t, allEmbeddings, 1) // Still should have the model2 embedding
}

func TestPkgVectorAPIAdapter_DeleteModelEmbeddings(t *testing.T) {
	// Create a mock internal repository
	mockInternalRepo := &MockVectorRepository{
		embeddings: make(map[string][]*repository.Embedding),
	}

	// Create the adapter with the mock internal repo
	adapter := &PkgVectorAPIAdapter{
		internal: mockInternalRepo,
	}

	// Create test context
	ctx := context.Background()
	contextID := "test-context"
	modelID := "model1"

	// Add some test data to the mock internal repo
	mockInternalRepo.embeddings[contextID] = []*repository.Embedding{
		{
			ID:        "1",
			ContextID: contextID,
			ModelID:   modelID,
			Text:      "Test text 1",
			Embedding: []float32{0.1, 0.2, 0.3},
		},
	}

	// Test initial state
	allEmbeddings, err := mockInternalRepo.GetContextEmbeddings(ctx, contextID)
	assert.NoError(t, err)
	assert.Len(t, allEmbeddings, 1)

	// Execute the method under test - delete through the adapter
	err = adapter.DeleteModelEmbeddings(ctx, contextID, modelID)
	assert.NoError(t, err)

	// Verify that embeddings were deleted in the internal repo
	allEmbeddings, err = mockInternalRepo.GetContextEmbeddings(ctx, contextID)
	assert.NoError(t, err)
	assert.Len(t, allEmbeddings, 0) // Should be empty after deletion
}

// Note: makeEmbeddingCopy function is already defined in vector_adapter.go

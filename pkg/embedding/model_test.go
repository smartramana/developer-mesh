package embedding_test

import (
	"testing"

	"github.com/S-Corkum/devops-mcp/internal/embedding"
	"github.com/stretchr/testify/assert"
)

// TestModelTypes verifies that the model type constants are defined correctly
func TestModelTypes(t *testing.T) {
	assert.Equal(t, embedding.ModelType("openai"), embedding.ModelTypeOpenAI)
	assert.Equal(t, embedding.ModelType("huggingface"), embedding.ModelTypeHuggingFace)
	assert.Equal(t, embedding.ModelType("custom"), embedding.ModelTypeCustom)
}

// TestOpenAIEmbeddingService verifies that the OpenAI service can be created
func TestOpenAIEmbeddingService(t *testing.T) {
	// Create a test service
	service, err := embedding.NewOpenAIEmbeddingService(
		"test-api-key", 
		"text-embedding-3-small",
		1536,
	)
	
	// Verify service creation
	assert.NoError(t, err)
	assert.NotNil(t, service)
	
	// Check model configuration
	config := service.GetModelConfig()
	assert.Equal(t, embedding.ModelTypeOpenAI, config.Type)
	assert.Equal(t, "text-embedding-3-small", config.Name)
	assert.Equal(t, 1536, config.Dimensions)
}

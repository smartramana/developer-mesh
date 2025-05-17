package embedding

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBedrockEmbeddingService(t *testing.T) {
	// Test with valid parameters
	config := &BedrockConfig{
		Region:  "us-west-2",
		ModelID: "amazon.titan-embed-text-v1",
	}
	service, err := NewBedrockEmbeddingService(config)
	assert.NoError(t, err)
	assert.NotNil(t, service)
	modelConfig := service.GetModelConfig()
	assert.Equal(t, "amazon.titan-embed-text-v1", modelConfig.Name)
	assert.Equal(t, ModelTypeBedrock, modelConfig.Type)
	assert.Equal(t, 1536, service.GetModelDimensions())

	// Test with a different valid model
	config = &BedrockConfig{
		Region:  "us-west-2",
		ModelID: "cohere.embed-english-v3",
	}
	service, err = NewBedrockEmbeddingService(config)
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, 1024, service.GetModelDimensions())

	// Test with invalid model
	config = &BedrockConfig{
		Region:  "us-west-2",
		ModelID: "invalid-model",
	}
	service, err = NewBedrockEmbeddingService(config)
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "unsupported AWS Bedrock model")

	// Test with empty region
	config = &BedrockConfig{
		Region:  "",
		ModelID: "amazon.titan-embed-text-v1",
	}
	service, err = NewBedrockEmbeddingService(config)
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "AWS region is required")

	// Test with nil config
	service, err = NewBedrockEmbeddingService(nil)
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "config is required")
}

func TestBedrockEmbeddingService_GenerateEmbedding(t *testing.T) {
	// Create a service with valid config
	config := &BedrockConfig{
		Region:  "us-west-2",
		ModelID: "amazon.titan-embed-text-v1",
	}
	service, err := NewBedrockEmbeddingService(config)
	assert.NoError(t, err)

	// Test generating an embedding
	embedding, err := service.GenerateEmbedding(context.Background(), "This is a test", "text", "test-id-1")
	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Equal(t, "test-id-1", embedding.ContentID)
	assert.Equal(t, "text", embedding.ContentType)
	assert.Equal(t, "amazon.titan-embed-text-v1", embedding.ModelID)
	assert.Equal(t, 1536, embedding.Dimensions)
	assert.Len(t, embedding.Vector, 1536)

	// Test with empty content
	embedding, err = service.GenerateEmbedding(context.Background(), "", "text", "test-id-2")
	assert.Error(t, err)
	assert.Nil(t, embedding)
	assert.Contains(t, err.Error(), "content cannot be empty")
}

func TestBedrockEmbeddingService_BatchGenerateEmbeddings(t *testing.T) {
	// Create a service with valid config
	config := &BedrockConfig{
		Region:  "us-west-2",
		ModelID: "amazon.titan-embed-text-v1",
	}
	service, err := NewBedrockEmbeddingService(config)
	assert.NoError(t, err)

	// Test batch generation with multiple texts
	texts := []string{"Text 1", "Text 2", "Text 3"}
	contentIDs := []string{"id-1", "id-2", "id-3"}
	embeddings, err := service.BatchGenerateEmbeddings(context.Background(), texts, "text", contentIDs)
	assert.NoError(t, err)
	assert.Len(t, embeddings, 3)
	
	for i, embedding := range embeddings {
		assert.Equal(t, contentIDs[i], embedding.ContentID)
		assert.Equal(t, "text", embedding.ContentType)
		assert.Equal(t, "amazon.titan-embed-text-v1", embedding.ModelID)
		assert.Equal(t, 1536, embedding.Dimensions)
		assert.Len(t, embedding.Vector, 1536)
	}

	// Test with empty texts
	embeddings, err = service.BatchGenerateEmbeddings(context.Background(), []string{}, "text", []string{})
	assert.Error(t, err)
	assert.Nil(t, embeddings)
	assert.Contains(t, err.Error(), "no texts provided")

	// Test with mismatched lengths
	embeddings, err = service.BatchGenerateEmbeddings(context.Background(), texts, "text", []string{"id-1"})
	assert.Error(t, err)
	assert.Nil(t, embeddings)
	assert.Contains(t, err.Error(), "number of texts must match")
}

func TestBedrockEmbeddingService_BatchProcessing(t *testing.T) {
	// Create a service with valid config
	config := &BedrockConfig{
		Region:  "us-west-2",
		ModelID: "amazon.titan-embed-text-v1",
	}
	service, err := NewBedrockEmbeddingService(config)
	assert.NoError(t, err)

	// Create a batch larger than the max batch size
	batchSize := maxBedrockBatchSize + 5
	texts := make([]string, batchSize)
	contentIDs := make([]string, batchSize)
	
	for i := 0; i < batchSize; i++ {
		texts[i] = fmt.Sprintf("Text %d", i)
		contentIDs[i] = fmt.Sprintf("id-%d", i)
	}
	
	// Process the large batch
	embeddings, err := service.BatchGenerateEmbeddings(context.Background(), texts, "text", contentIDs)
	assert.NoError(t, err)
	assert.Len(t, embeddings, batchSize)
	
	// Verify all embeddings were created
	for i, embedding := range embeddings {
		assert.Equal(t, contentIDs[i], embedding.ContentID)
	}
}

func TestNewLatestAnthropicModels(t *testing.T) {
	// Test Claude 3.5 Haiku
	service, err := NewMockBedrockEmbeddingService("anthropic.claude-3-5-haiku-20250531-v1:0")
	assert.NoError(t, err)
	assert.NotNil(t, service)
	
	// Check that model config is set properly
	config := service.GetModelConfig()
	assert.Equal(t, ModelTypeBedrock, config.Type)
	assert.Equal(t, "anthropic.claude-3-5-haiku-20250531-v1:0", config.Name)
	assert.Equal(t, 4096, service.GetModelDimensions())
	
	// Test embedding generation
	embedding, err := service.GenerateEmbedding(context.Background(), "Test content for Claude 3.5", "text", "claude-3-5-test")
	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Equal(t, "claude-3-5-test", embedding.ContentID)
	assert.Equal(t, "text", embedding.ContentType)
	assert.Equal(t, "anthropic.claude-3-5-haiku-20250531-v1:0", embedding.ModelID)
	assert.Equal(t, 4096, embedding.Dimensions)
	assert.Len(t, embedding.Vector, 4096)
	
	// Test Claude 3.7 Sonnet
	service, err = NewMockBedrockEmbeddingService("anthropic.claude-3-7-sonnet-20250531-v1:0")
	assert.NoError(t, err)
	assert.NotNil(t, service)
	
	// Check that model config is set properly
	config = service.GetModelConfig()
	assert.Equal(t, ModelTypeBedrock, config.Type)
	assert.Equal(t, "anthropic.claude-3-7-sonnet-20250531-v1:0", config.Name)
	assert.Equal(t, 8192, service.GetModelDimensions())
	
	// Test embedding generation
	embedding, err = service.GenerateEmbedding(context.Background(), "Test content for Claude 3.7", "text", "claude-3-7-test")
	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Equal(t, "claude-3-7-test", embedding.ContentID)
	assert.Equal(t, "text", embedding.ContentType)
	assert.Equal(t, "anthropic.claude-3-7-sonnet-20250531-v1:0", embedding.ModelID)
	assert.Equal(t, 8192, embedding.Dimensions)
	assert.Len(t, embedding.Vector, 8192)
}

func TestNewMockBedrockEmbeddingService(t *testing.T) {
	// Test with valid model ID
	service, err := NewMockBedrockEmbeddingService("amazon.titan-embed-text-v1")
	assert.NoError(t, err)
	assert.NotNil(t, service)
	
	// Check that it's configured as a mock
	assert.True(t, service.useMockEmbeddings)
	
	// Check that model config is set properly
	config := service.GetModelConfig()
	assert.Equal(t, ModelTypeBedrock, config.Type)
	assert.Equal(t, "amazon.titan-embed-text-v1", config.Name)
	assert.Equal(t, 1536, service.GetModelDimensions())
	
	// Test generating embeddings with the mock service
	embedding, err := service.GenerateEmbedding(context.Background(), "Test content", "text", "test-id")
	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Equal(t, "test-id", embedding.ContentID)
	assert.Equal(t, "text", embedding.ContentType)
	assert.Equal(t, "amazon.titan-embed-text-v1", embedding.ModelID)
	assert.Equal(t, 1536, embedding.Dimensions)
	assert.Len(t, embedding.Vector, 1536)
	
	// Test with invalid model ID
	service, err = NewMockBedrockEmbeddingService("invalid-model")
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "unsupported AWS Bedrock model")
	
	// Test with default model ID (empty string)
	service, err = NewMockBedrockEmbeddingService("")
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, defaultBedrockModel, service.config.Name)
}

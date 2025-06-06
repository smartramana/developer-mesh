package embedding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCoreModelTypes tests the ModelType constants
func TestCoreModelTypes(t *testing.T) {
	assert.Equal(t, ModelType("openai"), ModelTypeOpenAI, "ModelTypeOpenAI should be 'openai'")
	assert.Equal(t, ModelType("huggingface"), ModelTypeHuggingFace, "ModelTypeHuggingFace should be 'huggingface'")
	assert.Equal(t, ModelType("custom"), ModelTypeCustom, "ModelTypeCustom should be 'custom'")
}

// TestCoreModelConfig tests the ModelConfig structure
func TestCoreModelConfig(t *testing.T) {
	// Create a test config
	config := ModelConfig{
		Type:       ModelTypeOpenAI,
		Name:       "text-embedding-3-small",
		APIKey:     "test-api-key",
		Endpoint:   "https://api.openai.com/v1/embeddings",
		Dimensions: 1536,
		Parameters: map[string]interface{}{
			"param1": "value1",
		},
	}

	// Verify fields
	assert.Equal(t, ModelTypeOpenAI, config.Type, "config.Type should match")
	assert.Equal(t, "text-embedding-3-small", config.Name, "config.Name should match")
	assert.Equal(t, "test-api-key", config.APIKey, "config.APIKey should match")
	assert.Equal(t, "https://api.openai.com/v1/embeddings", config.Endpoint, "config.Endpoint should match")
	assert.Equal(t, 1536, config.Dimensions, "config.Dimensions should match")
	assert.Equal(t, "value1", config.Parameters["param1"], "config.Parameters['param1'] should match")
}

// TestCoreEmbeddingVector tests the EmbeddingVector structure
func TestCoreEmbeddingVector(t *testing.T) {
	// Create a test vector
	vector := EmbeddingVector{
		Vector:      []float32{0.1, 0.2, 0.3},
		Dimensions:  3,
		ModelID:     "test-model",
		ContentType: "test-type",
		ContentID:   "test-id",
		Metadata: map[string]interface{}{
			"key1": "value1",
		},
	}

	// Verify fields
	assert.Len(t, vector.Vector, 3, "vector.Vector should have length 3")
	assert.Equal(t, float32(0.1), vector.Vector[0], "vector.Vector[0] should be 0.1")
	assert.Equal(t, float32(0.2), vector.Vector[1], "vector.Vector[1] should be 0.2")
	assert.Equal(t, float32(0.3), vector.Vector[2], "vector.Vector[2] should be 0.3")
	assert.Equal(t, 3, vector.Dimensions, "vector.Dimensions should be 3")
	assert.Equal(t, "test-model", vector.ModelID, "vector.ModelID should match")
	assert.Equal(t, "test-type", vector.ContentType, "vector.ContentType should match")
	assert.Equal(t, "test-id", vector.ContentID, "vector.ContentID should match")
	assert.Equal(t, "value1", vector.Metadata["key1"], "vector.Metadata['key1'] should match")
}

// TestCoreInterfaceCompatibility ensures that interface implementations match requirements
func TestCoreInterfaceCompatibility(t *testing.T) {
	// These aren't functional tests, but they verify at compile time that the interfaces match

	// Create a variable of interface type and assign it nil to verify the interface
	// is implemented by the type
	var embeddingService EmbeddingService = (*OpenAIEmbeddingService)(nil)

	// This doesn't actually run as a test, it just verifies that the interfaces match
	// at compile time
	assert.Nil(t, embeddingService, "This should compile if OpenAIEmbeddingService implements EmbeddingService")
}

// TestEmbeddingServiceMethods uses a mock to verify service interface behavior
func TestEmbeddingServiceMethods(t *testing.T) {
	// Create a mock embedding service for testing
	mockService := &MockEmbeddingServiceForTests{
		MockVectors: make(map[string]*EmbeddingVector),
	}

	// Setup test data
	ctx := context.Background()
	testText := "This is a test text"
	contentType := "test"
	contentID := "test-123"

	// Set the mock to return a test vector
	testVector := &EmbeddingVector{
		Vector:      []float32{0.1, 0.2, 0.3},
		Dimensions:  3,
		ModelID:     "test-model",
		ContentType: contentType,
		ContentID:   contentID,
		Metadata:    make(map[string]interface{}),
	}
	mockService.MockVectors[testText] = testVector

	// Test GenerateEmbedding
	result, err := mockService.GenerateEmbedding(ctx, testText, contentType, contentID)
	assert.NoError(t, err, "GenerateEmbedding should not return an error")
	assert.Equal(t, testVector, result, "Generated embedding should match expected")

	// Test BatchGenerateEmbeddings
	texts := []string{testText, "Another test"}
	contentIDs := []string{contentID, "test-456"}

	// Add another mock vector
	anotherVector := &EmbeddingVector{
		Vector:      []float32{0.4, 0.5, 0.6},
		Dimensions:  3,
		ModelID:     "test-model",
		ContentType: contentType,
		ContentID:   "test-456",
		Metadata:    make(map[string]interface{}),
	}
	mockService.MockVectors["Another test"] = anotherVector

	// Test batch generation
	batchResults, err := mockService.BatchGenerateEmbeddings(ctx, texts, contentType, contentIDs)
	assert.NoError(t, err, "BatchGenerateEmbeddings should not return an error")
	assert.Len(t, batchResults, 2, "Batch should return 2 embeddings")
	assert.Equal(t, testVector, batchResults[0], "First embedding should match")
	assert.Equal(t, anotherVector, batchResults[1], "Second embedding should match")

	// Test model configuration
	config := mockService.GetModelConfig()
	assert.Equal(t, ModelTypeOpenAI, config.Type, "Model type should match")
	assert.Equal(t, "test-model", config.Name, "Model name should match")
	assert.Equal(t, 3, config.Dimensions, "Dimensions should match")

	// Test dimensions method
	dims := mockService.GetModelDimensions()
	assert.Equal(t, 3, dims, "Dimensions should match")
}

// MockEmbeddingServiceForTests is a simple implementation of EmbeddingService for testing
type MockEmbeddingServiceForTests struct {
	MockVectors map[string]*EmbeddingVector
}

// GenerateEmbedding for the mock service
func (m *MockEmbeddingServiceForTests) GenerateEmbedding(ctx context.Context, text string, contentType string, contentID string) (*EmbeddingVector, error) {
	if vector, ok := m.MockVectors[text]; ok {
		return vector, nil
	}

	// Return a default vector if not found in mock data
	return &EmbeddingVector{
		Vector:      []float32{0.0, 0.0, 0.0},
		Dimensions:  3,
		ModelID:     "test-model",
		ContentType: contentType,
		ContentID:   contentID,
		Metadata:    make(map[string]interface{}),
	}, nil
}

// BatchGenerateEmbeddings for the mock service
func (m *MockEmbeddingServiceForTests) BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error) {
	results := make([]*EmbeddingVector, len(texts))

	for i, text := range texts {
		var contentID string
		if i < len(contentIDs) {
			contentID = contentIDs[i]
		} else {
			contentID = "unknown"
		}

		vector, _ := m.GenerateEmbedding(ctx, text, contentType, contentID)
		results[i] = vector
	}

	return results, nil
}

// GetModelConfig for the mock service
func (m *MockEmbeddingServiceForTests) GetModelConfig() ModelConfig {
	return ModelConfig{
		Type:       ModelTypeOpenAI,
		Name:       "test-model",
		Dimensions: 3,
	}
}

// GetModelDimensions for the mock service
func (m *MockEmbeddingServiceForTests) GetModelDimensions() int {
	return 3
}

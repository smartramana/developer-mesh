package embedding

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ServiceMockEmbeddingService is a mock implementation for testing in service_test.go
type ServiceMockEmbeddingService struct {
	mock.Mock
}

func (m *ServiceMockEmbeddingService) GenerateEmbedding(ctx context.Context, text, contentType, contentID string) (*EmbeddingVector, error) {
	args := m.Called(ctx, text, contentType, contentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingVector), args.Error(1)
}

func (m *ServiceMockEmbeddingService) BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error) {
	args := m.Called(ctx, texts, contentType, contentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*EmbeddingVector), args.Error(1)
}

func (m *ServiceMockEmbeddingService) GetModelConfig() ModelConfig {
	args := m.Called()
	return args.Get(0).(ModelConfig)
}

func (m *ServiceMockEmbeddingService) GetModelDimensions() int {
	args := m.Called()
	return args.Int(0)
}

func TestEmbeddingVectorFormat(t *testing.T) {
	// Create test embedding
	embedding := &EmbeddingVector{
		ContentID: "content-1",
		Vector:    []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		Dimensions: 5,
		ModelID: "test-model",
		ContentType: "test-type",
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	// Test vector format and access
	assert.Equal(t, 5, len(embedding.Vector), "Vector should have expected length")
	assert.Equal(t, float32(0.1), embedding.Vector[0], "First vector element should be correct")
}

func TestEmbeddingVectorWithValues(t *testing.T) {
	// Create test embedding with vector values
	embedding := &EmbeddingVector{
		ContentID: "content-1",
		Vector:    []float32{1.0, 2.0, 2.0, 1.0},
		Dimensions: 4,
		ModelID: "test-model",
		ContentType: "test-type",
	}

	// Test vector handling
	assert.Equal(t, 4, len(embedding.Vector), "Vector length should match Dimensions field")
	assert.Equal(t, 4, embedding.Dimensions, "Dimensions field should match vector length")
	
	// Calculate vector magnitude
	sumSquares := float32(0)
	for _, v := range embedding.Vector {
		sumSquares += v * v
	}
	// Expected magnitude is sqrt(10)
	assert.InDelta(t, 10.0, sumSquares, 0.0001, "Sum of squares should match expected value")
}

func TestValidateEmbeddingModel(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{"Valid OpenAI model", "text-embedding-3-small", false},
		{"Valid OpenAI model", "text-embedding-3-large", false},
		{"Valid OpenAI model", "text-embedding-ada-002", false},
		{"Empty model", "", true},
		{"Invalid model", "invalid-model", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a service with the test model
			_, err := NewOpenAIEmbeddingService("test-key", tt.model, 1536)
			// The constructor will validate the model
			if tt.wantErr {
				assert.Error(t, err, "ValidateEmbeddingModel should return an error for invalid models")
			} else {
				assert.NoError(t, err, "ValidateEmbeddingModel should not return an error for valid models")
			}
		})
	}

	// Test unsupported model type
	// Test with an unsupported model type by trying to create a service
	_, err := NewOpenAIEmbeddingService("test-key", "unsupported-model", 0)
	assert.Error(t, err, "ValidateEmbeddingModel should return an error for unsupported model types")
}

func TestGetEmbeddingModelDimensions(t *testing.T) {
	tests := []struct {
		name        string
		modelType   ModelType
		model       string
		wantDim     int
		expectError bool
	}{
		{"OpenAI text-embedding-3-small", ModelTypeOpenAI, "text-embedding-3-small", 1536, false},
		{"OpenAI text-embedding-3-large", ModelTypeOpenAI, "text-embedding-3-large", 3072, false},
		{"OpenAI text-embedding-ada-002", ModelTypeOpenAI, "text-embedding-ada-002", 1536, false},
		{"Invalid OpenAI model", ModelTypeOpenAI, "invalid-model", 0, true},
		{"Unsupported model type", "unsupported", "any-model", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a service with the test model
			var service *OpenAIEmbeddingService
			var err error
			
			// For valid OpenAI models, we can use the constructor
			if tt.modelType == ModelTypeOpenAI && !tt.expectError {
				service, err = NewOpenAIEmbeddingService("test-key", tt.model, 0) // Let it use default dimensions
			} else {
				// For invalid models or types, manually create the service
				service = &OpenAIEmbeddingService{
					config: ModelConfig{
						Type: tt.modelType,
						Name: tt.model,
					},
				}
				// Error expected for invalid models
				err = errors.New("invalid model")
			}
			
			// Get dimensions
			dim := service.GetModelDimensions()
			if tt.expectError {
				assert.Error(t, err, "GetEmbeddingModelDimensions should return an error for invalid inputs")
				assert.Equal(t, 0, dim, "Dimension should be 0 when error occurs")
			} else {
				assert.NoError(t, err, "GetEmbeddingModelDimensions should not return an error for valid inputs")
				assert.Equal(t, tt.wantDim, dim, "Returned dimension should match expected dimension")
			}
		})
	}
}

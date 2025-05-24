package embedding

import (
	"context"
	"fmt"
)

// MockEmbeddingService provides a mock implementation of EmbeddingService for testing
type MockEmbeddingService struct {
	dimensions int
	vectors    map[string]*EmbeddingVector
}

// NewMockEmbeddingService creates a new mock embedding service with the specified dimensions
func NewMockEmbeddingService(dimensions int) EmbeddingService {
	return &MockEmbeddingService{
		dimensions: dimensions,
		vectors:    make(map[string]*EmbeddingVector),
	}
}

// GenerateEmbedding generates a mock embedding vector
func (m *MockEmbeddingService) GenerateEmbedding(ctx context.Context, text string, contentType string, contentID string) (*EmbeddingVector, error) {
	// Check if we have a pre-configured vector for this text
	if vector, ok := m.vectors[text]; ok {
		return vector, nil
	}
	
	// Generate a deterministic mock vector based on the text
	vector := make([]float32, m.dimensions)
	for i := 0; i < m.dimensions; i++ {
		// Create a deterministic value based on text and position
		hash := 0
		for _, ch := range text {
			hash = (hash*31 + int(ch)) % 1000
		}
		vector[i] = float32(hash+i) / float32(1000+m.dimensions)
	}
	
	return &EmbeddingVector{
		Vector:      vector,
		Dimensions:  m.dimensions,
		ModelID:     "mock-model",
		ContentType: contentType,
		ContentID:   contentID,
		Metadata: map[string]interface{}{
			"mock": true,
			"text": text,
		},
	}, nil
}

// BatchGenerateEmbeddings generates mock embeddings for multiple texts
func (m *MockEmbeddingService) BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error) {
	if len(texts) != len(contentIDs) && len(contentIDs) > 0 {
		return nil, fmt.Errorf("texts and contentIDs must have the same length")
	}
	
	results := make([]*EmbeddingVector, len(texts))
	for i, text := range texts {
		contentID := fmt.Sprintf("mock-%d", i)
		if i < len(contentIDs) {
			contentID = contentIDs[i]
		}
		
		vector, err := m.GenerateEmbedding(ctx, text, contentType, contentID)
		if err != nil {
			return nil, err
		}
		results[i] = vector
	}
	
	return results, nil
}

// GetModelConfig returns the mock model configuration
func (m *MockEmbeddingService) GetModelConfig() ModelConfig {
	return ModelConfig{
		Type:       ModelTypeCustom,
		Name:       "mock-model",
		Dimensions: m.dimensions,
		Parameters: map[string]interface{}{
			"mock": true,
		},
	}
}

// GetModelDimensions returns the configured dimensions
func (m *MockEmbeddingService) GetModelDimensions() int {
	return m.dimensions
}

// SetMockVector allows setting a specific vector for a given text (useful for testing)
func (m *MockEmbeddingService) SetMockVector(text string, vector *EmbeddingVector) {
	m.vectors[text] = vector
}
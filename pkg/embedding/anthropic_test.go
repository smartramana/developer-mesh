package embedding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAnthropicEmbeddingService(t *testing.T) {
	// Test with valid config
	config := &AnthropicConfig{
		APIKey: "test-api-key",
		Model:  "claude-3-5-haiku-20250531",
	}
	service, err := NewAnthropicEmbeddingService(config)
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, ModelTypeAnthropic, service.config.Type)
	assert.Equal(t, "claude-3-5-haiku-20250531", service.config.Name)
	assert.Equal(t, 4096, service.config.Dimensions)

	// Test without API key
	config = &AnthropicConfig{
		Model: "claude-3-5-haiku-20250531",
	}
	service, err = NewAnthropicEmbeddingService(config)
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "API key is required")

	// Test with invalid model
	config = &AnthropicConfig{
		APIKey: "test-api-key",
		Model:  "invalid-model",
	}
	service, err = NewAnthropicEmbeddingService(config)
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "unsupported Anthropic model")

	// Test with default model (empty model name)
	config = &AnthropicConfig{
		APIKey: "test-api-key",
		Model:  "",
	}
	service, err = NewAnthropicEmbeddingService(config)
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, defaultAnthropicModel, service.config.Name)
}

func TestNewMockAnthropicEmbeddingService(t *testing.T) {
	// Test with valid model
	service, err := NewMockAnthropicEmbeddingService("claude-3-5-haiku-20250531")
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, ModelTypeAnthropic, service.config.Type)
	assert.Equal(t, "claude-3-5-haiku-20250531", service.config.Name)
	assert.Equal(t, 4096, service.config.Dimensions)
	assert.True(t, service.useMockEmbeddings)

	// Test with invalid model
	service, err = NewMockAnthropicEmbeddingService("invalid-model")
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "unsupported Anthropic model")

	// Test with default model (empty model name)
	service, err = NewMockAnthropicEmbeddingService("")
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, defaultAnthropicModel, service.config.Name)
}

func TestAnthropicEmbeddingService_GenerateEmbedding(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		assert.Equal(t, http.MethodPost, r.Method)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"object": "embedding",
			"embedding": [0.1, 0.2, 0.3, 0.4, 0.5],
			"model": "claude-3-5-haiku-20250531"
		}`))
	}))
	defer server.Close()

	// Create a service with the mock server
	config := &AnthropicConfig{
		APIKey:   "test-api-key",
		Model:    "claude-3-5-haiku-20250531",
		Endpoint: server.URL,
	}
	service, err := NewAnthropicEmbeddingService(config)
	assert.NoError(t, err)

	// Test generating an embedding
	embedding, err := service.GenerateEmbedding(context.Background(), "Test content", "text", "test-id")
	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Equal(t, "test-id", embedding.ContentID)
	assert.Equal(t, "text", embedding.ContentType)
	assert.Equal(t, "claude-3-5-haiku-20250531", embedding.ModelID)
	assert.Equal(t, 4096, embedding.Dimensions)
	assert.Len(t, embedding.Vector, 5) // Only 5 values in our mock response

	// Test with empty text
	embedding, err = service.GenerateEmbedding(context.Background(), "", "text", "test-id")
	assert.Error(t, err)
	assert.Nil(t, embedding)
	assert.Contains(t, err.Error(), "text is required")
}

func TestAnthropicEmbeddingService_BatchGenerateEmbeddings(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		assert.Equal(t, http.MethodPost, r.Method)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"object": "embedding_batch",
			"embeddings": [
				[0.1, 0.2, 0.3, 0.4, 0.5],
				[0.6, 0.7, 0.8, 0.9, 1.0]
			],
			"model": "claude-3-5-haiku-20250531"
		}`))
	}))
	defer server.Close()

	// Create a service with the mock server
	config := &AnthropicConfig{
		APIKey:   "test-api-key",
		Model:    "claude-3-5-haiku-20250531",
		Endpoint: server.URL,
	}
	service, err := NewAnthropicEmbeddingService(config)
	assert.NoError(t, err)

	// Test generating embeddings for multiple texts
	texts := []string{"Text 1", "Text 2"}
	contentIDs := []string{"id-1", "id-2"}
	embeddings, err := service.BatchGenerateEmbeddings(context.Background(), texts, "text", contentIDs)
	assert.NoError(t, err)
	assert.NotNil(t, embeddings)
	assert.Len(t, embeddings, 2)

	// Check first embedding
	assert.Equal(t, "id-1", embeddings[0].ContentID)
	assert.Equal(t, "text", embeddings[0].ContentType)
	assert.Equal(t, "claude-3-5-haiku-20250531", embeddings[0].ModelID)
	assert.Equal(t, 4096, embeddings[0].Dimensions)
	assert.Len(t, embeddings[0].Vector, 5) // Only 5 values in our mock response

	// Check second embedding
	assert.Equal(t, "id-2", embeddings[1].ContentID)
	assert.Equal(t, "text", embeddings[1].ContentType)
	assert.Equal(t, "claude-3-5-haiku-20250531", embeddings[1].ModelID)
	assert.Equal(t, 4096, embeddings[1].Dimensions)
	assert.Len(t, embeddings[1].Vector, 5) // Only 5 values in our mock response

	// Test with empty texts
	embeddings, err = service.BatchGenerateEmbeddings(context.Background(), []string{}, "text", []string{})
	assert.Error(t, err)
	assert.Nil(t, embeddings)
	assert.Contains(t, err.Error(), "no texts provided")

	// Test with mismatched texts and content IDs
	embeddings, err = service.BatchGenerateEmbeddings(context.Background(), texts, "text", []string{"id-1"})
	assert.Error(t, err)
	assert.Nil(t, embeddings)
	assert.Contains(t, err.Error(), "number of texts must match")
}

func TestAnthropicEmbeddingService_MockEmbeddings(t *testing.T) {
	// Create a service with mock embeddings enabled
	config := &AnthropicConfig{
		APIKey:            "test-api-key",
		Model:             "claude-3-5-haiku-20250531",
		UseMockEmbeddings: true,
	}
	service, err := NewAnthropicEmbeddingService(config)
	assert.NoError(t, err)
	assert.True(t, service.useMockEmbeddings)

	// Test generating an embedding
	embedding, err := service.GenerateEmbedding(context.Background(), "Test content", "text", "test-id")
	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Equal(t, "test-id", embedding.ContentID)
	assert.Equal(t, "text", embedding.ContentType)
	assert.Equal(t, "claude-3-5-haiku-20250531", embedding.ModelID)
	assert.Equal(t, 4096, embedding.Dimensions)
	assert.Len(t, embedding.Vector, 4096)

	// Test batch generating embeddings
	texts := []string{"Text 1", "Text 2"}
	contentIDs := []string{"id-1", "id-2"}
	embeddings, err := service.BatchGenerateEmbeddings(context.Background(), texts, "text", contentIDs)
	assert.NoError(t, err)
	assert.NotNil(t, embeddings)
	assert.Len(t, embeddings, 2)
	assert.Equal(t, "id-1", embeddings[0].ContentID)
	assert.Equal(t, "id-2", embeddings[1].ContentID)
	assert.Len(t, embeddings[0].Vector, 4096)
	assert.Len(t, embeddings[1].Vector, 4096)
}

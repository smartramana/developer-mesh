package embedding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOpenAIEmbeddingService(t *testing.T) {
	// Test with valid parameters
	service, err := NewOpenAIEmbeddingService("test-api-key", "text-embedding-3-small", 1536)
	assert.NoError(t, err)
	assert.NotNil(t, service)
	config := service.GetModelConfig()
	assert.Equal(t, "text-embedding-3-small", config.Name)
	assert.Equal(t, 1536, service.GetModelDimensions())

	// Test with an invalid model
	service, err = NewOpenAIEmbeddingService("test-api-key", "invalid-model", 1536)
	assert.Error(t, err) // Model validation is now enforced at creation time
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "unsupported OpenAI model")

	// Test with empty API key
	service, err = NewOpenAIEmbeddingService("", "text-embedding-3-small", 1536)
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "API key is required")
}

func TestOpenAIEmbeddingService_GenerateEmbedding(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/embeddings", r.URL.Path)

		// Check Authorization header
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"object": "list",
			"data": [
				{
					"object": "embedding",
					"embedding": [0.1, 0.2, 0.3, 0.4, 0.5],
					"index": 0
				}
			],
			"model": "text-embedding-3-small",
			"usage": {
				"prompt_tokens": 5,
				"total_tokens": 5
			}
		}`))
	}))
	defer server.Close()

	// Create OpenAI service with custom base URL pointing to our test server
	service, err := NewOpenAIEmbeddingService("test-api-key", "text-embedding-3-small", 1536)
	assert.NoError(t, err)
	// Set the endpoint to point to our test server
	service.config.Endpoint = server.URL + "/v1/embeddings"
	assert.NotNil(t, service)

	// Test generating embedding
	embedding, err := service.GenerateEmbedding(context.Background(), "Test content", "test", "test-id")
	assert.NoError(t, err)
	assert.NotNil(t, embedding)
	assert.Equal(t, "test-id", embedding.ContentID)
	assert.Equal(t, "test", embedding.ContentType)
	assert.Equal(t, []float32{0.1, 0.2, 0.3, 0.4, 0.5}, embedding.Vector)
	assert.NotNil(t, embedding.Metadata)
}

func TestOpenAIEmbeddingService_ErrorHandling(t *testing.T) {
	// Create a mock HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{
			"error": {
				"message": "Test API error",
				"type": "invalid_request_error",
				"code": "test_error"
			}
		}`))
	}))
	defer server.Close()

	// Create OpenAI service with custom base URL pointing to our test server
	service, err := NewOpenAIEmbeddingService("test-api-key", "text-embedding-3-small", 1536)
	assert.NoError(t, err)
	// Set the endpoint to point to our test server
	service.config.Endpoint = server.URL + "/v1/embeddings"

	// Test handling API error
	embedding, err := service.GenerateEmbedding(context.Background(), "Test content", "test", "test-id")
	assert.Error(t, err)
	assert.Nil(t, embedding)
	assert.Contains(t, err.Error(), "OpenAI API error")
	assert.Contains(t, err.Error(), "Test API error")
}

func TestOpenAIEmbeddingService_EmptyContent(t *testing.T) {
	service, err := NewOpenAIEmbeddingService("test-api-key", "text-embedding-3-small", 1536)
	assert.NoError(t, err)

	// Test with empty content
	embedding, err := service.GenerateEmbedding(context.Background(), "", "test", "test-id")
	assert.Error(t, err)
	assert.Nil(t, embedding)
	assert.Contains(t, err.Error(), "content cannot be empty")
}

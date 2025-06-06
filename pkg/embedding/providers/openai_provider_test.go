package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAIProvider(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		config := ProviderConfig{
			APIKey: "test-api-key",
		}
		
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "openai", provider.Name())
		assert.Equal(t, "https://api.openai.com/v1", provider.config.Endpoint)
	})

	t.Run("missing API key", func(t *testing.T) {
		config := ProviderConfig{}
		
		_, err := NewOpenAIProvider(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("custom endpoint", func(t *testing.T) {
		config := ProviderConfig{
			APIKey:   "test-api-key",
			Endpoint: "https://custom.openai.com",
		}
		
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)
		assert.Equal(t, "https://custom.openai.com", provider.config.Endpoint)
	})
}

func TestOpenAIProvider_GenerateEmbedding(t *testing.T) {
	ctx := context.Background()

	t.Run("successful generation", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/embeddings", r.URL.Path)
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
			
			// Parse request body
			var req openAIRequest
			body, _ := io.ReadAll(r.Body)
			err := json.Unmarshal(body, &req)
			require.NoError(t, err)
			
			assert.Equal(t, "test content", req.Input)
			assert.Equal(t, "text-embedding-3-small", req.Model)
			
			// Send response
			resp := openAIResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{
					{
						Object:    "embedding",
						Embedding: generateTestEmbedding(1536),
						Index:     0,
					},
				},
				Model: "text-embedding-3-small",
				Usage: struct {
					PromptTokens int `json:"prompt_tokens"`
					TotalTokens  int `json:"total_tokens"`
				}{
					PromptTokens: 3,
					TotalTokens:  3,
				},
			}
			
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := ProviderConfig{
			APIKey:   "test-api-key",
			Endpoint: server.URL,
		}
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		req := GenerateEmbeddingRequest{
			Text:      "test content",
			Model:     "text-embedding-3-small",
			RequestID: "test-123",
		}

		resp, err := provider.GenerateEmbedding(ctx, req)
		require.NoError(t, err)
		
		assert.Equal(t, "text-embedding-3-small", resp.Model)
		assert.Equal(t, 1536, resp.Dimensions)
		assert.Len(t, resp.Embedding, 1536)
		assert.Equal(t, 3, resp.TokensUsed)
		assert.Equal(t, "openai", resp.ProviderInfo.Provider)
	})

	t.Run("with dimension reduction", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req openAIRequest
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &req)
			
			// Verify dimension parameter
			assert.NotNil(t, req.Dimensions)
			assert.Equal(t, 768, *req.Dimensions)
			
			resp := openAIResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{
					{
						Object:    "embedding",
						Embedding: generateTestEmbedding(768),
						Index:     0,
					},
				},
				Model: "text-embedding-3-small",
				Usage: struct {
					PromptTokens int `json:"prompt_tokens"`
					TotalTokens  int `json:"total_tokens"`
				}{
					PromptTokens: 3,
					TotalTokens:  3,
				},
			}
			
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := ProviderConfig{
			APIKey:   "test-api-key",
			Endpoint: server.URL,
		}
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		req := GenerateEmbeddingRequest{
			Text:  "test content",
			Model: "text-embedding-3-small",
			Metadata: map[string]interface{}{
				"dimensions": 768,
			},
		}

		resp, err := provider.GenerateEmbedding(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 768, resp.Dimensions)
		assert.Len(t, resp.Embedding, 768)
	})

	t.Run("API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			errResp := openAIErrorResponse{
				Error: struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Param   string `json:"param,omitempty"`
					Code    string `json:"code,omitempty"`
				}{
					Message: "Invalid API key",
					Type:    "invalid_request_error",
					Code:    "invalid_api_key",
				},
			}
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(errResp)
		}))
		defer server.Close()

		config := ProviderConfig{
			APIKey:   "invalid-key",
			Endpoint: server.URL,
		}
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		req := GenerateEmbeddingRequest{
			Text:  "test",
			Model: "text-embedding-3-small",
		}

		_, err = provider.GenerateEmbedding(ctx, req)
		require.Error(t, err)
		
		provErr, ok := err.(*ProviderError)
		require.True(t, ok)
		assert.Equal(t, "openai", provErr.Provider)
		assert.Equal(t, "invalid_api_key", provErr.Code)
		assert.Equal(t, 401, provErr.StatusCode)
		assert.False(t, provErr.IsRetryable)
	})

	t.Run("rate limit with retry", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			
			if attempts < 2 {
				// First attempt fails with rate limit
				errResp := openAIErrorResponse{
					Error: struct {
						Message string `json:"message"`
						Type    string `json:"type"`
						Param   string `json:"param,omitempty"`
						Code    string `json:"code,omitempty"`
					}{
						Message: "Rate limit exceeded",
						Type:    "rate_limit_error",
						Code:    "rate_limit_exceeded",
					},
				}
				
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(errResp)
				return
			}
			
			// Second attempt succeeds
			resp := openAIResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{
					{
						Object:    "embedding",
						Embedding: generateTestEmbedding(1536),
						Index:     0,
					},
				},
				Model: "text-embedding-3-small",
			}
			
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := ProviderConfig{
			APIKey:         "test-api-key",
			Endpoint:       server.URL,
			MaxRetries:     2,
			RetryDelayBase: 10 * time.Millisecond,
		}
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		req := GenerateEmbeddingRequest{
			Text:  "test",
			Model: "text-embedding-3-small",
		}

		resp, err := provider.GenerateEmbedding(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 2, attempts)
	})

	t.Run("model not found", func(t *testing.T) {
		config := ProviderConfig{
			APIKey: "test-api-key",
		}
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		req := GenerateEmbeddingRequest{
			Text:  "test",
			Model: "non-existent-model",
		}

		_, err = provider.GenerateEmbedding(ctx, req)
		require.Error(t, err)
		
		provErr, ok := err.(*ProviderError)
		require.True(t, ok)
		assert.Equal(t, "MODEL_NOT_FOUND", provErr.Code)
	})
}

func TestOpenAIProvider_BatchGenerateEmbeddings(t *testing.T) {
	ctx := context.Background()

	t.Run("successful batch generation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req openAIRequest
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &req)
			
			// Verify batch input
			texts, ok := req.Input.([]interface{})
			require.True(t, ok)
			assert.Len(t, texts, 3)
			
			resp := openAIResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{
					{
						Object:    "embedding",
						Embedding: generateTestEmbedding(1536),
						Index:     0,
					},
					{
						Object:    "embedding",
						Embedding: generateTestEmbedding(1536),
						Index:     1,
					},
					{
						Object:    "embedding",
						Embedding: generateTestEmbedding(1536),
						Index:     2,
					},
				},
				Model: "text-embedding-3-small",
				Usage: struct {
					PromptTokens int `json:"prompt_tokens"`
					TotalTokens  int `json:"total_tokens"`
				}{
					PromptTokens: 10,
					TotalTokens:  10,
				},
			}
			
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := ProviderConfig{
			APIKey:   "test-api-key",
			Endpoint: server.URL,
		}
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		req := BatchGenerateEmbeddingRequest{
			Texts:  []string{"text 1", "text 2", "text 3"},
			Model:  "text-embedding-3-small",
		}

		resp, err := provider.BatchGenerateEmbeddings(ctx, req)
		require.NoError(t, err)
		
		assert.Len(t, resp.Embeddings, 3)
		assert.Equal(t, 1536, resp.Dimensions)
		assert.Equal(t, 10, resp.TotalTokens)
		
		for _, embedding := range resp.Embeddings {
			assert.Len(t, embedding, 1536)
		}
	})
}

func TestOpenAIProvider_GetModels(t *testing.T) {
	config := ProviderConfig{
		APIKey: "test-api-key",
	}
	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)

	t.Run("get supported models", func(t *testing.T) {
		models := provider.GetSupportedModels()
		assert.Len(t, models, 3)
		
		// Check for specific models
		modelNames := make(map[string]bool)
		for _, m := range models {
			modelNames[m.Name] = true
		}
		
		assert.True(t, modelNames["text-embedding-3-small"])
		assert.True(t, modelNames["text-embedding-3-large"])
		assert.True(t, modelNames["text-embedding-ada-002"])
	})

	t.Run("get specific model", func(t *testing.T) {
		model, err := provider.GetModel("text-embedding-3-large")
		require.NoError(t, err)
		
		assert.Equal(t, "text-embedding-3-large", model.Name)
		assert.Equal(t, 3072, model.Dimensions)
		assert.Equal(t, 0.13, model.CostPer1MTokens)
		assert.True(t, model.SupportsDimensionReduction)
		assert.Contains(t, model.SupportedTaskTypes, "research")
	})
}

func TestOpenAIProvider_HealthCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := openAIResponse{
				Object: "list",
				Data: []struct {
					Object    string    `json:"object"`
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{
					{
						Object:    "embedding",
						Embedding: generateTestEmbedding(1536),
						Index:     0,
					},
				},
				Model: "text-embedding-ada-002",
			}
			
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		config := ProviderConfig{
			APIKey:   "test-api-key",
			Endpoint: server.URL,
		}
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		err = provider.HealthCheck(ctx)
		assert.NoError(t, err)
	})

	t.Run("unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		config := ProviderConfig{
			APIKey:   "test-api-key",
			Endpoint: server.URL,
		}
		provider, err := NewOpenAIProvider(config)
		require.NoError(t, err)

		err = provider.HealthCheck(ctx)
		assert.Error(t, err)
	})
}

// Helper function to generate test embeddings
func generateTestEmbedding(dimensions int) []float32 {
	embedding := make([]float32, dimensions)
	for i := range embedding {
		embedding[i] = float32(i) / float32(dimensions)
	}
	return embedding
}
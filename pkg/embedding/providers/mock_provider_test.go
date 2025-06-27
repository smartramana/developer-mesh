package providers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockProvider_GenerateEmbedding(t *testing.T) {
	ctx := context.Background()

	t.Run("successful generation", func(t *testing.T) {
		provider := NewMockProvider("test-provider")

		req := GenerateEmbeddingRequest{
			Text:  "test content",
			Model: "mock-model-small",
			Metadata: map[string]interface{}{
				"test": "data",
			},
		}

		resp, err := provider.GenerateEmbedding(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "mock-model-small", resp.Model)
		assert.Equal(t, 1536, resp.Dimensions)
		assert.Len(t, resp.Embedding, 1536)
		assert.Greater(t, resp.TokensUsed, 0)
		assert.Equal(t, "test-provider", resp.ProviderInfo.Provider)

		// Verify embedding is normalized
		var sum float32
		for _, val := range resp.Embedding {
			sum += val * val
		}
		assert.InDelta(t, 1.0, sum, 0.01, "embedding should be normalized")

		// Verify call was tracked
		calls := provider.GetGenerateCalls()
		assert.Len(t, calls, 1)
		assert.Equal(t, req, calls[0])
	})

	t.Run("model not found", func(t *testing.T) {
		provider := NewMockProvider("test-provider")

		req := GenerateEmbeddingRequest{
			Text:  "test content",
			Model: "non-existent-model",
		}

		_, err := provider.GenerateEmbedding(ctx, req)
		require.Error(t, err)

		provErr, ok := err.(*ProviderError)
		require.True(t, ok)
		assert.Equal(t, "MODEL_NOT_FOUND", provErr.Code)
		assert.Equal(t, 400, provErr.StatusCode)
	})

	t.Run("with latency", func(t *testing.T) {
		provider := NewMockProvider("test-provider", WithLatency(100*time.Millisecond))

		start := time.Now()
		_, err := provider.GenerateEmbedding(ctx, GenerateEmbeddingRequest{
			Text:  "test",
			Model: "mock-model-small",
		})
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
	})

	t.Run("with failure rate", func(t *testing.T) {
		provider := NewMockProvider("test-provider", WithFailureRate(1.0))

		_, err := provider.GenerateEmbedding(ctx, GenerateEmbeddingRequest{
			Text:  "test",
			Model: "mock-model-small",
		})

		require.Error(t, err)
		provErr, ok := err.(*ProviderError)
		require.True(t, ok)
		assert.Equal(t, "MOCK_FAILURE", provErr.Code)
		assert.True(t, provErr.IsRetryable)
	})

	t.Run("fail after count", func(t *testing.T) {
		provider := NewMockProvider("test-provider", WithFailAfter(2))

		// First two requests should succeed
		for i := 0; i < 2; i++ {
			_, err := provider.GenerateEmbedding(ctx, GenerateEmbeddingRequest{
				Text:  fmt.Sprintf("test %d", i),
				Model: "mock-model-small",
			})
			require.NoError(t, err)
		}

		// Third request should fail
		_, err := provider.GenerateEmbedding(ctx, GenerateEmbeddingRequest{
			Text:  "test 3",
			Model: "mock-model-small",
		})
		require.Error(t, err)
	})

	t.Run("provider closed", func(t *testing.T) {
		provider := NewMockProvider("test-provider")
		require.NoError(t, provider.Close())

		_, err := provider.GenerateEmbedding(ctx, GenerateEmbeddingRequest{
			Text:  "test",
			Model: "mock-model-small",
		})

		require.Error(t, err)
		provErr, ok := err.(*ProviderError)
		require.True(t, ok)
		assert.Equal(t, "PROVIDER_CLOSED", provErr.Code)
	})
}

func TestMockProvider_BatchGenerateEmbeddings(t *testing.T) {
	ctx := context.Background()

	t.Run("successful batch generation", func(t *testing.T) {
		provider := NewMockProvider("test-provider")

		req := BatchGenerateEmbeddingRequest{
			Texts: []string{"text 1", "text 2", "text 3"},
			Model: "mock-model-large",
			Metadata: map[string]interface{}{
				"batch": true,
			},
		}

		resp, err := provider.BatchGenerateEmbeddings(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "mock-model-large", resp.Model)
		assert.Equal(t, 3072, resp.Dimensions)
		assert.Len(t, resp.Embeddings, 3)

		for _, embedding := range resp.Embeddings {
			assert.Len(t, embedding, 3072)
		}

		assert.Greater(t, resp.TotalTokens, 0)

		// Verify calls were tracked
		calls := provider.GetBatchGenerateCalls()
		assert.Len(t, calls, 1)
		assert.Equal(t, req, calls[0])
	})

	t.Run("different embeddings for different texts", func(t *testing.T) {
		provider := NewMockProvider("test-provider")

		resp, err := provider.BatchGenerateEmbeddings(ctx, BatchGenerateEmbeddingRequest{
			Texts: []string{"apple", "banana", "apple"},
			Model: "mock-model-small",
		})
		require.NoError(t, err)

		// Same text should produce same embedding
		assert.Equal(t, resp.Embeddings[0], resp.Embeddings[2])

		// Different texts should produce different embeddings
		assert.NotEqual(t, resp.Embeddings[0], resp.Embeddings[1])
	})
}

func TestMockProvider_Models(t *testing.T) {
	provider := NewMockProvider("test-provider")

	t.Run("get supported models", func(t *testing.T) {
		models := provider.GetSupportedModels()
		assert.Len(t, models, 4) // We have 4 default models

		// Check one model in detail
		var smallModel *ModelInfo
		for _, m := range models {
			if m.Name == "mock-model-small" {
				smallModel = &m
				break
			}
		}

		require.NotNil(t, smallModel)
		assert.Equal(t, "Mock Small Model", smallModel.DisplayName)
		assert.Equal(t, 1536, smallModel.Dimensions)
		assert.True(t, smallModel.IsActive)
	})

	t.Run("get specific model", func(t *testing.T) {
		model, err := provider.GetModel("mock-model-code")
		require.NoError(t, err)

		assert.Equal(t, "mock-model-code", model.Name)
		assert.Equal(t, 1024, model.Dimensions)
		assert.Contains(t, model.SupportedTaskTypes, "code_analysis")
	})

	t.Run("get non-existent model", func(t *testing.T) {
		_, err := provider.GetModel("non-existent")
		require.Error(t, err)

		provErr, ok := err.(*ProviderError)
		require.True(t, ok)
		assert.Equal(t, "MODEL_NOT_FOUND", provErr.Code)
	})

	t.Run("add custom model", func(t *testing.T) {
		customModel := ModelInfo{
			Name:            "custom-model",
			DisplayName:     "Custom Model",
			Dimensions:      768,
			MaxTokens:       4096,
			CostPer1MTokens: 0.05,
			IsActive:        true,
		}

		provider.SetModel(customModel)

		model, err := provider.GetModel("custom-model")
		require.NoError(t, err)
		assert.Equal(t, customModel, model)
	})
}

func TestMockProvider_HealthCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("healthy provider", func(t *testing.T) {
		provider := NewMockProvider("test-provider")
		err := provider.HealthCheck(ctx)
		assert.NoError(t, err)
	})

	t.Run("unhealthy provider", func(t *testing.T) {
		healthErr := fmt.Errorf("service unavailable")
		provider := NewMockProvider("test-provider", WithHealthCheckError(healthErr))

		err := provider.HealthCheck(ctx)
		assert.Equal(t, healthErr, err)
	})

	t.Run("closed provider", func(t *testing.T) {
		provider := NewMockProvider("test-provider")
		require.NoError(t, provider.Close())

		err := provider.HealthCheck(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})
}

func TestMockProvider_CallTracking(t *testing.T) {
	ctx := context.Background()
	provider := NewMockProvider("test-provider")

	// Make some calls
	_, _ = provider.GenerateEmbedding(ctx, GenerateEmbeddingRequest{
		Text:  "test 1",
		Model: "mock-model-small",
	})

	_, _ = provider.GenerateEmbedding(ctx, GenerateEmbeddingRequest{
		Text:  "test 2",
		Model: "mock-model-large",
	})

	_, _ = provider.BatchGenerateEmbeddings(ctx, BatchGenerateEmbeddingRequest{
		Texts: []string{"batch 1", "batch 2"},
		Model: "mock-model-small",
	})

	// Check calls
	generateCalls := provider.GetGenerateCalls()
	assert.Len(t, generateCalls, 2)
	assert.Equal(t, "test 1", generateCalls[0].Text)
	assert.Equal(t, "test 2", generateCalls[1].Text)

	batchCalls := provider.GetBatchGenerateCalls()
	assert.Len(t, batchCalls, 1)
	assert.Equal(t, []string{"batch 1", "batch 2"}, batchCalls[0].Texts)

	// Reset calls
	provider.ResetCalls()
	assert.Len(t, provider.GetGenerateCalls(), 0)
	assert.Len(t, provider.GetBatchGenerateCalls(), 0)
}

package embedding_test

import (
	"testing"

	"github.com/S-Corkum/devops-mcp/internal/embedding"
	"github.com/stretchr/testify/assert"
)

// TestEmbeddingVectorStructure verifies the EmbeddingVector structure and fields
func TestEmbeddingVectorStructure(t *testing.T) {
	// Create a test vector
	vector := &embedding.EmbeddingVector{
		Vector:      []float32{0.1, 0.2, 0.3},
		Dimensions:  3,
		ModelID:     "test-model",
		ContentType: "test-type",
		ContentID:   "test-id",
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	// Verify the vector fields
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, vector.Vector)
	assert.Equal(t, 3, vector.Dimensions)
	assert.Equal(t, "test-model", vector.ModelID)
	assert.Equal(t, "test-type", vector.ContentType)
	assert.Equal(t, "test-id", vector.ContentID)
	assert.Equal(t, "value1", vector.Metadata["key1"])
	assert.Equal(t, 123, vector.Metadata["key2"])
}

// TestEmbeddingFactoryConfig verifies the EmbeddingFactoryConfig structure and fields
func TestEmbeddingFactoryConfig(t *testing.T) {
	// Create a test config
	config := &embedding.EmbeddingFactoryConfig{
		ModelType:       embedding.ModelTypeOpenAI,
		ModelName:       "text-embedding-3-small",
		ModelAPIKey:     "test-api-key",
		ModelDimensions: 1536,
		DatabaseSchema:  "mcp",
		Concurrency:     4,
		BatchSize:       10,
		IncludeComments: true,
		EnrichMetadata:  true,
	}

	// Verify config fields
	assert.Equal(t, embedding.ModelTypeOpenAI, config.ModelType)
	assert.Equal(t, "text-embedding-3-small", config.ModelName)
	assert.Equal(t, "test-api-key", config.ModelAPIKey)
	assert.Equal(t, 1536, config.ModelDimensions)
	assert.Equal(t, "mcp", config.DatabaseSchema)
	assert.Equal(t, 4, config.Concurrency)
	assert.Equal(t, 10, config.BatchSize)
	assert.True(t, config.IncludeComments)
	assert.True(t, config.EnrichMetadata)
}

// TestEmbeddingPipelineConfig verifies the EmbeddingPipelineConfig structure and fields
func TestEmbeddingPipelineConfig(t *testing.T) {
	// Create a test config
	config := &embedding.EmbeddingPipelineConfig{
		Concurrency:     4,
		BatchSize:       10,
		IncludeComments: true,
		EnrichMetadata:  true,
	}

	// Verify config fields
	assert.Equal(t, 4, config.Concurrency)
	assert.Equal(t, 10, config.BatchSize)
	assert.True(t, config.IncludeComments)
	assert.True(t, config.EnrichMetadata)
}

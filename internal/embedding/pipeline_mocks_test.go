package embedding

import (
	"errors"
	"testing"
)

// TestHelpers contains utilities for tests
type TestHelpers struct{}

// MockChunkingWrapper wraps a MockChunkingService to make it compatible with pipeline
type MockChunkingWrapper struct {
	*MockChunkingService
}

// NewTestEmbeddingPipeline creates a pipeline for testing with mocks
func NewTestEmbeddingPipeline(
	t *testing.T,
	embeddingService EmbeddingService,
	storage EmbeddingStorage,
	chunkingService *MockChunkingService,
	contentProvider GitHubContentProvider,
	config *EmbeddingPipelineConfig,
) (*DefaultEmbeddingPipeline, error) {
	// Validate required parameters
	if embeddingService == nil {
		return nil, errors.New("embedding service is required")
	}
	
	if storage == nil {
		return nil, errors.New("embedding storage is required")
	}
	
	if chunkingService == nil {
		return nil, errors.New("chunking service is required")
	}
	
	if contentProvider == nil {
		return nil, errors.New("content provider is required")
	}
	
	if config == nil {
		return nil, errors.New("config is required")
	}
	
	// Validate config values
	if config.Concurrency <= 0 {
		return nil, errors.New("invalid concurrency value - must be greater than 0")
	}
	
	if config.BatchSize <= 0 {
		return nil, errors.New("invalid batch size - must be greater than 0")
	}

	// Create the DefaultEmbeddingPipeline directly with the mock components
	// This avoids type mismatches by working with interfaces directly
	pipeline := &DefaultEmbeddingPipeline{
		embeddingService: embeddingService,
		storage:          storage,
		contentProvider:  contentProvider,
		config:           config,
		chunkingService:  chunkingService, // Should work now with ChunkingInterface
	}
	
	return pipeline, nil
}

// No need to redefine these types as they are already defined elsewhere

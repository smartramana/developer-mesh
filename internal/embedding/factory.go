package embedding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/S-Corkum/devops-mcp/internal/chunking"
)

// EmbeddingFactoryConfig contains configuration for the embedding factory
type EmbeddingFactoryConfig struct {
	// Model configuration
	ModelType      ModelType `json:"model_type"`
	ModelName      string    `json:"model_name"`
	ModelAPIKey    string    `json:"model_api_key,omitempty"`
	ModelEndpoint  string    `json:"model_endpoint,omitempty"`
	ModelDimensions int       `json:"model_dimensions"`
	
	// Storage configuration
	DatabaseConnection *sql.DB `json:"-"`
	DatabaseSchema     string  `json:"database_schema"`
	
	// Pipeline configuration
	Concurrency     int  `json:"concurrency"`
	BatchSize       int  `json:"batch_size"`
	IncludeComments bool `json:"include_comments"`
	EnrichMetadata  bool `json:"enrich_metadata"`
}

// NewEmbeddingFactory creates a new embedding factory with the specified configuration
func NewEmbeddingFactory(config *EmbeddingFactoryConfig) (*EmbeddingFactory, error) {
	if config == nil {
		return nil, errors.New("configuration is required")
	}
	
	return &EmbeddingFactory{
		config: config,
	}, nil
}

// EmbeddingFactory creates and configures embedding components
type EmbeddingFactory struct {
	config *EmbeddingFactoryConfig
}

// CreateEmbeddingService creates an embedding service based on the factory configuration
func (f *EmbeddingFactory) CreateEmbeddingService() (EmbeddingService, error) {
	switch f.config.ModelType {
	case ModelTypeOpenAI:
		return NewOpenAIEmbeddingService(
			f.config.ModelAPIKey,
			f.config.ModelName,
			f.config.ModelDimensions,
		)
	case ModelTypeHuggingFace:
		// Not implemented yet
		return nil, errors.New("HuggingFace embedding service not implemented yet")
	case ModelTypeCustom:
		// Not implemented yet
		return nil, errors.New("custom embedding service not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported model type: %s", f.config.ModelType)
	}
}

// CreateEmbeddingStorage creates an embedding storage based on the factory configuration
func (f *EmbeddingFactory) CreateEmbeddingStorage() (EmbeddingStorage, error) {
	if f.config.DatabaseConnection == nil {
		return nil, errors.New("database connection is required")
	}
	
	return NewPgVectorStorage(f.config.DatabaseConnection, f.config.DatabaseSchema)
}

// CreateEmbeddingPipeline creates a complete embedding pipeline
func (f *EmbeddingFactory) CreateEmbeddingPipeline(
	chunkingService *chunking.ChunkingService,
	contentProvider GitHubContentProvider,
) (*DefaultEmbeddingPipeline, error) {
	// Create the embedding service
	embeddingService, err := f.CreateEmbeddingService()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding service: %w", err)
	}
	
	// Create the embedding storage
	storage, err := f.CreateEmbeddingStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding storage: %w", err)
	}
	
	// Create the pipeline configuration
	pipelineConfig := &EmbeddingPipelineConfig{
		Concurrency:     f.config.Concurrency,
		BatchSize:       f.config.BatchSize,
		IncludeComments: f.config.IncludeComments,
		EnrichMetadata:  f.config.EnrichMetadata,
	}
	
	// Create the embedding pipeline
	pipeline, err := NewEmbeddingPipeline(
		embeddingService,
		storage,
		chunkingService,
		contentProvider,
		pipelineConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding pipeline: %w", err)
	}
	
	return pipeline, nil
}

// Initialize tests all components and returns a fully configured pipeline ready for use
func (f *EmbeddingFactory) Initialize(ctx context.Context, chunkingService *chunking.ChunkingService, contentProvider GitHubContentProvider) (*DefaultEmbeddingPipeline, error) {
	// Create the embedding pipeline
	pipeline, err := f.CreateEmbeddingPipeline(chunkingService, contentProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding pipeline: %w", err)
	}
	
	// Optionally test the embedding service with a simple ping
	// This is just to verify that the embedding service can be reached
	embeddingService, err := f.CreateEmbeddingService()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding service: %w", err)
	}
	
	// Test embedding generation with a simple text
	_, err = embeddingService.GenerateEmbedding(ctx, "This is a test", "test", "test-id")
	if err != nil {
		return nil, fmt.Errorf("embedding service test failed: %w", err)
	}
	
	return pipeline, nil
}

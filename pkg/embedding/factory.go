package embedding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// EmbeddingFactoryConfig contains configuration for the embedding factory
type EmbeddingFactoryConfig struct {
	// Model configuration
	ModelType       ModelType `json:"model_type"`
	ModelName       string    `json:"model_name"`
	ModelAPIKey     string    `json:"model_api_key,omitempty"`
	ModelEndpoint   string    `json:"model_endpoint,omitempty"`
	ModelDimensions int       `json:"model_dimensions"`

	// Additional model parameters (used for provider-specific configurations)
	Parameters map[string]interface{} `json:"parameters,omitempty"`

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

	// Validate required configuration parameters
	if config.ModelType == "" {
		return nil, errors.New("model type is required")
	}

	// Check if the model type is supported
	if config.ModelType != ModelTypeOpenAI && config.ModelType != ModelTypeBedrock &&
		config.ModelType != ModelTypeHuggingFace && config.ModelType != ModelTypeAnthropic &&
		config.ModelType != ModelTypeCustom {
		return nil, fmt.Errorf("unsupported model type: %s", config.ModelType)
	}

	if config.ModelName == "" {
		return nil, errors.New("model name is required")
	}

	// Check for required API key for certain model types
	if config.ModelType == ModelTypeOpenAI && config.ModelAPIKey == "" {
		return nil, errors.New("model API key is required for OpenAI models")
	}

	if config.DatabaseConnection == nil {
		return nil, errors.New("database connection is required")
	}

	if config.DatabaseSchema == "" {
		return nil, errors.New("database schema is required")
	}

	// Set default values for optional parameters
	if config.Concurrency <= 0 {
		config.Concurrency = 4 // Default concurrency
	}

	if config.BatchSize <= 0 {
		config.BatchSize = 10 // Default batch size
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
	case ModelTypeAnthropic:
		// Extract Anthropic configuration
		endpoint := ""
		useMock := false

		// Check for endpoint in ModelEndpoint field
		if f.config.ModelEndpoint != "" {
			endpoint = f.config.ModelEndpoint
		}

		// Check for parameters
		if f.config.Parameters != nil {
			// Check for endpoint parameter
			if e, ok := f.config.Parameters["endpoint"].(string); ok && e != "" {
				endpoint = e
			}

			// Check for mock mode configuration
			if mockVal, ok := f.config.Parameters["use_mock_embeddings"].(bool); ok {
				useMock = mockVal
			} else if mockVal, ok := f.config.Parameters["use_mock"].(bool); ok {
				// Alternate naming for backward compatibility
				useMock = mockVal
			}
		}

		// Create Anthropic configuration
		config := &AnthropicConfig{
			APIKey:            f.config.ModelAPIKey,
			Endpoint:          endpoint,
			Model:             f.config.ModelName,
			UseMockEmbeddings: useMock,
		}

		// If use_mock is explicitly true, use the mock constructor directly
		if useMock {
			return NewMockAnthropicEmbeddingService(f.config.ModelName)
		}

		return NewAnthropicEmbeddingService(config)

	case ModelTypeOpenAI:
		return NewOpenAIEmbeddingService(
			f.config.ModelAPIKey,
			f.config.ModelName,
			f.config.ModelDimensions,
		)
	case ModelTypeBedrock:
		// Get AWS configuration parameters
		region := "us-west-2" // Default region
		accessKey := ""
		secretKey := ""
		sessionToken := ""
		useMock := false

		if f.config.ModelEndpoint != "" {
			region = f.config.ModelEndpoint // Using ModelEndpoint for region as a temporary solution
		}

		if f.config.Parameters != nil {
			// Check for region parameter
			if r, ok := f.config.Parameters["region"].(string); ok && r != "" {
				region = r
			}

			// Check for credentials with both naming conventions
			// 1. First check for the standard AWS SDK names
			if ak, ok := f.config.Parameters["access_key_id"].(string); ok {
				accessKey = ak
			} else if ak, ok := f.config.Parameters["access_key"].(string); ok {
				// Backward compatibility
				accessKey = ak
			}

			if sk, ok := f.config.Parameters["secret_access_key"].(string); ok {
				secretKey = sk
			} else if sk, ok := f.config.Parameters["secret_key"].(string); ok {
				// Backward compatibility
				secretKey = sk
			}

			if st, ok := f.config.Parameters["session_token"].(string); ok {
				sessionToken = st
			}

			// Check for mock mode configuration
			if mockVal, ok := f.config.Parameters["use_mock_embeddings"].(bool); ok {
				useMock = mockVal
			} else if mockVal, ok := f.config.Parameters["use_mock"].(bool); ok {
				// Alternate naming for backward compatibility
				useMock = mockVal
			}
		}

		// Create Bedrock configuration
		config := &BedrockConfig{
			Region:            region,
			AccessKeyID:       accessKey,
			SecretAccessKey:   secretKey,
			SessionToken:      sessionToken,
			ModelID:           f.config.ModelName,
			UseMockEmbeddings: useMock,
		}

		// If use_mock is explicitly true, use the mock constructor directly
		if useMock {
			return NewMockBedrockEmbeddingService(f.config.ModelName)
		}

		// Otherwise use standard constructor which will still fall back to mock if needed
		return NewBedrockEmbeddingService(config)
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
	// Validate required parameters
	if chunkingService == nil {
		return nil, errors.New("chunking service is required")
	}

	if contentProvider == nil {
		return nil, errors.New("content provider is required")
	}

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

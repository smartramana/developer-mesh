package embedding

import (
	"testing"

	"github.com/S-Corkum/devops-mcp/internal/chunking"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmbeddingFactory(t *testing.T) {
	// Create a mock DB
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Test with valid config
	config := &EmbeddingFactoryConfig{
		ModelType:          ModelTypeOpenAI,
		ModelName:          "text-embedding-3-small",
		ModelAPIKey:        "test-api-key",
		ModelDimensions:    1536,
		DatabaseConnection: db,
		DatabaseSchema:     "mcp",
		Concurrency:        4,
		BatchSize:          10,
		IncludeComments:    true,
		EnrichMetadata:     true,
	}

	factory, err := NewEmbeddingFactory(config)
	assert.NoError(t, err)
	assert.NotNil(t, factory)
	assert.Equal(t, config, factory.config)

	// Test with missing model type
	invalidConfig := &EmbeddingFactoryConfig{
		ModelName:          "text-embedding-3-small",
		ModelAPIKey:        "test-api-key",
		ModelDimensions:    1536,
		DatabaseConnection: db,
		DatabaseSchema:     "mcp",
	}

	factory, err = NewEmbeddingFactory(invalidConfig)
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "model type is required")

	// Test with missing model name
	invalidConfig = &EmbeddingFactoryConfig{
		ModelType:          ModelTypeOpenAI,
		ModelAPIKey:        "test-api-key",
		ModelDimensions:    1536,
		DatabaseConnection: db,
		DatabaseSchema:     "mcp",
	}

	factory, err = NewEmbeddingFactory(invalidConfig)
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "model name is required")

	// Test with missing API key
	invalidConfig = &EmbeddingFactoryConfig{
		ModelType:          ModelTypeOpenAI,
		ModelName:          "text-embedding-3-small",
		ModelDimensions:    1536,
		DatabaseConnection: db,
		DatabaseSchema:     "mcp",
	}

	factory, err = NewEmbeddingFactory(invalidConfig)
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "model API key is required")

	// Test with missing DB connection
	invalidConfig = &EmbeddingFactoryConfig{
		ModelType:       ModelTypeOpenAI,
		ModelName:       "text-embedding-3-small",
		ModelAPIKey:     "test-api-key",
		ModelDimensions: 1536,
		DatabaseSchema:  "mcp",
	}

	factory, err = NewEmbeddingFactory(invalidConfig)
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "database connection is required")

	// Test with missing DB schema
	invalidConfig = &EmbeddingFactoryConfig{
		ModelType:          ModelTypeOpenAI,
		ModelName:          "text-embedding-3-small",
		ModelAPIKey:        "test-api-key",
		ModelDimensions:    1536,
		DatabaseConnection: db,
	}

	factory, err = NewEmbeddingFactory(invalidConfig)
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "database schema is required")

	// Test with unsupported model type
	invalidConfig = &EmbeddingFactoryConfig{
		ModelType:          "unsupported",
		ModelName:          "test-model",
		ModelAPIKey:        "test-api-key",
		ModelDimensions:    1536,
		DatabaseConnection: db,
		DatabaseSchema:     "mcp",
	}

	factory, err = NewEmbeddingFactory(invalidConfig)
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "unsupported model type")
}

func TestEmbeddingFactory_CreateEmbeddingService(t *testing.T) {
	// Create a mock DB
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Create factory with OpenAI config
	config := &EmbeddingFactoryConfig{
		ModelType:          ModelTypeOpenAI,
		ModelName:          "text-embedding-3-small",
		ModelAPIKey:        "test-api-key",
		ModelDimensions:    1536,
		DatabaseConnection: db,
		DatabaseSchema:     "mcp",
	}

	factory, err := NewEmbeddingFactory(config)
	require.NoError(t, err)

	// Test creating an OpenAI embedding service
	service, err := factory.CreateEmbeddingService()
	assert.NoError(t, err)
	assert.NotNil(t, service)
	modelConfig := service.GetModelConfig()
	assert.Equal(t, "text-embedding-3-small", modelConfig.Name)
	assert.Equal(t, 1536, service.GetModelDimensions())

	// Test creating AWS Bedrock embedding service
	// Create a new factory with Bedrock config
	bedrockConfig := &EmbeddingFactoryConfig{
		ModelType:          ModelTypeBedrock,
		ModelName:          "amazon.titan-embed-text-v1",
		ModelDimensions:    1536,
		DatabaseConnection: db,
		DatabaseSchema:     "mcp",
		Parameters: map[string]interface{}{
			"region": "us-west-2",
		},
	}

	bedrockFactory, err := NewEmbeddingFactory(bedrockConfig)
	assert.NoError(t, err)
	assert.NotNil(t, bedrockFactory)

	// Create and verify the Bedrock service
	bedrockService, err := bedrockFactory.CreateEmbeddingService()
	assert.NoError(t, err)
	assert.NotNil(t, bedrockService)

	// Verify the model type and dimensions
	bedrockModelConfig := bedrockService.GetModelConfig()
	assert.Equal(t, ModelTypeBedrock, bedrockModelConfig.Type)
	assert.Equal(t, "amazon.titan-embed-text-v1", bedrockModelConfig.Name)
	assert.Equal(t, 1536, bedrockService.GetModelDimensions())

	// Modify factory to use unsupported model type
	factory.config.ModelType = "unsupported"
	service, err = factory.CreateEmbeddingService()
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "unsupported model type")
}

func TestEmbeddingFactory_CreateEmbeddingStorage(t *testing.T) {
	// Create a mock DB with query expectations
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	
	// Set up expectations for checking pgvector extension
	rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_extension WHERE extname = 'vector'\)`).WillReturnRows(rows)

	// Create factory with valid config
	config := &EmbeddingFactoryConfig{
		ModelType:          ModelTypeOpenAI,
		ModelName:          "text-embedding-3-small",
		ModelAPIKey:        "test-api-key",
		ModelDimensions:    1536,
		DatabaseConnection: db,
		DatabaseSchema:     "mcp",
	}

	factory, err := NewEmbeddingFactory(config)
	require.NoError(t, err)

	// Test creating a storage implementation
	storage, err := factory.CreateEmbeddingStorage()
	assert.NoError(t, err)
	assert.NotNil(t, storage)

	// Test with nil DB connection
	factory.config.DatabaseConnection = nil
	storage, err = factory.CreateEmbeddingStorage()
	assert.Error(t, err)
	assert.Nil(t, storage)
	assert.Contains(t, err.Error(), "database connection is required")
}

func TestEmbeddingFactory_CreateEmbeddingPipeline(t *testing.T) {
	// This test focuses only on the validation logic of CreateEmbeddingPipeline
	// We'll manually create a factory with a modified implementation to avoid database operations

	// Create a factory with a test config
	factory := &EmbeddingFactory{
		config: &EmbeddingFactoryConfig{
			ModelType:       ModelTypeOpenAI,
			ModelName:       "test-model",
			ModelAPIKey:     "test-key",
			ModelDimensions: 1536,
			DatabaseSchema:  "test",
		},
	}

	// Create test dependencies
	chunkingService := chunking.NewChunkingService()
	contentProvider := NewMockGitHubContentProvider()

	// Test with nil chunking service
	pipeline, err := factory.CreateEmbeddingPipeline(nil, contentProvider)
	assert.Error(t, err)
	assert.Nil(t, pipeline)
	assert.Contains(t, err.Error(), "chunking service is required")

	// Test with nil content provider
	pipeline, err = factory.CreateEmbeddingPipeline(chunkingService, nil)
	assert.Error(t, err)
	assert.Nil(t, pipeline)
	assert.Contains(t, err.Error(), "content provider is required")
}

func TestRunIntegrationTests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// These tests would connect to a real database and OpenAI API
	// They are skipped by default but could be run in a CI/CD pipeline
	// with proper environment setup.
	
	// This is just a placeholder - actual integration tests would
	// be more extensive and require environment variables for
	// database credentials and API keys.
	t.Skip("Integration tests are not implemented")
}

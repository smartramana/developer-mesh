package embedding

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEmbeddingServiceTestify is a mock implementation of the EmbeddingService interface using testify/mock
type MockEmbeddingServiceTestify struct {
	mock.Mock
}

func (m *MockEmbeddingServiceTestify) GenerateEmbedding(ctx context.Context, text, contentType, contentID string) (*EmbeddingVector, error) {
	args := m.Called(ctx, text, contentType, contentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingVector), args.Error(1)
}

// GetModelConfig implements the EmbeddingService interface
func (m *MockEmbeddingServiceTestify) GetModelConfig() ModelConfig {
	args := m.Called()
	return args.Get(0).(ModelConfig)
}

// GetModelDimensions implements the EmbeddingService interface
func (m *MockEmbeddingServiceTestify) GetModelDimensions() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockEmbeddingServiceTestify) BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error) {
	args := m.Called(ctx, texts, contentType, contentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*EmbeddingVector), args.Error(1)
}

// MockEmbeddingStorage is a mock implementation of the EmbeddingStorage interface
type MockEmbeddingStorage struct {
	mock.Mock
}

func (m *MockEmbeddingStorage) StoreEmbedding(ctx context.Context, embedding *EmbeddingVector) error {
	args := m.Called(ctx, embedding)
	return args.Error(0)
}

func (m *MockEmbeddingStorage) BatchStoreEmbeddings(ctx context.Context, embeddings []*EmbeddingVector) error {
	args := m.Called(ctx, embeddings)
	return args.Error(0)
}

func (m *MockEmbeddingStorage) FindSimilarEmbeddings(ctx context.Context, embedding *EmbeddingVector, limit int, threshold float32) ([]*EmbeddingVector, error) {
	args := m.Called(ctx, embedding, limit, threshold)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*EmbeddingVector), args.Error(1)
}

// DeleteEmbeddingsByContentIDs implements the EmbeddingStorage interface
func (m *MockEmbeddingStorage) DeleteEmbeddingsByContentIDs(ctx context.Context, contentIDs []string) error {
	args := m.Called(ctx, contentIDs)
	return args.Error(0)
}

// GetEmbeddingsByContentIDs implements the EmbeddingStorage interface
func (m *MockEmbeddingStorage) GetEmbeddingsByContentIDs(ctx context.Context, contentIDs []string) ([]*EmbeddingVector, error) {
	args := m.Called(ctx, contentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*EmbeddingVector), args.Error(1)
}

// Using the TestMockGitHubContentProvider defined in github_provider_mock_test.go

// TestNewEmbeddingPipeline tests the creation of a new embedding pipeline
func TestNewEmbeddingPipeline(t *testing.T) {
	mockService := new(MockEmbeddingServiceTestify)
	mockStorage := new(MockEmbeddingStorage)
	chunkingService := NewMockChunkingService()
	mockContentProvider := NewTestMockGitHubContentProvider()

	config := DefaultEmbeddingPipelineConfig()

	pipeline, err := NewTestEmbeddingPipeline(t, mockService, mockStorage, chunkingService, mockContentProvider, config)
	assert.NoError(t, err)
	assert.NotNil(t, pipeline)

	// Test with nil embedding service
	pipeline, err = NewTestEmbeddingPipeline(t, nil, mockStorage, chunkingService, mockContentProvider, config)
	assert.Error(t, err)
	assert.Nil(t, pipeline)
	assert.Contains(t, err.Error(), "embedding service is required")

	// Test with nil storage
	pipeline, err = NewTestEmbeddingPipeline(t, mockService, nil, chunkingService, mockContentProvider, config)
	assert.Error(t, err)
	assert.Nil(t, pipeline)
	assert.Contains(t, err.Error(), "embedding storage is required")

	// Test with nil config
	pipeline, err = NewTestEmbeddingPipeline(t, mockService, mockStorage, chunkingService, mockContentProvider, nil)
	assert.Error(t, err)
	assert.Nil(t, pipeline)
	assert.Contains(t, err.Error(), "config is required")

	// Test with invalid config
	badConfig := &EmbeddingPipelineConfig{
		Concurrency: 0, // Invalid concurrency
	}
	pipeline, err = NewTestEmbeddingPipeline(t, mockService, mockStorage, chunkingService, mockContentProvider, badConfig)
	assert.Error(t, err)
	assert.Nil(t, pipeline)
	assert.Contains(t, err.Error(), "concurrency")

	// Test with nil chunking service
	pipeline, err = NewTestEmbeddingPipeline(t, mockService, mockStorage, nil, mockContentProvider, config)
	assert.Error(t, err)
	assert.Nil(t, pipeline)
	assert.Contains(t, err.Error(), "chunking service is required")

	// Test with nil content provider
	pipeline, err = NewTestEmbeddingPipeline(t, mockService, mockStorage, chunkingService, nil, config)
	assert.Error(t, err)
	assert.Nil(t, pipeline)
	assert.Contains(t, err.Error(), "content provider is required")
}

// TestDefaultPipelineConfig tests the default pipeline configuration
func TestDefaultPipelineConfig(t *testing.T) {
	config := DefaultEmbeddingPipelineConfig()
	assert.NotNil(t, config)
	assert.Greater(t, config.Concurrency, 0)
	assert.Greater(t, config.BatchSize, 0)
}

// TestEmbeddingPipeline_ProcessContent tests the ProcessContent method
func TestEmbeddingPipeline_ProcessContent(t *testing.T) {
	// Create mocks
	mockService := new(MockEmbeddingServiceTestify)
	mockStorage := new(MockEmbeddingStorage)
	chunkingService := new(MockChunkingService)
	mockContentProvider := NewTestMockGitHubContentProvider()

	// Create pipeline
	pipeline, err := NewTestEmbeddingPipeline(
		t,
		mockService,
		mockStorage,
		chunkingService,
		mockContentProvider,
		&EmbeddingPipelineConfig{
			Concurrency:     1, // Use a single worker for deterministic testing
			BatchSize:       10,
			IncludeComments: true,
			EnrichMetadata:  true,
		},
	)
	assert.NoError(t, err)

	// Test data
	ctx := context.Background()
	contentType := "test-type"
	contentID := "test-id"
	text := "Test content"

	// Create a sample embedding vector
	embedding := &EmbeddingVector{
		ContentID:   contentID,
		Vector:      []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		Dimensions:  5,
		ModelID:     "test-model",
		ContentType: contentType,
		Metadata: map[string]interface{}{
			"key1": "value1",
		},
	}

	// Set expectations
	mockService.On("GenerateEmbedding", ctx, text, contentType, contentID).
		Return(embedding, nil).Once()

	mockStorage.On("StoreEmbedding", ctx, embedding).
		Return(nil).Once()

	// Test successful processing
	err = pipeline.ProcessContent(ctx, text, contentType, contentID)
	assert.NoError(t, err)
	mockService.AssertExpectations(t)
	mockStorage.AssertExpectations(t)

	// Test error in generating embedding
	expectedError := errors.New("embedding error")
	mockService.On("GenerateEmbedding", ctx, text, contentType, contentID).
		Return(nil, expectedError).Once()

	err = pipeline.ProcessContent(ctx, text, contentType, contentID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), expectedError.Error())
	mockService.AssertExpectations(t)

	// Test error in storing embedding
	mockService.On("GenerateEmbedding", ctx, text, contentType, contentID).
		Return(embedding, nil).Once()

	expectedError = errors.New("storage error")
	mockStorage.On("StoreEmbedding", ctx, embedding).
		Return(expectedError).Once()

	err = pipeline.ProcessContent(ctx, text, contentType, contentID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), expectedError.Error())
	mockService.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

// TestEmbeddingPipeline_BatchProcessContent tests the batch processing
func TestEmbeddingPipeline_BatchProcessContent(t *testing.T) {
	// Create mocks
	mockService := new(MockEmbeddingServiceTestify)
	mockStorage := new(MockEmbeddingStorage)
	chunkingService := NewMockChunkingService()
	mockContentProvider := NewTestMockGitHubContentProvider()

	// Create pipeline
	pipeline, err := NewTestEmbeddingPipeline(
		t,
		mockService,
		mockStorage,
		chunkingService,
		mockContentProvider,
		&EmbeddingPipelineConfig{
			Concurrency:     2, // Use 2 workers for testing concurrency
			BatchSize:       3, // Use a small batch size for testing
			IncludeComments: true,
			EnrichMetadata:  true,
		},
	)
	assert.NoError(t, err)

	// Test data
	ctx := context.Background()
	contentType := "test-type"
	contentIDs := []string{"id1", "id2", "id3", "id4", "id5"}
	contents := []string{
		"Content 1",
		"Content 2",
		"Content 3",
		"Content 4",
		"Content 5",
	}

	// Create sample embeddings
	embeddings1 := []*EmbeddingVector{
		{
			ContentID:   contentIDs[0],
			Vector:      []float32{0.1, 0.2},
			Dimensions:  2,
			ModelID:     "test-model",
			ContentType: contentType,
		},
		{
			ContentID:   contentIDs[1],
			Vector:      []float32{0.3, 0.4},
			Dimensions:  2,
			ModelID:     "test-model",
			ContentType: contentType,
		},
		{
			ContentID:   contentIDs[2],
			Vector:      []float32{0.5, 0.6},
			Dimensions:  2,
			ModelID:     "test-model",
			ContentType: contentType,
		},
	}

	embeddings2 := []*EmbeddingVector{
		{
			ContentID:   contentIDs[3],
			Vector:      []float32{0.7, 0.8},
			Dimensions:  2,
			ModelID:     "test-model",
			ContentType: contentType,
		},
		{
			ContentID:   contentIDs[4],
			Vector:      []float32{0.9, 1.0},
			Dimensions:  2,
			ModelID:     "test-model",
			ContentType: contentType,
		},
	}

	// Set expectations for first batch
	mockService.On("BatchGenerateEmbeddings", ctx, contents[0:3], contentType, contentIDs[0:3]).
		Return(embeddings1, nil).Once()
	mockStorage.On("BatchStoreEmbeddings", ctx, embeddings1).
		Return(nil).Once()

	// Set expectations for second batch
	mockService.On("BatchGenerateEmbeddings", ctx, contents[3:5], contentType, contentIDs[3:5]).
		Return(embeddings2, nil).Once()
	mockStorage.On("BatchStoreEmbeddings", ctx, embeddings2).
		Return(nil).Once()

	// Test batch processing
	err = pipeline.BatchProcessContent(ctx, contents, contentType, contentIDs)
	assert.NoError(t, err)
	mockService.AssertExpectations(t)
	mockStorage.AssertExpectations(t)

	// Test with empty batch
	err = pipeline.BatchProcessContent(ctx, []string{}, contentType, []string{})
	assert.NoError(t, err)

	// Test with mismatched content and ID lengths
	err = pipeline.BatchProcessContent(ctx, []string{"content"}, contentType, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "number of contents must match")

	// Test error in generating embeddings
	mockService.On("BatchGenerateEmbeddings", ctx, contents[0:3], contentType, contentIDs[0:3]).
		Return(nil, errors.New("batch generation error")).Once()

	err = pipeline.BatchProcessContent(ctx, contents, contentType, contentIDs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch generation error")
	mockService.AssertExpectations(t)

	// Test error in storing embeddings
	mockService.On("BatchGenerateEmbeddings", ctx, contents[0:3], contentType, contentIDs[0:3]).
		Return(embeddings1, nil).Once()
	mockStorage.On("BatchStoreEmbeddings", ctx, embeddings1).
		Return(errors.New("batch storage error")).Once()

	err = pipeline.BatchProcessContent(ctx, contents, contentType, contentIDs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch storage error")
	mockService.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

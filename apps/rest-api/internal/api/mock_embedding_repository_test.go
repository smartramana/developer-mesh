package api_test

import (
	"context"
	"rest-api/internal/repository"
	"github.com/stretchr/testify/mock"
)

type MockEmbeddingRepository struct {
	mock.Mock
}

func (m *MockEmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error {
	// Use a flexible approach to handle the actual argument type
	m.Called(ctx, mock.Anything)
	embedding.ID = "embedding-test-id" // Set a test ID for assertions
	return nil
}

func (m *MockEmbeddingRepository) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*repository.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, modelID, limit, similarityThreshold)
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*repository.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, limit)
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error) {
	args := m.Called(ctx, contextID)
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockEmbeddingRepository) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error) {
	args := m.Called(ctx, contextID, modelID)
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockEmbeddingRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	args := m.Called(ctx, contextID, modelID)
	return args.Error(0)
}

package api_test

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/repository/vector"
	"github.com/stretchr/testify/mock"
)

type MockEmbeddingRepository struct {
	mock.Mock
}

func (m *MockEmbeddingRepository) StoreEmbedding(ctx context.Context, embedding *vector.Embedding) error {
	args := m.Called(ctx, embedding)
	embedding.ID = "embedding-test-id"
	return args.Error(0)
}

func (m *MockEmbeddingRepository) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*vector.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, modelID, limit, similarityThreshold)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*vector.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, limit)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*vector.Embedding, error) {
	args := m.Called(ctx, contextID)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockEmbeddingRepository) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*vector.Embedding, error) {
	args := m.Called(ctx, contextID, modelID)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *MockEmbeddingRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockEmbeddingRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	args := m.Called(ctx, contextID, modelID)
	return args.Error(0)
}

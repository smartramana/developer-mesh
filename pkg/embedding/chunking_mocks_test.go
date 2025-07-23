package embedding

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/chunking"
	"github.com/stretchr/testify/mock"
)

// MockChunkingService is a mock implementation of the ChunkingInterface
type MockChunkingService struct {
	mock.Mock
}

// NewMockChunkingService creates a new MockChunkingService
func NewMockChunkingService() *MockChunkingService {
	return &MockChunkingService{}
}

// ChunkCode implements ChunkingInterface
func (m *MockChunkingService) ChunkCode(ctx context.Context, content string, path string) ([]*chunking.CodeChunk, error) {
	args := m.Called(ctx, content, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*chunking.CodeChunk), args.Error(1)
}

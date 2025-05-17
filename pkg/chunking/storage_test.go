package chunking

import (
	"context"
	"testing"

	"github.com/S-Corkum/devops-mcp/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGitHubContentStorage is a mock implementation of GitHubContentStorage
type MockGitHubContentStorage struct {
	mock.Mock
}

// StoreContent mocks the StoreContent method
func (m *MockGitHubContentStorage) StoreContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
	data []byte,
	metadata map[string]interface{},
) (*storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType, contentID, data, metadata)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.ContentMetadata), args.Error(1)
}

// GetContent mocks the GetContent method
func (m *MockGitHubContentStorage) GetContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
) ([]byte, *storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType, contentID)
	if args.Get(1) == nil {
		return args.Get(0).([]byte), nil, args.Error(2)
	}
	return args.Get(0).([]byte), args.Get(1).(*storage.ContentMetadata), args.Error(2)
}

// GetContentByURI mocks the GetContentByURI method
func (m *MockGitHubContentStorage) GetContentByURI(
	ctx context.Context,
	uri string,
) ([]byte, *storage.ContentMetadata, error) {
	args := m.Called(ctx, uri)
	if args.Get(1) == nil {
		return args.Get(0).([]byte), nil, args.Error(2)
	}
	return args.Get(0).([]byte), args.Get(1).(*storage.ContentMetadata), args.Error(2)
}

// ListContent mocks the ListContent method
func (m *MockGitHubContentStorage) ListContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
) ([]*storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.ContentMetadata), args.Error(1)
}

// DeleteContent mocks the DeleteContent method
func (m *MockGitHubContentStorage) DeleteContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
) error {
	args := m.Called(ctx, owner, repo, contentType, contentID)
	return args.Error(0)
}

// GetS3Client mocks the GetS3Client method
func (m *MockGitHubContentStorage) GetS3Client() storage.S3ClientInterface {
	args := m.Called()
	return args.Get(0).(storage.S3ClientInterface)
}

// MockLanguageParser is a mock implementation of LanguageParser
type MockLanguageParser struct {
	mock.Mock
}

// Parse mocks the Parse method
func (m *MockLanguageParser) Parse(ctx context.Context, code string, filename string) ([]*CodeChunk, error) {
	args := m.Called(ctx, code, filename)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*CodeChunk), args.Error(1)
}

// GetLanguage mocks the GetLanguage method
func (m *MockLanguageParser) GetLanguage() Language {
	args := m.Called()
	return args.Get(0).(Language)
}

func TestChunkingManager_ChunkAndStoreFile(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockChunkingService := NewChunkingService()
	
	// Create a test chunking manager
	manager := NewChunkingManager(mockChunkingService, mockStorage)
	
	// Setup test data
	ctx := context.Background()
	owner := "test-owner"
	repo := "test-repo"
	code := []byte("func testFunction() { return 42 }")
	filename := "test.go"
	fileMetadata := map[string]interface{}{
		"commit_id": "abc123",
		"path":      "/src/test.go",
	}
	
	// Create a mock content metadata for the storage return
	contentMetadata := &storage.ContentMetadata{
		Owner:       owner,
		Repo:        repo,
		ContentType: storage.ContentTypeFile,
		ContentID:   "test_id",
		Checksum:    "test_checksum",
		URI:         "test_uri",
		Metadata:    map[string]interface{}{},
	}
	
	// Setup storage mock expectations
	// We'll use a MatchedBy function to match any content ID since it's dynamically generated
	mockStorage.On("StoreContent", 
		ctx, 
		owner, 
		repo, 
		storage.ContentTypeFile, 
		mock.MatchedBy(func(s string) bool { return true }), 
		mock.MatchedBy(func(b []byte) bool { return true }), 
		mock.MatchedBy(func(m map[string]interface{}) bool { return true }),
	).Return(contentMetadata, nil)
	
	// Call the method
	chunks, err := manager.ChunkAndStoreFile(ctx, owner, repo, code, filename, fileMetadata)
	
	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, chunks)
	assert.True(t, len(chunks) > 0)
	mockStorage.AssertExpectations(t)
}

func TestChunkingManager_ListChunks(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockChunkingService := NewChunkingService()
	
	// Create a test chunking manager
	manager := NewChunkingManager(mockChunkingService, mockStorage)
	
	// Setup test data
	ctx := context.Background()
	owner := "test-owner"
	repo := "test-repo"
	
	// Create test content metadata
	contentMetadata1 := &storage.ContentMetadata{
		Owner:       owner,
		Repo:        repo,
		ContentType: storage.ContentTypeFile,
		ContentID:   "chunk_1",
		Checksum:    "checksum_1",
		URI:         "uri_1",
		Metadata: map[string]interface{}{
			"chunk_id":   "123",
			"chunk_type": "function",
			"language":   "go",
		},
	}
	
	contentMetadata2 := &storage.ContentMetadata{
		Owner:       owner,
		Repo:        repo,
		ContentType: storage.ContentTypeFile,
		ContentID:   "chunk_2",
		Checksum:    "checksum_2",
		URI:         "uri_2",
		Metadata: map[string]interface{}{
			"chunk_id":   "456",
			"chunk_type": "method",
			"language":   "go",
		},
	}
	
	// Create test chunk data
	chunk1Data := []byte(`{"id":"123","type":"function","name":"func1","path":"test.go","content":"func func1() {}","language":"go","start_line":1,"end_line":3}`)
	chunk2Data := []byte(`{"id":"456","type":"method","name":"method1","path":"test.go","content":"func (t *Test) method1() {}","language":"go","start_line":5,"end_line":7}`)
	
	// Setup mock expectations
	mockStorage.On("ListContent", ctx, owner, repo, storage.ContentTypeFile).
		Return([]*storage.ContentMetadata{contentMetadata1, contentMetadata2}, nil)
	
	mockStorage.On("GetContent", ctx, owner, repo, storage.ContentTypeFile, "chunk_1").
		Return(chunk1Data, contentMetadata1, nil)
	
	mockStorage.On("GetContent", ctx, owner, repo, storage.ContentTypeFile, "chunk_2").
		Return(chunk2Data, contentMetadata2, nil)
	
	// Call the method
	chunks, err := manager.ListChunks(ctx, owner, repo)
	
	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, chunks)
	assert.Equal(t, 2, len(chunks))
	assert.Equal(t, "123", chunks[0].ID)
	assert.Equal(t, "456", chunks[1].ID)
	assert.Equal(t, ChunkTypeFunction, chunks[0].Type)
	assert.Equal(t, ChunkTypeMethod, chunks[1].Type)
	mockStorage.AssertExpectations(t)
}

func TestChunkingManager_GetChunk(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockChunkingService := NewChunkingService()
	
	// Create a test chunking manager
	manager := NewChunkingManager(mockChunkingService, mockStorage)
	
	// Setup test data
	ctx := context.Background()
	owner := "test-owner"
	repo := "test-repo"
	chunkID := "123"
	
	// Create test content metadata
	contentMetadata := &storage.ContentMetadata{
		Owner:       owner,
		Repo:        repo,
		ContentType: storage.ContentTypeFile,
		ContentID:   "chunk_1",
		Checksum:    "checksum_1",
		URI:         "uri_1",
		Metadata: map[string]interface{}{
			"chunk_id":   chunkID,
			"chunk_type": "function",
			"language":   "go",
		},
	}
	
	// Create test chunk data
	chunkData := []byte(`{"id":"123","type":"function","name":"func1","path":"test.go","content":"func func1() {}","language":"go","start_line":1,"end_line":3}`)
	
	// Setup mock expectations
	mockStorage.On("ListContent", ctx, owner, repo, storage.ContentTypeFile).
		Return([]*storage.ContentMetadata{contentMetadata}, nil)
	
	mockStorage.On("GetContent", ctx, owner, repo, storage.ContentTypeFile, "chunk_1").
		Return(chunkData, contentMetadata, nil)
	
	// Call the method
	chunk, err := manager.GetChunk(ctx, owner, repo, chunkID)
	
	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, chunk)
	assert.Equal(t, chunkID, chunk.ID)
	assert.Equal(t, ChunkTypeFunction, chunk.Type)
	assert.Equal(t, "func1", chunk.Name)
	mockStorage.AssertExpectations(t)
}

func TestChunkingManager_GetChunksByType(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockChunkingService := NewChunkingService()
	
	// Create a test chunking manager
	manager := NewChunkingManager(mockChunkingService, mockStorage)
	
	// Setup test data
	ctx := context.Background()
	owner := "test-owner"
	repo := "test-repo"
	
	// Create test content metadata
	contentMetadata1 := &storage.ContentMetadata{
		Owner:       owner,
		Repo:        repo,
		ContentType: storage.ContentTypeFile,
		ContentID:   "chunk_1",
		Checksum:    "checksum_1",
		URI:         "uri_1",
		Metadata: map[string]interface{}{
			"chunk_id":   "123",
			"chunk_type": "function",
			"language":   "go",
		},
	}
	
	contentMetadata2 := &storage.ContentMetadata{
		Owner:       owner,
		Repo:        repo,
		ContentType: storage.ContentTypeFile,
		ContentID:   "chunk_2",
		Checksum:    "checksum_2",
		URI:         "uri_2",
		Metadata: map[string]interface{}{
			"chunk_id":   "456",
			"chunk_type": "method",
			"language":   "go",
		},
	}
	
	// Create test chunk data
	chunk1Data := []byte(`{"id":"123","type":"function","name":"func1","path":"test.go","content":"func func1() {}","language":"go","start_line":1,"end_line":3}`)
	chunk2Data := []byte(`{"id":"456","type":"method","name":"method1","path":"test.go","content":"func (t *Test) method1() {}","language":"go","start_line":5,"end_line":7}`)
	
	// Setup mock expectations
	mockStorage.On("ListContent", ctx, owner, repo, storage.ContentTypeFile).
		Return([]*storage.ContentMetadata{contentMetadata1, contentMetadata2}, nil)
	
	mockStorage.On("GetContent", ctx, owner, repo, storage.ContentTypeFile, "chunk_1").
		Return(chunk1Data, contentMetadata1, nil)
	
	mockStorage.On("GetContent", ctx, owner, repo, storage.ContentTypeFile, "chunk_2").
		Return(chunk2Data, contentMetadata2, nil)
	
	// Call the method to get only function chunks
	functionChunks, err := manager.GetChunksByType(ctx, owner, repo, ChunkTypeFunction)
	
	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, functionChunks)
	assert.Equal(t, 1, len(functionChunks))
	assert.Equal(t, "123", functionChunks[0].ID)
	assert.Equal(t, ChunkTypeFunction, functionChunks[0].Type)
	
	// Call the method to get only method chunks
	methodChunks, err := manager.GetChunksByType(ctx, owner, repo, ChunkTypeMethod)
	
	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, methodChunks)
	assert.Equal(t, 1, len(methodChunks))
	assert.Equal(t, "456", methodChunks[0].ID)
	assert.Equal(t, ChunkTypeMethod, methodChunks[0].Type)
	
	mockStorage.AssertExpectations(t)
}

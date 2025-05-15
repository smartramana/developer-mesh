package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockGitHubContentStorage is a mock implementation of GitHubContentStorager
type MockGitHubContentStorage struct {
	mock.Mock
}

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

func (m *MockGitHubContentStorage) GetContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType storage.ContentType,
	contentID string,
) ([]byte, *storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType, contentID)
	var content []byte
	if args.Get(0) != nil {
		content = args.Get(0).([]byte)
	}
	var metadata *storage.ContentMetadata
	if args.Get(1) != nil {
		metadata = args.Get(1).(*storage.ContentMetadata)
	}
	return content, metadata, args.Error(2)
}

func (m *MockGitHubContentStorage) GetContentByURI(
	ctx context.Context,
	uri string,
) ([]byte, *storage.ContentMetadata, error) {
	args := m.Called(ctx, uri)
	var content []byte
	if args.Get(0) != nil {
		content = args.Get(0).([]byte)
	}
	var metadata *storage.ContentMetadata
	if args.Get(1) != nil {
		metadata = args.Get(1).(*storage.ContentMetadata)
	}
	return content, metadata, args.Error(2)
}

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

func (m *MockGitHubContentStorage) GetS3Client() storage.S3Client {
	args := m.Called()
	return args.Get(0).(storage.S3Client)
}

// MockGitHubContentDB is a mock implementation of GitHubContentDBer
type MockGitHubContentDB struct {
	mock.Mock
}

func (m *MockGitHubContentDB) StoreGitHubContent(
	ctx context.Context,
	metadata *storage.ContentMetadata,
) error {
	args := m.Called(ctx, metadata)
	return args.Error(0)
}

func (m *MockGitHubContentDB) GetGitHubContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType string,
	contentID string,
) (*storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType, contentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.ContentMetadata), args.Error(1)
}

func (m *MockGitHubContentDB) DeleteGitHubContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType string,
	contentID string,
) error {
	args := m.Called(ctx, owner, repo, contentType, contentID)
	return args.Error(0)
}

func (m *MockGitHubContentDB) ListGitHubContent(
	ctx context.Context,
	owner string,
	repo string,
	contentType string,
	limit int,
) ([]*storage.ContentMetadata, error) {
	args := m.Called(ctx, owner, repo, contentType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*storage.ContentMetadata), args.Error(1)
}

// MockMetrics is a mock implementation of the Metrics interface
type MockMetrics struct {
	mock.Mock
}

func (m *MockMetrics) RecordLatency(operation string, duration time.Duration) {
	m.Called(operation, duration)
}

func (m *MockMetrics) IncrementCount(metric string) {
	m.Called(metric)
}

func (m *MockMetrics) IncrementCountBy(metric string, n int) {
	m.Called(metric, n)
}

func TestNewGitHubContentManager(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create manager
	manager := NewGitHubContentManager(mockStorage, mockDB, mockMetrics)

	// Verify
	assert.NotNil(t, manager)
	assert.Equal(t, mockStorage, manager.storage)
	assert.Equal(t, mockDB, manager.db)
	assert.Equal(t, mockMetrics, manager.metrics)
}

func TestGitHubContentManager_StoreContent(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create manager
	manager := NewGitHubContentManager(mockStorage, mockDB, mockMetrics)

	// Create test context
	ctx := context.Background()

	// Setup test data
	now := time.Now().UTC()
	testData := []byte("test-content-data")
	testMetadata := map[string]interface{}{"test": "value"}

	// Create expected metadata response
	expectedMetadata := &storage.ContentMetadata{
		Owner:       "owner",
		Repo:        "repo",
		ContentType: storage.ContentTypeIssue,
		ContentID:   "123",
		Checksum:    "9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13",
		URI:         "s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13",
		Size:        int64(len(testData)),
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    testMetadata,
	}

	// Setup expectations for metrics
	mockMetrics.On("RecordLatency", "github.content.store", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.store.count").Once()

	// Test successful storage
	mockStorage.On("StoreContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123", testData, testMetadata).
		Return(expectedMetadata, nil).Once()
	mockDB.On("StoreGitHubContent", ctx, expectedMetadata).
		Return(nil).Once()

	// Call the method
	result, err := manager.StoreContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, expectedMetadata, result)
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)

	// Test storage error
	mockMetrics.On("RecordLatency", "github.content.store", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.store.error").Once()

	mockStorage.On("StoreContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123", testData, testMetadata).
		Return(nil, errors.New("storage error")).Once()

	// Call the method
	result, err = manager.StoreContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to store GitHub content")
	mockStorage.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)

	// Test database error
	mockMetrics.On("RecordLatency", "github.content.store", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.store.error").Once()

	mockStorage.On("StoreContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123", testData, testMetadata).
		Return(expectedMetadata, nil).Once()
	mockDB.On("StoreGitHubContent", ctx, expectedMetadata).
		Return(errors.New("database error")).Once()

	// Call the method
	result, err = manager.StoreContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to store GitHub content metadata")
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGitHubContentManager_GetContent(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create manager
	manager := NewGitHubContentManager(mockStorage, mockDB, mockMetrics)

	// Create test context
	ctx := context.Background()

	// Setup test data
	testData := []byte("test-content-data")
	now := time.Now().UTC()
	expectedMetadata := &storage.ContentMetadata{
		Owner:       "owner",
		Repo:        "repo",
		ContentType: storage.ContentTypeIssue,
		ContentID:   "123",
		Checksum:    "9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13",
		URI:         "s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13",
		Size:        int64(len(testData)),
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    map[string]interface{}{"test": "value"},
	}

	// Setup expectations for metrics
	mockMetrics.On("RecordLatency", "github.content.get", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.get.count").Once()

	// Test successful retrieval
	mockStorage.On("GetContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123").
		Return(testData, expectedMetadata, nil).Once()

	// Call the method
	content, metadata, err := manager.GetContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123")

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, testData, content)
	assert.Equal(t, expectedMetadata, metadata)
	mockStorage.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)

	// Test retrieval error
	mockMetrics.On("RecordLatency", "github.content.get", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.get.error").Once()

	mockStorage.On("GetContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123").
		Return(nil, nil, errors.New("storage error")).Once()

	// Call the method
	content, metadata, err = manager.GetContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123")

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "failed to get GitHub content")
	mockStorage.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGitHubContentManager_GetContentByURI(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create manager
	manager := NewGitHubContentManager(mockStorage, mockDB, mockMetrics)

	// Create test context
	ctx := context.Background()

	// Setup test data
	testData := []byte("test-content-data")
	testURI := "s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13"
	now := time.Now().UTC()
	expectedMetadata := &storage.ContentMetadata{
		Owner:       "owner",
		Repo:        "repo",
		ContentType: storage.ContentTypeIssue,
		ContentID:   "123",
		Checksum:    "9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13",
		URI:         testURI,
		Size:        int64(len(testData)),
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    map[string]interface{}{"test": "value"},
	}

	// Setup expectations for metrics
	mockMetrics.On("RecordLatency", "github.content.get_by_uri", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.get_by_uri.count").Once()

	// Test successful retrieval
	mockStorage.On("GetContentByURI", ctx, testURI).
		Return(testData, expectedMetadata, nil).Once()

	// Call the method
	content, metadata, err := manager.GetContentByURI(ctx, testURI)

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, testData, content)
	assert.Equal(t, expectedMetadata, metadata)
	mockStorage.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)

	// Test retrieval error
	mockMetrics.On("RecordLatency", "github.content.get_by_uri", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.get_by_uri.error").Once()

	mockStorage.On("GetContentByURI", ctx, testURI).
		Return(nil, nil, errors.New("storage error")).Once()

	// Call the method
	content, metadata, err = manager.GetContentByURI(ctx, testURI)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "failed to get GitHub content by URI")
	mockStorage.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGitHubContentManager_DeleteContent(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create manager
	manager := NewGitHubContentManager(mockStorage, mockDB, mockMetrics)

	// Create test context
	ctx := context.Background()

	// Setup expectations for metrics
	mockMetrics.On("RecordLatency", "github.content.delete", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.delete.count").Once()

	// Test successful deletion
	mockStorage.On("DeleteContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123").
		Return(nil).Once()
	mockDB.On("DeleteGitHubContent", ctx, "owner", "repo", "issue", "123").
		Return(nil).Once()

	// Call the method
	err := manager.DeleteContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123")

	// Verify results
	assert.NoError(t, err)
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)

	// Test storage deletion error
	mockMetrics.On("RecordLatency", "github.content.delete", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.delete.error").Once()

	mockStorage.On("DeleteContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123").
		Return(errors.New("storage error")).Once()

	// Call the method
	err = manager.DeleteContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123")

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete GitHub content")
	mockStorage.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)

	// Test database deletion error
	mockMetrics.On("RecordLatency", "github.content.delete", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.delete.error").Once()

	mockStorage.On("DeleteContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123").
		Return(nil).Once()
	mockDB.On("DeleteGitHubContent", ctx, "owner", "repo", "issue", "123").
		Return(errors.New("database error")).Once()

	// Call the method
	err = manager.DeleteContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123")

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete GitHub content metadata")
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGitHubContentManager_ListContent(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create manager
	manager := NewGitHubContentManager(mockStorage, mockDB, mockMetrics)

	// Create test context
	ctx := context.Background()

	// Setup test data
	now := time.Now().UTC()
	expectedMetadata := []*storage.ContentMetadata{
		{
			Owner:       "owner",
			Repo:        "repo",
			ContentType: storage.ContentTypeIssue,
			ContentID:   "123",
			Checksum:    "hash1",
			URI:         "s3://test-bucket/content/hash1",
			Size:        100,
			CreatedAt:   now,
			UpdatedAt:   now,
			Metadata:    map[string]interface{}{"test": "value1"},
		},
		{
			Owner:       "owner",
			Repo:        "repo",
			ContentType: storage.ContentTypeIssue,
			ContentID:   "456",
			Checksum:    "hash2",
			URI:         "s3://test-bucket/content/hash2",
			Size:        200,
			CreatedAt:   now,
			UpdatedAt:   now,
			Metadata:    map[string]interface{}{"test": "value2"},
		},
	}

	// Setup expectations for metrics
	mockMetrics.On("RecordLatency", "github.content.list", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.list.count").Once()

	// Test successful listing
	mockDB.On("ListGitHubContent", ctx, "owner", "repo", "issue", 0).
		Return(expectedMetadata, nil).Once()

	// Call the method
	result, err := manager.ListContent(ctx, "owner", "repo", storage.ContentTypeIssue, 0)

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, expectedMetadata, result)
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)

	// Test listing error
	mockMetrics.On("RecordLatency", "github.content.list", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("IncrementCount", "github.content.list.error").Once()

	mockDB.On("ListGitHubContent", ctx, "owner", "repo", "issue", 0).
		Return(nil, errors.New("database error")).Once()

	// Call the method
	result, err = manager.ListContent(ctx, "owner", "repo", storage.ContentTypeIssue, 0)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list GitHub content")
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

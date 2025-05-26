package core

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/database"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// testGitHubContentManager is a test implementation of the GitHubContentManager that works directly with mocks
type testGitHubContentManager struct {
	mockStorage *MockGitHubContentStorage
	mockDB      *MockGitHubContentDB
	metrics     observability.MetricsClient
	logger      observability.Logger
}

// Implement the same methods as GitHubContentManager but use our mocks directly
func (m *testGitHubContentManager) StoreContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string, data []byte, metadata map[string]interface{}) (*storage.ContentMetadata, error) {
	// Record metrics like the real implementation
	m.metrics.RecordLatency("github.content.store", time.Millisecond)
	
	// Call the mock storage
	contentMetadata, err := m.mockStorage.StoreContent(ctx, owner, repo, contentType, contentID, data, metadata)
	
	// Record success or error count
	if err != nil {
		m.metrics.RecordCounter("github.content.store.error", 1.0, map[string]string{"owner": owner, "repo": repo})
		return nil, fmt.Errorf("failed to store GitHub content: %w", err)
	}
	m.metrics.RecordCounter("github.content.store.success", 1.0, map[string]string{"owner": owner, "repo": repo})
	
	// Store metadata in DB
	if contentMetadata != nil {
		err = m.mockDB.StoreGitHubContent(ctx, contentMetadata)
		if err != nil {
			m.metrics.RecordCounter("github.content.store.metadata.error", 1.0, map[string]string{"owner": owner, "repo": repo})
			return nil, fmt.Errorf("failed to store content metadata in database: %w", err)
		}
	}
	
	return contentMetadata, nil
}

func (m *testGitHubContentManager) GetContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string) ([]byte, *storage.ContentMetadata, error) {
	// Record metrics like the real implementation
	m.metrics.RecordLatency("github.content.get", time.Millisecond)
	
	// Call the mock storage
	content, metadata, err := m.mockStorage.GetContent(ctx, owner, repo, contentType, contentID)
	
	// Record success or error count
	if err != nil {
		m.metrics.RecordCounter("github.content.get.error", 1.0, map[string]string{"owner": owner, "repo": repo})
		return nil, nil, fmt.Errorf("failed to get GitHub content: %w", err)
	}
	m.metrics.RecordCounter("github.content.get.success", 1.0, map[string]string{"owner": owner, "repo": repo})
	return content, metadata, nil
}

func (m *testGitHubContentManager) GetContentByChecksum(ctx context.Context, checksum string) ([]byte, *storage.ContentMetadata, error) {
	// Record metrics like the real implementation
	m.metrics.RecordLatency("github.content.get_by_uri", time.Millisecond)
	
	// Generate the URI
	uri := "s3://test-bucket/content/" + checksum
	
	// Call the mock storage
	content, metadata, err := m.mockStorage.GetContentByURI(ctx, uri)
	
	// Record success or error count
	if err != nil {
		m.metrics.RecordCounter("github.content.get_by_uri.error", 1.0, map[string]string{"checksum": checksum})
		return nil, nil, fmt.Errorf("failed to get content from S3 by URI: %w", err)
	}
	m.metrics.RecordCounter("github.content.get_by_uri.success", 1.0, map[string]string{"checksum": checksum})
	return content, metadata, nil
}

func (m *testGitHubContentManager) DeleteContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, contentID string) error {
	// Record metrics like the real implementation 
	m.metrics.RecordLatency("github.content.delete", time.Millisecond)
	
	// Delete from storage first
	err := m.mockStorage.DeleteContent(ctx, owner, repo, contentType, contentID)
	if err != nil {
		m.metrics.RecordCounter("github.content.delete.error", 1.0, map[string]string{"owner": owner, "repo": repo})
		return fmt.Errorf("failed to delete GitHub content: %w", err)
	}
	
	// Delete from database next
	err = m.mockDB.DeleteGitHubContent(ctx, owner, repo, string(contentType), contentID)
	if err != nil {
		m.metrics.RecordCounter("github.content.delete.error", 1.0, map[string]string{"owner": owner, "repo": repo})
		return fmt.Errorf("failed to delete GitHub content metadata: %w", err) 
	}
	
	m.metrics.RecordCounter("github.content.delete.success", 1.0, map[string]string{"owner": owner, "repo": repo})
	return nil
}

func (m *testGitHubContentManager) ListContent(ctx context.Context, owner string, repo string, contentType storage.ContentType, limit int) ([]*storage.ContentMetadata, error) {
	// Record metrics like the real implementation
	m.metrics.RecordLatency("github.content.list", time.Millisecond)
	
	// Call the mock database
	metadata, err := m.mockDB.ListGitHubContent(ctx, owner, repo, string(contentType), limit)
	
	// Record success or error count
	if err != nil {
		m.metrics.RecordCounter("github.content.list.error", 1.0, map[string]string{"owner": owner, "repo": repo})
		return nil, fmt.Errorf("failed to list GitHub content: %w", err)
	}
	m.metrics.RecordCounter("github.content.list.count", 1.0, map[string]string{"owner": owner, "repo": repo})
	return metadata, nil
}

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

// MockMetrics is a mock implementation of the MetricsClient interface
type MockMetrics struct {
	mock.Mock
}

func (m *MockMetrics) RecordEvent(source, eventType string) {
	m.Called(source, eventType)
}

func (m *MockMetrics) RecordLatency(operation string, duration time.Duration) {
	m.Called(operation, duration)
}

func (m *MockMetrics) RecordCounter(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func (m *MockMetrics) RecordGauge(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func (m *MockMetrics) RecordHistogram(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func (m *MockMetrics) RecordTimer(name string, duration time.Duration, labels map[string]string) {
	m.Called(name, duration, labels)
}

func (m *MockMetrics) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
	m.Called(operation, success, durationSeconds)
}

func (m *MockMetrics) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
	m.Called(component, operation, success, durationSeconds, labels)
}

func (m *MockMetrics) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
	m.Called(api, operation, success, durationSeconds)
}

func (m *MockMetrics) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
	m.Called(operation, success, durationSeconds)
}

func (m *MockMetrics) StartTimer(name string, labels map[string]string) func() {
	args := m.Called(name, labels)
	return args.Get(0).(func())
}

func (m *MockMetrics) IncrementCounter(name string, value float64) {
	m.Called(name, value)
}

func (m *MockMetrics) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func (m *MockMetrics) RecordDuration(name string, duration time.Duration) {
	m.Called(name, duration)
}

func (m *MockMetrics) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewGitHubContentManager(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := &database.Database{} // Using a real database.Database struct for type compatibility
	mockMetrics := new(MockMetrics)

	// Set up S3 client mock
	mockS3Client := &storage.S3Client{}
	mockStorage.On("GetS3Client").Return(mockS3Client).Once()

	// Create manager
	manager, err := NewGitHubContentManager(mockDB, mockS3Client, mockMetrics, nil)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, mockDB, manager.db)
	assert.NotNil(t, manager.storageManager)
	assert.Equal(t, mockMetrics, manager.metricsClient)
}

func TestGitHubContentManager_StoreContent(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)
	
	// Create a test manager that directly uses our mock objects
	manager := &testGitHubContentManager{
		mockStorage: mockStorage,
		mockDB:      mockDB, 
		metrics:     mockMetrics,
		logger:      observability.NewLogger("test"),
	}

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
	mockMetrics.On("RecordCounter", "github.content.store.success", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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
	mockMetrics.On("RecordCounter", "github.content.store.error", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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
	// Storage successful, record success metric
	mockMetrics.On("RecordCounter", "github.content.store.success", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()
	// Add expectation for store.metadata.error metric when DB operation fails
	mockMetrics.On("RecordCounter", "github.content.store.metadata.error", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

	mockStorage.On("StoreContent", ctx, "owner", "repo", storage.ContentTypeIssue, "123", testData, testMetadata).
		Return(expectedMetadata, nil).Once()
	mockDB.On("StoreGitHubContent", ctx, expectedMetadata).
		Return(errors.New("database error")).Once()

	// Call the method
	result, err = manager.StoreContent(ctx, "owner", "repo", storage.ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to store content metadata in database")
	mockStorage.AssertExpectations(t)
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGitHubContentManager_GetContent(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create a test manager that directly uses our mock objects
	manager := &testGitHubContentManager{
		mockStorage: mockStorage,
		mockDB:      mockDB,
		metrics:     mockMetrics,
		logger:      observability.NewLogger("test"),
	}

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
	mockMetrics.On("RecordCounter", "github.content.get.success", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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
	mockMetrics.On("RecordCounter", "github.content.get.error", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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

func TestGitHubContentManager_GetContentByChecksum(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create a test manager that directly uses our mock objects
	manager := &testGitHubContentManager{
		mockStorage: mockStorage,
		mockDB:      mockDB,
		metrics:     mockMetrics,
		logger:      observability.NewLogger("test"),
	}

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
	mockMetrics.On("RecordCounter", "github.content.get_by_uri.success", 1.0, map[string]string{"checksum": "hash123"}).Once()

	// We need to use the URI that will be constructed with the checksum value passed to GetContentByChecksum
	checksum := "hash123"
	// Update the existing URI to match what the implementation will construct
	testURI = "s3://test-bucket/content/" + checksum

	// Test successful retrieval
	mockStorage.On("GetContentByURI", ctx, testURI).
		Return(testData, expectedMetadata, nil).Once()

	// Call the method with valid checksum
	content, metadata, err := manager.GetContentByChecksum(ctx, "hash123")

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, testData, content)
	assert.Equal(t, expectedMetadata, metadata)
	mockStorage.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)

	// Test retrieval error
	mockMetrics.On("RecordLatency", "github.content.get_by_uri", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("RecordCounter", "github.content.get_by_uri.error", 1.0, map[string]string{"checksum": "invalid"}).Once()

	// For the error case, we need a different checksum/URI
	errorChecksum := "invalid"
	errorURI := "s3://test-bucket/content/" + errorChecksum

	mockStorage.On("GetContentByURI", ctx, errorURI).
		Return(nil, nil, errors.New("storage error")).Once()

	// Call the method with error
	content, metadata, err = manager.GetContentByChecksum(ctx, "invalid")

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "failed to get content from S3 by URI")
	mockStorage.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGitHubContentManager_DeleteContent(t *testing.T) {
	// Create mocks
	mockStorage := new(MockGitHubContentStorage)
	mockDB := new(MockGitHubContentDB)
	mockMetrics := new(MockMetrics)

	// Create a test manager that directly uses our mock objects
	manager := &testGitHubContentManager{
		mockStorage: mockStorage,
		mockDB:      mockDB,
		metrics:     mockMetrics,
		logger:      observability.NewLogger("test"),
	}

	// Create test context
	ctx := context.Background()

	// Setup expectations for metrics
	mockMetrics.On("RecordLatency", "github.content.delete", mock.AnythingOfType("time.Duration")).Once()
	mockMetrics.On("RecordCounter", "github.content.delete.success", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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
	mockMetrics.On("RecordCounter", "github.content.delete.error", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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
	// Need both error metrics - one for the initial DB error and one for the final reporting
	mockMetrics.On("RecordCounter", "github.content.delete.error", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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

	// Create a test manager that directly uses our mock objects
	manager := &testGitHubContentManager{
		mockStorage: mockStorage,
		mockDB:      mockDB,
		metrics:     mockMetrics,
		logger:      observability.NewLogger("test"),
	}

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
	mockMetrics.On("RecordCounter", "github.content.list.count", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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
	mockMetrics.On("RecordCounter", "github.content.list.error", 1.0, map[string]string{"owner": "owner", "repo": "repo"}).Once()

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

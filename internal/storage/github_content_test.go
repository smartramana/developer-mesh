package storage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockS3Client is a mock implementation of S3Clienter interface for testing
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) GetBucketName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockS3Client) UploadFile(ctx context.Context, key string, data []byte, contentType string) error {
	args := m.Called(ctx, key, data, contentType)
	return args.Error(0)
}

func (m *MockS3Client) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockS3Client) DeleteFile(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockS3Client) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// TestNewGitHubContentStorage tests the constructor for GitHubContentStorage
func TestNewGitHubContentStorage(t *testing.T) {
	mock := new(MockS3Client)

	// Mock behavior
	mock.On("GetBucketName").Return("test-bucket")

	// Create GitHubContentStorage
	storage := &GitHubContentStorage{
		s3Client:   mock,
		bucketName: mock.GetBucketName(),
	}

	// Verify
	assert.NotNil(t, storage)
	assert.Equal(t, mock, storage.s3Client)
	assert.Equal(t, "test-bucket", storage.bucketName)

	mock.AssertExpectations(t)
}

// TestGitHubContentStorage_GetS3Client tests GetS3Client method
func TestGitHubContentStorage_GetS3Client(t *testing.T) {
	mock := new(MockS3Client)

	// Mock behavior
	mock.On("GetBucketName").Return("test-bucket")

	// Create GitHubContentStorage
	storage := &GitHubContentStorage{
		s3Client:   mock,
		bucketName: mock.GetBucketName(),
	}

	// Test
	result := storage.GetS3Client()

	// Verify
	assert.Equal(t, mock, result)

	mock.AssertExpectations(t)
}

// TestGitHubContentStorage_StoreContent tests StoreContent method
func TestGitHubContentStorage_StoreContent(t *testing.T) {
	ctx := context.Background()

	// Test data
	testData := []byte("test content")
	testMetadata := map[string]interface{}{"test": "value"}

	// Successful upload
	t.Run("New Content", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - new content
		mock.On("ListFiles", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "content/")
		})).Return([]string{}, nil).Once()

		mock.On("UploadFile", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "content/")
		}), testData, "application/octet-stream").Return(nil).Once()

		mock.On("UploadFile", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "repositories/owner/repo/issue/123")
		}), mock.AnythingOfType("[]uint8"), "application/json").Return(nil).Once()

		mock.On("UploadFile", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "repositories/owner/repo/issue/123.ref")
		}), mock.AnythingOfType("[]uint8"), "text/plain").Return(nil).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

		// Verify
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "owner", result.Owner)
		assert.Equal(t, "repo", result.Repo)
		assert.Equal(t, ContentTypeIssue, result.ContentType)
		assert.Equal(t, "123", result.ContentID)
		assert.Equal(t, int64(len(testData)), result.Size)
		assert.Equal(t, testMetadata, result.Metadata)

		mock.AssertExpectations(t)
	})

	// Test deduplication
	t.Run("Existing Content", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - content exists
		mock.On("ListFiles", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "content/")
		})).Return([]string{"content/someHash"}, nil).Once()

		mock.On("UploadFile", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "repositories/owner/repo/issue/123")
		}), mock.AnythingOfType("[]uint8"), "application/json").Return(nil).Once()

		mock.On("UploadFile", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "repositories/owner/repo/issue/123.ref")
		}), mock.AnythingOfType("[]uint8"), "text/plain").Return(nil).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

		// Verify
		assert.NoError(t, err)
		assert.NotNil(t, result)

		mock.AssertExpectations(t)
	})

	// Test error checking if content exists
	t.Run("ListFiles Error", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - error checking content
		mock.On("ListFiles", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "content/")
		})).Return(nil, errors.New("list error")).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

		// Verify
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to check if content exists")

		mock.AssertExpectations(t)
	})

	// Test error uploading content
	t.Run("Upload Error", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - upload error
		mock.On("ListFiles", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "content/")
		})).Return([]string{}, nil).Once()

		mock.On("UploadFile", mock.Anything, mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "content/")
		}), testData, "application/octet-stream").Return(errors.New("upload error")).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

		// Verify
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to upload content to S3")

		mock.AssertExpectations(t)
	})
}

// TestGitHubContentStorage_GetContent tests GetContent method
func TestGitHubContentStorage_GetContent(t *testing.T) {
	ctx := context.Background()

	// Test data
	testData := []byte("test content")
	testRef := []byte("content/someHash")
	testMetadataObj := &ContentMetadata{
		Owner:       "owner",
		Repo:        "repo",
		ContentType: ContentTypeIssue,
		ContentID:   "123",
		Checksum:    "someHash",
		URI:         "s3://test-bucket/content/someHash",
	}
	testMetadataJSON, _ := json.Marshal(testMetadataObj)

	// Success case
	t.Run("Success", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - success
		mock.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123.ref").Return(testRef, nil).Once()
		mock.On("DownloadFile", mock.Anything, "content/someHash").Return(testData, nil).Once()
		mock.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123").Return(testMetadataJSON, nil).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		content, metadata, err := storage.GetContent(ctx, "owner", "repo", ContentTypeIssue, "123")

		// Verify
		assert.NoError(t, err)
		assert.Equal(t, testData, content)
		assert.NotNil(t, metadata)
		assert.Equal(t, "owner", metadata.Owner)
		assert.Equal(t, "repo", metadata.Repo)

		mock.AssertExpectations(t)
	})

	// Error getting reference
	t.Run("Reference Error", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - reference error
		mock.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123.ref").Return(nil, errors.New("reference error")).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		content, metadata, err := storage.GetContent(ctx, "owner", "repo", ContentTypeIssue, "123")

		// Verify
		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Nil(t, metadata)
		assert.Contains(t, err.Error(), "failed to download content reference")

		mock.AssertExpectations(t)
	})
}

// TestGitHubContentStorage_DeleteContent tests DeleteContent method
func TestGitHubContentStorage_DeleteContent(t *testing.T) {
	ctx := context.Background()

	// Test data
	testRef := []byte("content/someHash")

	// Success case
	t.Run("Success", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - success
		mock.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123.ref").Return(testRef, nil).Once()
		mock.On("DeleteFile", mock.Anything, "repositories/owner/repo/issue/123.ref").Return(nil).Once()
		mock.On("DeleteFile", mock.Anything, "repositories/owner/repo/issue/123").Return(nil).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		err := storage.DeleteContent(ctx, "owner", "repo", ContentTypeIssue, "123")

		// Verify
		assert.NoError(t, err)

		mock.AssertExpectations(t)
	})

	// Error getting reference
	t.Run("Reference Error", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - reference error
		mock.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123.ref").Return(nil, errors.New("reference error")).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		err := storage.DeleteContent(ctx, "owner", "repo", ContentTypeIssue, "123")

		// Verify
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download content reference")

		mock.AssertExpectations(t)
	})
}

// TestGitHubContentStorage_ListContent tests ListContent method
func TestGitHubContentStorage_ListContent(t *testing.T) {
	ctx := context.Background()

	// Test data
	testMetadata1, _ := json.Marshal(ContentMetadata{ContentID: "123"})
	testMetadata2, _ := json.Marshal(ContentMetadata{ContentID: "456"})

	// Success case
	t.Run("Success", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - success
		mock.On("ListFiles", mock.Anything, "repositories/owner/repo/issue/").Return([]string{
			"repositories/owner/repo/issue/123",
			"repositories/owner/repo/issue/123.ref",
			"repositories/owner/repo/issue/456",
			"repositories/owner/repo/issue/456.ref",
		}, nil).Once()

		mock.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123").Return(testMetadata1, nil).Once()
		mock.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/456").Return(testMetadata2, nil).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		results, err := storage.ListContent(ctx, "owner", "repo", ContentTypeIssue)

		// Verify
		assert.NoError(t, err)
		assert.Len(t, results, 2)

		mock.AssertExpectations(t)
	})

	// Error listing content
	t.Run("List Error", func(t *testing.T) {
		mock := new(MockS3Client)
		mock.On("GetBucketName").Return("test-bucket")

		// Mock behavior - list error
		mock.On("ListFiles", mock.Anything, "repositories/owner/repo/issue/").Return(nil, errors.New("list error")).Once()

		// Create GitHubContentStorage
		storage := &GitHubContentStorage{
			s3Client:   mock,
			bucketName: mock.GetBucketName(),
		}

		// Test
		results, err := storage.ListContent(ctx, "owner", "repo", ContentTypeIssue)

		// Verify
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "failed to list content")

		mock.AssertExpectations(t)
	})
}

// TestCalculateContentHash tests CalculateContentHash function
func TestCalculateContentHash(t *testing.T) {
	// Test data
	testData := []byte("test content")
	expectedHash := "6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"

	// Test
	hash := CalculateContentHash(testData)

	// Verify
	assert.Equal(t, expectedHash, hash)
}

// GitHubS3ClientMock mocks the S3Client interface for testing
type GitHubS3ClientMock struct {
	mock.Mock
	bucketName string
}

// Create a new mock implementation that will satisfy the S3Client interface
func NewGitHubS3ClientMock(bucketName string) *S3Client {
	mock := &GitHubS3ClientMock{
		bucketName: bucketName,
	}
	return &S3Client{impl: mock}
}

// Implement S3Clienter interface methods for GitHubS3ClientMock
func (m *GitHubS3ClientMock) GetBucketName() string {
	return m.bucketName
}

func (m *GitHubS3ClientMock) UploadFile(ctx context.Context, key string, data []byte, contentType string) error {
	args := m.Called(ctx, key, data, contentType)
	return args.Error(0)
}

func (m *GitHubS3ClientMock) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *GitHubS3ClientMock) DeleteFile(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *GitHubS3ClientMock) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func TestNewGitHubContentStorage(t *testing.T) {
	// Create a mock S3Client
	mockClient := NewGitHubS3ClientMock("test-bucket")

	// Call the constructor
	storage := NewGitHubContentStorage(mockClient)
	
	// Verify the storage was created correctly
	assert.NotNil(t, storage)
	assert.Equal(t, mockClient, storage.s3Client)
	assert.Equal(t, "test-bucket", storage.bucketName)
}

func TestGetS3Client(t *testing.T) {
	// Create a mock S3Client
	mockClient := NewGitHubS3ClientMock("test-bucket")

	// Create storage with mock client
	storage := NewGitHubContentStorage(mockClient)
	
	// Verify GetS3Client returns the correct client
	assert.Equal(t, mockClient, storage.GetS3Client())
}

func TestStoreContent(t *testing.T) {
	ctx := context.Background()
	
	// Test successful content storage with new content
	{
		// Create a mock S3Client
		mockClient := NewGitHubS3ClientMock("test-bucket")
		mockImpl := mockClient.impl.(*GitHubS3ClientMock)
		
		// Create storage with mock client
		storage := NewGitHubContentStorage(mockClient)
		
		// Test data
		testData := []byte("test-content-data")
		testMetadata := map[string]interface{}{"test": "value"}
		
		// Mock responses for the StoreContent method
		mockImpl.On("ListFiles", mock.Anything, "content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13").
			Return([]string{}, nil).Once() // No existing content
			
		mockImpl.On("UploadFile", mock.Anything, 
			"content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", 
			testData, "application/octet-stream").
			Return(nil).Once() // Successful upload
			
		mockImpl.On("UploadFile", mock.Anything, mock.MatchedBy(func(s string) bool {
			return strings.HasPrefix(s, "repositories/owner/repo/issue/123")
		}), mock.AnythingOfType("[]uint8"), "application/json").
			Return(nil).Once() // Metadata upload
			
		mockImpl.On("UploadFile", mock.Anything, mock.MatchedBy(func(s string) bool {
			return strings.HasPrefix(s, "repositories/owner/repo/issue/123.ref")
		}), mock.AnythingOfType("[]uint8"), "text/plain").
			Return(nil).Once() // Reference upload
			
		// Call the method
		contentMetadata, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)
		
		// Verify results
		assert.NoError(t, err)
		assert.NotNil(t, contentMetadata)
		assert.Equal(t, "owner", contentMetadata.Owner)
		assert.Equal(t, "repo", contentMetadata.Repo)
		assert.Equal(t, ContentTypeIssue, contentMetadata.ContentType)
		assert.Equal(t, "123", contentMetadata.ContentID)
		assert.Equal(t, "9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", contentMetadata.Checksum)
		assert.Equal(t, "s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", contentMetadata.URI)
		assert.Equal(t, int64(len(testData)), contentMetadata.Size)
		assert.Equal(t, testMetadata, contentMetadata.Metadata)
		
		// Verify mock expectations were met
		mockImpl.AssertExpectations(t)
	}
	// Test content already exists (deduplication)
	{
		// Create a mock S3Client
		mockClient := new(MockS3Client)
		mockClient.On("GetBucketName").Return("test-bucket")
		
		// Create storage with mock client
		storage := NewGitHubContentStorage(mockClient)
		
		// Test data
		testData := []byte("test-content-data")
		testMetadata := map[string]interface{}{"test": "value"}
		
		// Mock responses for deduplication case
		mockClient.On("ListFiles", mock.Anything, "content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13").
			Return([]string{"content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13"}, nil).Once() // Content exists
			
		mockClient.On("UploadFile", mock.Anything, mock.MatchedBy(func(s string) bool {
			return strings.HasPrefix(s, "repositories/owner/repo/issue/123")
		}), mock.AnythingOfType("[]uint8"), "application/json").
			Return(nil).Once() // Metadata upload
			
		mockClient.On("UploadFile", mock.Anything, mock.MatchedBy(func(s string) bool {
			return strings.HasPrefix(s, "repositories/owner/repo/issue/123.ref")
		}), mock.AnythingOfType("[]uint8"), "text/plain").
			Return(nil).Once() // Reference upload
			
		// Call the method
		contentMetadata, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)
		
		// Verify results
		assert.NoError(t, err)
		assert.NotNil(t, contentMetadata)
		
		// Verify mock expectations were met
		mockClient.AssertExpectations(t)
	}
	// Test error checking if content exists
	{
		// Create a mock S3Client
		mockClient := new(MockS3Client)
		mockClient.On("GetBucketName").Return("test-bucket")
		
		// Create storage with mock client
		storage := NewGitHubContentStorage(mockClient)
		
		// Test data
		testData := []byte("test-content-data")
		testMetadata := map[string]interface{}{"test": "value"}
		
		// Mock error response when checking content
		mockClient.On("ListFiles", mock.Anything, "content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13").
			Return(nil, errors.New("list error")).Once() // Error checking content
			
		// Call the method
		contentMetadata, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)
		
		// Verify results
		assert.Error(t, err)
		assert.Nil(t, contentMetadata)
		assert.Contains(t, err.Error(), "failed to check if content exists")
		
		// Verify mock expectations were met
		mockClient.AssertExpectations(t)
	}
	// Test error uploading content
	{
		// Create a mock S3Client
		mockClient := new(MockS3Client)
		mockClient.On("GetBucketName").Return("test-bucket")
		
		// Create storage with mock client
		storage := NewGitHubContentStorage(mockClient)
		
		// Test data
		testData := []byte("test-content-data")
		testMetadata := map[string]interface{}{"test": "value"}
		
		// Mock responses with an upload error
		mockClient.On("ListFiles", mock.Anything, "content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13").
			Return([]string{}, nil).Once() // No existing content
			
		mockClient.On("UploadFile", mock.Anything, 
			"content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", 
			testData, "application/octet-stream").
			Return(errors.New("upload error")).Once() // Error uploading
			
		// Call the method
		contentMetadata, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)
		
		// Verify results
		assert.Error(t, err)
		assert.Nil(t, contentMetadata)
		assert.Contains(t, err.Error(), "failed to upload content to S3")
		
		// Verify mock expectations were met
		mockClient.AssertExpectations(t)
	}
}

func TestGetContent(t *testing.T) {
	ctx := context.Background()
	mockS3Client := new(MockGitHubS3Client)
	mockS3Client.On("GetBucketName").Return("test-bucket")
	
	storage := NewGitHubContentStorage(mockS3Client)
	
	// Test successful content retrieval
	testData := []byte("test-content-data")
	testRef := []byte("content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13")
	testMetadataJSON := []byte(`{"owner":"owner","repo":"repo","content_type":"issue","content_id":"123","checksum":"9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13","uri":"s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13","size":16,"created_at":"2025-05-15T18:00:00Z","updated_at":"2025-05-15T18:00:00Z","metadata":{"test":"value"}}`)
	
	// Mock responses for the GetContent method
	mockS3Client.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123.ref").
		Return(testRef, nil).Once() // Successfully get reference
		
	mockS3Client.On("DownloadFile", mock.Anything, "content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13").
		Return(testData, nil).Once() // Successfully get content
		
	mockS3Client.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123").
		Return(testMetadataJSON, nil).Once() // Successfully get metadata
		
	// Call the method
	content, metadata, err := storage.GetContent(ctx, "owner", "repo", ContentTypeIssue, "123")
	
	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, testData, content)
	assert.NotNil(t, metadata)
	assert.Equal(t, "owner", metadata.Owner)
	assert.Equal(t, "repo", metadata.Repo)
	assert.Equal(t, ContentTypeIssue, metadata.ContentType)
	assert.Equal(t, "123", metadata.ContentID)
	
	mockS3Client.AssertExpectations(t)
	
	// Test error getting reference
	mockS3Client = new(MockGitHubS3Client)
	mockS3Client.On("GetBucketName").Return("test-bucket")
	storage = NewGitHubContentStorage(mockS3Client)
	
	mockS3Client.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123.ref").
		Return(nil, errors.New("reference not found")).Once() // Error getting reference
		
	// Call the method
	content, metadata, err = storage.GetContent(ctx, "owner", "repo", ContentTypeIssue, "123")
	
	// Verify results
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "failed to download content reference")
	
	mockS3Client.AssertExpectations(t)
}

func TestGetContentByURI(t *testing.T) {
	ctx := context.Background()
	mockS3Client := new(MockGitHubS3Client)
	mockS3Client.On("GetBucketName").Return("test-bucket")
	
	storage := NewGitHubContentStorage(mockS3Client)
	
	// Test successful direct content retrieval
	testData := []byte("test-content-data")
	uri := "s3://test-bucket/content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13"
	
	// Mock responses for the GetContentByURI method
	mockS3Client.On("DownloadFile", mock.Anything, "content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13").
		Return(testData, nil).Once() // Successfully get content
		
	mockS3Client.On("ListFiles", mock.Anything, "repositories").
		Return([]string{"repositories/owner/repo/issue/123"}, nil).Once() // List repository objects
		
	mockS3Client.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123").
		Return([]byte(`{"owner":"owner","repo":"repo","content_type":"issue","content_id":"123","checksum":"9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13"}`), nil).Once() // Get metadata
		
	// Call the method
	content, metadata, err := storage.GetContentByURI(ctx, uri)
	
	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, testData, content)
	assert.NotNil(t, metadata)
	assert.Equal(t, "9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13", metadata.Checksum)
	
	mockS3Client.AssertExpectations(t)
	
	// Test invalid URI format
	mockS3Client = new(MockGitHubS3Client)
	mockS3Client.On("GetBucketName").Return("test-bucket")
	storage = NewGitHubContentStorage(mockS3Client)
	
	// Call the method with invalid URI
	content, metadata, err = storage.GetContentByURI(ctx, "invalid-uri")
	
	// Verify results
	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "invalid URI format")
	
	mockS3Client.AssertExpectations(t)
}

func TestDeleteContent(t *testing.T) {
	ctx := context.Background()
	mockS3Client := new(MockGitHubS3Client)
	mockS3Client.On("GetBucketName").Return("test-bucket")
	
	storage := NewGitHubContentStorage(mockS3Client)
	
	// Test successful content deletion
	testRef := []byte("content/9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13")
	
	// Mock responses for the DeleteContent method
	mockS3Client.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123.ref").
		Return(testRef, nil).Once() // Successfully get reference
		
	mockS3Client.On("DeleteFile", mock.Anything, "repositories/owner/repo/issue/123.ref").
		Return(nil).Once() // Successfully delete reference
		
	mockS3Client.On("DeleteFile", mock.Anything, "repositories/owner/repo/issue/123").
		Return(nil).Once() // Successfully delete metadata
		
	// Call the method
	err := storage.DeleteContent(ctx, "owner", "repo", ContentTypeIssue, "123")
	
	// Verify results
	assert.NoError(t, err)
	
	mockS3Client.AssertExpectations(t)
	
	// Test error getting reference
	mockS3Client = new(MockGitHubS3Client)
	mockS3Client.On("GetBucketName").Return("test-bucket")
	storage = NewGitHubContentStorage(mockS3Client)
	
	mockS3Client.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123.ref").
		Return(nil, errors.New("reference not found")).Once() // Error getting reference
		
	// Call the method
	err = storage.DeleteContent(ctx, "owner", "repo", ContentTypeIssue, "123")
	
	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download content reference")
	
	mockS3Client.AssertExpectations(t)
}

func TestListContent(t *testing.T) {
	ctx := context.Background()
	mockS3Client := new(MockGitHubS3Client)
	mockS3Client.On("GetBucketName").Return("test-bucket")
	
	storage := NewGitHubContentStorage(mockS3Client)
	
	// Test successful content listing
	// Mock responses for the ListContent method
	mockS3Client.On("ListFiles", mock.Anything, "repositories/owner/repo/issue/").
		Return([]string{
			"repositories/owner/repo/issue/123",
			"repositories/owner/repo/issue/123.ref",
			"repositories/owner/repo/issue/456",
			"repositories/owner/repo/issue/456.ref",
		}, nil).Once() // Successfully list content
		
	mockS3Client.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/123").
		Return([]byte(`{"owner":"owner","repo":"repo","content_type":"issue","content_id":"123","checksum":"hash1"}`), nil).Once() // Get metadata
		
	mockS3Client.On("DownloadFile", mock.Anything, "repositories/owner/repo/issue/456").
		Return([]byte(`{"owner":"owner","repo":"repo","content_type":"issue","content_id":"456","checksum":"hash2"}`), nil).Once() // Get metadata
		
	// Call the method
	results, err := storage.ListContent(ctx, "owner", "repo", ContentTypeIssue)
	
	// Verify results
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "123", results[0].ContentID)
	assert.Equal(t, "456", results[1].ContentID)
	
	mockS3Client.AssertExpectations(t)
	
	// Test error listing content
	mockS3Client = new(MockGitHubS3Client)
	mockS3Client.On("GetBucketName").Return("test-bucket")
	storage = NewGitHubContentStorage(mockS3Client)
	
	mockS3Client.On("ListFiles", mock.Anything, "repositories/owner/repo/issue/").
		Return(nil, errors.New("list error")).Once() // Error listing content
		
	// Call the method
	results, err = storage.ListContent(ctx, "owner", "repo", ContentTypeIssue)
	
	// Verify results
	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "failed to list content")
	
	mockS3Client.AssertExpectations(t)
}

func TestCalculateContentHash(t *testing.T) {
	// Test content hash calculation
	testData := []byte("test-content-data")
	hash := CalculateContentHash(testData)
	
	// Expected hash for "test-content-data"
	expectedHash := "9801739daae44ec5293d4e1f53d3f4d2d426d91c2a7d6d0da6291a5f5e9f1d13"
	
	assert.Equal(t, expectedHash, hash)
}

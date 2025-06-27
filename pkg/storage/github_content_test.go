package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// GitHubMockS3Client is a mock implementation of S3Client for testing
// Using a different name to avoid conflict with MockS3Client in s3_test.go
type GitHubMockS3Client struct {
	mock.Mock
	bucketName string
}

// GetBucketName returns the mock bucket name
func (m *GitHubMockS3Client) GetBucketName() string {
	return m.bucketName
}

// UploadFile mocks the S3Client.UploadFile method
func (m *GitHubMockS3Client) UploadFile(ctx context.Context, key string, data []byte, contentType string) error {
	args := m.Called(ctx, key, data, contentType)
	return args.Error(0)
}

// DownloadFile mocks the S3Client.DownloadFile method
func (m *GitHubMockS3Client) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

// DeleteFile mocks the S3Client.DeleteFile method
func (m *GitHubMockS3Client) DeleteFile(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

// ListFiles mocks the S3Client.ListFiles method
func (m *GitHubMockS3Client) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// createMockStorage creates a mock S3Client and GitHubContentStorage for testing
// Note: This is a test helper function that uses our GitHubMockS3Client
func createMockStorage(t *testing.T) (*GitHubMockS3Client, *GitHubContentStorage) {
	mockS3Client := &GitHubMockS3Client{
		bucketName: "test-bucket",
	}
	storage := &GitHubContentStorage{
		s3Client:   mockS3Client,
		bucketName: mockS3Client.bucketName,
	}
	return mockS3Client, storage
}

// Test the GitHubContentStorage constructor
func TestNewGitHubContentStorage(t *testing.T) {
	mockS3Client := &GitHubMockS3Client{
		bucketName: "test-bucket",
	}
	storage := NewGitHubContentStorage(mockS3Client)
	assert.NotNil(t, storage)
	assert.Equal(t, "test-bucket", storage.bucketName)
}

// Test the GetS3Client method
func TestGitHubContentStorage_GetS3Client(t *testing.T) {
	mockS3Client, storage := createMockStorage(t)

	result := storage.GetS3Client()

	assert.Equal(t, mockS3Client, result)
}

// Test the StoreContent method with new content
func TestGitHubContentStorage_StoreContent_NewContent(t *testing.T) {
	mockS3Client, storage := createMockStorage(t)
	ctx := context.Background()

	// Test data
	testData := []byte("test content")
	testMetadata := map[string]interface{}{"test": "value"}

	// Calculate expected hash
	expectedHash := sha256.Sum256(testData)
	expectedHashStr := hex.EncodeToString(expectedHash[:])

	// Setup mock behavior
	mockS3Client.On("ListFiles", ctx, fmt.Sprintf("content/%s", expectedHashStr)).Return([]string{}, nil)
	mockS3Client.On("UploadFile", ctx, fmt.Sprintf("content/%s", expectedHashStr), testData, "application/octet-stream").Return(nil)

	// For the metadata JSON - ensure metadata is never nil to fix PostgreSQL empty string issue
	// We don't need to store metadataBytes locally since it's only used in the call expectations
	// This implements the fix for the PostgreSQL empty string issue mentioned in memories
	mockS3Client.On("UploadFile", ctx, "repositories/owner/repo/issue/123", mock.AnythingOfType("[]uint8"), "application/json").Return(nil)

	// For the reference file
	mockS3Client.On("UploadFile", ctx, "repositories/owner/repo/issue/123.ref", mock.AnythingOfType("[]uint8"), "text/plain").Return(nil)

	// Call the method under test
	result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "owner", result.Owner)
	assert.Equal(t, "repo", result.Repo)
	assert.Equal(t, ContentTypeIssue, result.ContentType)
	assert.Equal(t, "123", result.ContentID)
	assert.Equal(t, int64(len(testData)), result.Size)
	assert.Equal(t, testMetadata, result.Metadata)

	// Verify all expectations were met
	mockS3Client.AssertExpectations(t)
}

// Test the StoreContent method with existing content (deduplication)
func TestGitHubContentStorage_StoreContent_ExistingContent(t *testing.T) {
	mockS3Client, storage := createMockStorage(t)
	ctx := context.Background()

	// Test data
	testData := []byte("test content")
	testMetadata := map[string]interface{}{"test": "value"}

	// Calculate expected hash
	expectedHash := sha256.Sum256(testData)
	expectedHashStr := hex.EncodeToString(expectedHash[:])

	// Setup mock behavior - content already exists
	mockS3Client.On("ListFiles", ctx, fmt.Sprintf("content/%s", expectedHashStr)).Return([]string{fmt.Sprintf("content/%s", expectedHashStr)}, nil)

	// For the metadata JSON - still needs to be stored regardless of content existence
	// This is crucial for the PostgreSQL metadata JSON handling fix
	mockS3Client.On("UploadFile", ctx, "repositories/owner/repo/issue/123", mock.AnythingOfType("[]uint8"), "application/json").Return(nil)

	// For the reference file
	mockS3Client.On("UploadFile", ctx, "repositories/owner/repo/issue/123.ref", mock.AnythingOfType("[]uint8"), "text/plain").Return(nil)

	// Call the method under test
	result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify all expectations were met
	mockS3Client.AssertExpectations(t)
}

// Test error handling when listing files fails
func TestGitHubContentStorage_StoreContent_ListFilesError(t *testing.T) {
	mockS3Client, storage := createMockStorage(t)
	ctx := context.Background()

	// Test data
	testData := []byte("test content")
	testMetadata := map[string]interface{}{"test": "value"}

	// Calculate expected hash
	expectedHash := sha256.Sum256(testData)
	expectedHashStr := hex.EncodeToString(expectedHash[:])

	// Setup mock behavior - error listing files
	mockS3Client.On("ListFiles", ctx, fmt.Sprintf("content/%s", expectedHashStr)).Return(nil, errors.New("list error"))

	// Call the method under test
	result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list error")

	// Verify all expectations were met
	mockS3Client.AssertExpectations(t)
}

// Test error handling when uploading content fails
func TestGitHubContentStorage_StoreContent_UploadError(t *testing.T) {
	mockS3Client, storage := createMockStorage(t)
	ctx := context.Background()

	// Test data
	testData := []byte("test content")
	testMetadata := map[string]interface{}{"test": "value"}

	// Calculate expected hash
	expectedHash := sha256.Sum256(testData)
	expectedHashStr := hex.EncodeToString(expectedHash[:])

	// Setup mock behavior
	mockS3Client.On("ListFiles", ctx, fmt.Sprintf("content/%s", expectedHashStr)).Return([]string{}, nil)
	mockS3Client.On("UploadFile", ctx, fmt.Sprintf("content/%s", expectedHashStr), testData, "application/octet-stream").Return(errors.New("upload error"))

	// Call the method under test
	result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "upload error")

	// Verify all expectations were met
	mockS3Client.AssertExpectations(t)
}

// Test error handling when uploading metadata fails
func TestGitHubContentStorage_StoreContent_MetadataError(t *testing.T) {
	mockS3Client, storage := createMockStorage(t)
	ctx := context.Background()

	// Test data
	testData := []byte("test content")
	testMetadata := map[string]interface{}{"test": "value"}

	// Calculate expected hash
	hasher := sha256.New()
	hasher.Write(testData)
	expectedHash := hasher.Sum(nil)
	expectedHashStr := hex.EncodeToString(expectedHash)

	// Setup mock behavior
	mockS3Client.On("ListFiles", ctx, fmt.Sprintf("content/%s", expectedHashStr)).Return([]string{}, nil)
	mockS3Client.On("UploadFile", ctx, fmt.Sprintf("content/%s", expectedHashStr), testData, "application/octet-stream").Return(nil)

	// For the metadata JSON - simulate an error during metadata upload
	// This is particularly important as it tests the error handling for the PostgreSQL metadata issue
	mockS3Client.On("UploadFile", ctx, "repositories/owner/repo/issue/123", mock.AnythingOfType("[]uint8"), "application/json").Return(errors.New("metadata error"))

	// Call the method under test
	result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "metadata error")

	// Verify all expectations were met
	mockS3Client.AssertExpectations(t)
}

// Test that we can store new content and create reference file with empty metadata
func TestGitHubContentStorage_StoreContent_EmptyMetadata(t *testing.T) {
	ctx := context.Background()
	testData := []byte("test content")
	// Empty metadata should be properly handled - using nil to test the empty metadata case
	// This tests the fix for the PostgreSQL error where empty metadata was causing "invalid input syntax for type json"
	var testMetadata map[string]interface{} = nil

	// Calculate expected hash
	hasher := sha256.New()
	hasher.Write(testData)
	expectedHash := hasher.Sum(nil)
	expectedHashStr := hex.EncodeToString(expectedHash)

	// Create mock and storage
	mockS3Client, storage := createMockStorage(t)

	// Set up expectations
	mockS3Client.On("ListFiles", ctx, fmt.Sprintf("content/%s", expectedHashStr)).Return([]string{}, nil)
	mockS3Client.On("UploadFile", ctx, fmt.Sprintf("content/%s", expectedHashStr), testData, "application/octet-stream").Return(nil)

	// The actual test will use the testMetadata variable in the StoreContent call below

	// For the metadata JSON - ensure we use '{}' instead of empty string ''
	// This is a key test for the PostgreSQL empty string issue mentioned in your memories
	// The empty metadata should be converted to {} instead of '' to prevent PostgreSQL errors
	mockS3Client.On("UploadFile", ctx, "repositories/owner/repo/issue/123", mock.AnythingOfType("[]uint8"), "application/json").
		Run(func(args mock.Arguments) {
			// Verify that the empty metadata field is properly initialized as {} not ""
			metadataBytes := args.Get(2).([]byte)
			// Unmarshal the ContentMetadata object
			var contentMetadata ContentMetadata
			err := json.Unmarshal(metadataBytes, &contentMetadata)
			assert.NoError(t, err, "Should be able to unmarshal metadata JSON")
			// Verify that Metadata field exists and is an empty map, not nil
			assert.NotNil(t, contentMetadata.Metadata, "Metadata field should not be nil")
			assert.Empty(t, contentMetadata.Metadata, "Metadata field should be empty map")
		}).Return(nil)

	// For the reference file
	mockS3Client.On("UploadFile", ctx, "repositories/owner/repo/issue/123.ref", mock.AnythingOfType("[]uint8"), "text/plain").Return(nil)

	// Call the method under test - using testMetadata (empty string) that should be converted to {}
	result, err := storage.StoreContent(ctx, "owner", "repo", ContentTypeIssue, "123", testData, testMetadata)

	// Verify results
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Debug the metadata value
	fmt.Printf("Original metadata type: %T\n", result.Metadata)
	fmt.Printf("Original metadata value: %v\n", result.Metadata)

	// Use the GetMetadata method that ensures safe handling of nil metadata
	safeMetadata := result.GetMetadata()
	fmt.Printf("Safe metadata type: %T\n", safeMetadata)
	fmt.Printf("Safe metadata value: %v\n", safeMetadata)

	// Now we can safely check that it's an empty map
	assert.NotNil(t, safeMetadata, "GetMetadata() should never return nil")
	assert.Equal(t, 0, len(safeMetadata), "Metadata should be an empty map")

	// Verify all expectations were met
	mockS3Client.AssertExpectations(t)
}

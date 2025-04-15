package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockS3Client is a mock for the AWS S3 client
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.DeleteObjectOutput), args.Error(1)
}

// Mock for ListObjectsV2
func (m *MockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.ListObjectsV2Output), args.Error(1)
}

// MockS3Paginator is a mock for the S3 paginator
type MockS3Paginator struct {
	mock.Mock
	currentPage int
	pages       []*s3.ListObjectsV2Output
}

func (m *MockS3Paginator) HasMorePages() bool {
	return m.currentPage < len(m.pages)
}

func (m *MockS3Paginator) NextPage(ctx context.Context, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if !m.HasMorePages() {
		return nil, errors.New("no more pages")
	}
	page := m.pages[m.currentPage]
	m.currentPage++
	return page, nil
}

func TestGetBucketName(t *testing.T) {
	config := S3Config{
		Bucket: "test-bucket",
	}
	
	client := &S3Client{
		config: config,
	}
	
	assert.Equal(t, "test-bucket", client.GetBucketName())
}

func TestDeleteFile(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockS3Client)
	
	config := S3Config{
		Bucket:         "test-bucket",
		RequestTimeout: 5 * time.Second,
	}
	
	s3Client := &S3Client{
		client: mockClient,
		config: config,
	}
	
	// Test successful delete
	mockClient.On("DeleteObject", mock.Anything, &s3.DeleteObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key"),
	}).Return(&s3.DeleteObjectOutput{}, nil).Once()
	
	err := s3Client.DeleteFile(ctx, "test-key")
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
	
	// Test delete error
	mockClient.On("DeleteObject", mock.Anything, &s3.DeleteObjectInput{
		Bucket: aws.String("test-bucket"),
		Key:    aws.String("test-key"),
	}).Return(nil, errors.New("delete error")).Once()
	
	err = s3Client.DeleteFile(ctx, "test-key")
	assert.Error(t, err)
	assert.Equal(t, "delete error", err.Error())
	mockClient.AssertExpectations(t)
	
	// Test empty key
	err = s3Client.DeleteFile(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key cannot be empty")
}

func TestListFiles(t *testing.T) {
	ctx := context.Background()
	mockClient := new(MockS3Client)
	
	config := S3Config{
		Bucket:         "test-bucket",
		RequestTimeout: 5 * time.Second,
	}
	
	s3Client := &S3Client{
		client: mockClient,
		config: config,
	}
	
	// Set up mock paginator results
	page1 := &s3.ListObjectsV2Output{
		Contents: []s3.Object{
			{Key: aws.String("prefix/file1.txt")},
			{Key: aws.String("prefix/file2.txt")},
		},
	}
	
	page2 := &s3.ListObjectsV2Output{
		Contents: []s3.Object{
			{Key: aws.String("prefix/file3.txt")},
		},
	}
	
	// Set up mock paginator
	paginator := &MockS3Paginator{
		pages: []*s3.ListObjectsV2Output{page1, page2},
	}
	
	// Replace the standard paginator with our mock
	originalPaginator := s3.NewListObjectsV2Paginator
	defer func() {
		s3.NewListObjectsV2Paginator = originalPaginator
	}()
	
	// Replace with mock function
	s3.NewListObjectsV2Paginator = func(client s3.ListObjectsV2API, params *s3.ListObjectsV2Input, optFns ...func(*s3.ListObjectsV2PaginatorOptions)) *s3.ListObjectsV2Paginator {
		// This is a simplified mock that just returns our paginator
		assert.Equal(t, "test-bucket", *params.Bucket)
		assert.Equal(t, "prefix", *params.Prefix)
		return &s3.ListObjectsV2Paginator{
			HasMorePages: paginator.HasMorePages,
			NextPage: func(ctx context.Context, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return paginator.NextPage(ctx, optFns...)
			},
		}
	}
	
	// Test list files
	keys, err := s3Client.ListFiles(ctx, "prefix")
	assert.NoError(t, err)
	assert.Len(t, keys, 3)
	assert.Equal(t, "prefix/file1.txt", keys[0])
	assert.Equal(t, "prefix/file2.txt", keys[1])
	assert.Equal(t, "prefix/file3.txt", keys[2])
}

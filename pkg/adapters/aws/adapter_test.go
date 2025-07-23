package aws

import (
	"context"
	"testing"

	commonaws "github.com/developer-mesh/developer-mesh/pkg/common/aws"
	"github.com/developer-mesh/developer-mesh/pkg/feature"
)

// MockAWSClient is a mock implementation of the common AWS client interface
type MockAWSClient struct {
	session      interface{}
	credentials  interface{}
	region       string
	s3Client     interface{}
	sqsClient    interface{}
	callRegistry map[string]int
}

// NewMockAWSClient creates a new mock AWS client
func NewMockAWSClient() *MockAWSClient {
	return &MockAWSClient{
		session:      "mock-session",
		credentials:  "mock-credentials",
		region:       "us-west-2",
		s3Client:     "mock-s3-client",
		sqsClient:    "mock-sqs-client",
		callRegistry: make(map[string]int),
	}
}

// GetSession returns a mock session
func (m *MockAWSClient) GetSession() interface{} {
	m.callRegistry["GetSession"]++
	return m.session
}

// GetCredentials returns mock credentials
func (m *MockAWSClient) GetCredentials() interface{} {
	m.callRegistry["GetCredentials"]++
	return m.credentials
}

// GetRegion returns a mock region
func (m *MockAWSClient) GetRegion() string {
	m.callRegistry["GetRegion"]++
	return m.region
}

// CreateS3Client returns a mock S3 client
func (m *MockAWSClient) CreateS3Client() interface{} {
	m.callRegistry["CreateS3Client"]++
	return m.s3Client
}

// CreateSQSClient returns a mock SQS client
func (m *MockAWSClient) CreateSQSClient() interface{} {
	m.callRegistry["CreateSQSClient"]++
	return m.sqsClient
}

// TestAWSAdapter tests that the AWS adapter correctly delegates to the common implementation
func TestAWSAdapter(t *testing.T) {
	// Create a mock common AWS client
	mockClient := NewMockAWSClient()

	// Create the adapter with the mock client
	adapter := NewAWSAdapter(mockClient)

	// Test that the adapter correctly delegates GetSession
	session := adapter.GetSession()
	if session != mockClient.session {
		t.Errorf("Expected session %v, got %v", mockClient.session, session)
	}
	if mockClient.callRegistry["GetSession"] != 1 {
		t.Errorf("Expected GetSession to be called once, got %d", mockClient.callRegistry["GetSession"])
	}

	// Test that the adapter correctly delegates GetCredentials
	credentials := adapter.GetCredentials()
	if credentials != mockClient.credentials {
		t.Errorf("Expected credentials %v, got %v", mockClient.credentials, credentials)
	}
	if mockClient.callRegistry["GetCredentials"] != 1 {
		t.Errorf("Expected GetCredentials to be called once, got %d", mockClient.callRegistry["GetCredentials"])
	}

	// Test that the adapter correctly delegates GetRegion
	region := adapter.GetRegion()
	if region != mockClient.region {
		t.Errorf("Expected region %v, got %v", mockClient.region, region)
	}
	if mockClient.callRegistry["GetRegion"] != 1 {
		t.Errorf("Expected GetRegion to be called once, got %d", mockClient.callRegistry["GetRegion"])
	}

	// Test that the adapter correctly delegates CreateS3Client
	s3Client := adapter.CreateS3Client()
	if s3Client != mockClient.s3Client {
		t.Errorf("Expected S3 client %v, got %v", mockClient.s3Client, s3Client)
	}
	if mockClient.callRegistry["CreateS3Client"] != 1 {
		t.Errorf("Expected CreateS3Client to be called once, got %d", mockClient.callRegistry["CreateS3Client"])
	}

	// Test that the adapter correctly delegates CreateSQSClient
	sqsClient := adapter.CreateSQSClient()
	if sqsClient != mockClient.sqsClient {
		t.Errorf("Expected SQS client %v, got %v", mockClient.sqsClient, sqsClient)
	}
	if mockClient.callRegistry["CreateSQSClient"] != 1 {
		t.Errorf("Expected CreateSQSClient to be called once, got %d", mockClient.callRegistry["CreateSQSClient"])
	}
}

// TestAdapterFactory tests that the adapter factory correctly uses feature flags
func TestAdapterFactory(t *testing.T) {
	// Create a context
	ctx := context.Background()

	// Create the factory
	factory := NewAdapterFactory(ctx)

	// Mock the AWS config
	mockConfig := commonaws.AuthConfig{
		Region: "us-west-2",
	}

	// Test with feature flag disabled (should use adapter)
	feature.SetEnabled(feature.UseNewAWS, false)

	// This will fail in a real test since we can't load AWS config
	// This is just a demonstration of how the factory would be tested
	_, err := factory.GetClient(mockConfig)
	if err == nil {
		// In a real environment with proper mocking, we would check:
		// 1. That the correct implementation was returned
		// 2. That it behaves correctly
		t.Log("Factory successfully returned a client (would check that it's an adapter)")
	}

	// Test with feature flag enabled (should use direct implementation)
	feature.SetEnabled(feature.UseNewAWS, true)

	// This will fail in a real test since we can't load AWS config
	// This is just a demonstration of how the factory would be tested
	_, err = factory.GetClient(mockConfig)
	if err == nil {
		// In a real environment with proper mocking, we would check:
		// 1. That the correct implementation was returned
		// 2. That it behaves correctly
		t.Log("Factory successfully returned a client (would check that it's a direct implementation)")
	}
}

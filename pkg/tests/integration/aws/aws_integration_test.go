package aws

import (
	"context"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/common/aws"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/tests/integration"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWSIntegration(t *testing.T) {
	helper := integration.NewTestHelper(t)
	_ = helper // Using in future tests

	// Create logger for AWS services
	logger := observability.NewLogger("aws-test")
	require.NotNil(t, logger)

	t.Run("AWS authentication and configuration integration", func(t *testing.T) {
		// Create AWS config
		awsConfig, err := aws.GetAWSConfig(context.Background(), aws.AuthConfig{
			Region: "us-east-1",
			// Using mock credentials for integration testing - use Endpoint for localstack
			Endpoint: "http://localhost:4566",
		})

		// In a real integration test environment, this would connect to localstack
		// For this test, we're just verifying the code path works without error
		if err != nil {
			t.Skip("Skipping AWS config test - localstack likely not available")
		}

		require.NotNil(t, awsConfig)

		// Verify region was properly set
		assert.Equal(t, "us-east-1", awsConfig.Region)
	})

	t.Run("AWS service client integration with observability", func(t *testing.T) {
		// This test verifies that AWS clients can be created with proper logging integration
		// Create config for testing
		mockConfig, err := aws.GetAWSConfig(context.Background(), aws.AuthConfig{
			Region: "us-east-1",
			// Use Endpoint to specify localstack URL
			Endpoint: "http://localhost:4566",
		})

		if err != nil {
			t.Skip("Skipping AWS client test - localstack likely not available")
		}

		// Create S3 client - this tests that the AWS package correctly integrates
		// with the AWS SDK and our observability stack
		s3Client := s3.NewFromConfig(mockConfig)
		require.NotNil(t, s3Client)

		// Create RDS client through our package
		rdsClient, err := aws.NewExtendedRDSClient(context.Background(), aws.RDSConnectionConfig{
			// Set required fields for RDSConnectionConfig
			Host:     "localhost",
			Port:     5432,
			Database: "test",
			Username: "test",
			// Set Auth config nested inside RDSConnectionConfig
			Auth: aws.AuthConfig{
				Region:   "us-east-1",
				Endpoint: "http://localhost:4566",
			},
		})

		if err != nil {
			t.Skip("Skipping RDS client test - localstack likely not available")
		}

		require.NotNil(t, rdsClient)

		// Test IRSA detection (this should use the mock path in test environment)
		isIRSA := aws.IsIRSAEnabled()
		// We don't assert a specific value, we just verify the function runs without error
		t.Logf("IRSA enabled: %v", isIRSA)
	})
}

// Package aws provides adapters for AWS services to support the migration process
// between legacy interfaces and the new common/aws implementation.
package aws

import (
	"context"

	commonaws "github.com/developer-mesh/developer-mesh/pkg/common/aws"
	"github.com/developer-mesh/developer-mesh/pkg/feature"
	"github.com/aws/aws-sdk-go-v2/config"
)

// LegacyAWSInterface represents the legacy interface expected by client code
type LegacyAWSInterface interface {
	GetSession() any
	GetCredentials() any
	GetRegion() string
	CreateS3Client() any
	CreateSQSClient() any
}

// AWSAdapter adapts between legacy AWS interface and the new common AWS interface
// following the successful adapter pattern used in the vector API implementation.
type AWSAdapter struct {
	commonClient commonaws.AWSClient
}

// NewAWSAdapter creates a new adapter that implements the legacy interface
// while delegating to the new common AWS implementation
func NewAWSAdapter(commonClient commonaws.AWSClient) *AWSAdapter {
	return &AWSAdapter{
		commonClient: commonClient,
	}
}

// GetSession implements the legacy interface but delegates to the new implementation
func (a *AWSAdapter) GetSession() any {
	// The adapter pattern handles potential differences in return types
	return a.commonClient.GetSession()
}

// GetCredentials implements the legacy interface but delegates to the new implementation
func (a *AWSAdapter) GetCredentials() any {
	return a.commonClient.GetCredentials()
}

// GetRegion implements the legacy interface but delegates to the new implementation
func (a *AWSAdapter) GetRegion() string {
	return a.commonClient.GetRegion()
}

// CreateS3Client implements the legacy interface but delegates to the new implementation
func (a *AWSAdapter) CreateS3Client() any {
	return a.commonClient.CreateS3Client()
}

// CreateSQSClient implements the legacy interface but delegates to the new implementation
func (a *AWSAdapter) CreateSQSClient() any {
	return a.commonClient.CreateSQSClient()
}

// AdapterFactory provides a way to create adapters based on feature flags
type AdapterFactory struct {
	ctx context.Context
}

// NewAdapterFactory creates a new factory for AWS adapters
func NewAdapterFactory(ctx context.Context) *AdapterFactory {
	return &AdapterFactory{
		ctx: ctx,
	}
}

// GetClient returns either a legacy AWS client or the new implementation
// based on feature flags
func (f *AdapterFactory) GetClient(cfg commonaws.AuthConfig) (LegacyAWSInterface, error) {
	// Check feature flag to determine which implementation to use
	if feature.IsEnabled(feature.UseNewAWS) {
		// Create the new implementation directly
		awsConfig, err := config.LoadDefaultConfig(f.ctx)
		if err != nil {
			return nil, err
		}

		commonClient := commonaws.NewAWSClient(f.ctx, &awsConfig)
		return commonClient, nil
	}

	// Create the common implementation and wrap it in an adapter
	awsConfig, err := config.LoadDefaultConfig(f.ctx)
	if err != nil {
		return nil, err
	}

	commonClient := commonaws.NewAWSClient(f.ctx, &awsConfig)
	return NewAWSAdapter(commonClient), nil
}

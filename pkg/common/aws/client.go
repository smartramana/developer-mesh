// Package aws provides core AWS functionality for the Go workspace.
package aws

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// AWSClient provides a standard interface for AWS clients
type AWSClient interface {
	GetSession() any
	GetCredentials() any
	GetRegion() string
	CreateS3Client() any
	CreateSQSClient() any
}

// LegacyAuthConfig wraps AWS authentication configuration (legacy version)
type LegacyAuthConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Profile         string
	Endpoint        string
}

// RDSConfig holds configuration for RDS
type RDSConfig struct {
	Region     string
	SecretName string
}

// RDSClient is a client for AWS RDS
type RDSClient struct {
	Config *aws.Config
}

// StandardAWSClient implements the AWSClient interface with standard AWS functionality
type StandardAWSClient struct {
	ctx       context.Context
	awsConfig *aws.Config
	region    string
	session   any
	s3Client  *s3.Client
	sqsClient *sqs.Client
}

// NewAWSClient creates a new AWS client with the provided config
func NewAWSClient(ctx context.Context, cfg *aws.Config) AWSClient {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-2" // Default region
	}

	return &StandardAWSClient{
		ctx:       ctx,
		awsConfig: cfg,
		region:    region,
	}
}

// GetSession returns the AWS session
func (c *StandardAWSClient) GetSession() any {
	return c.session
}

// GetCredentials returns the AWS credentials
func (c *StandardAWSClient) GetCredentials() any {
	if c.awsConfig == nil {
		return nil
	}
	return c.awsConfig.Credentials
}

// GetRegion returns the AWS region
func (c *StandardAWSClient) GetRegion() string {
	return c.region
}

// CreateS3Client creates and returns an S3 client
func (c *StandardAWSClient) CreateS3Client() any {
	if c.s3Client == nil {
		c.s3Client = s3.NewFromConfig(*c.awsConfig)
	}
	return c.s3Client
}

// CreateSQSClient creates and returns an SQS client
func (c *StandardAWSClient) CreateSQSClient() any {
	if c.sqsClient == nil {
		c.sqsClient = sqs.NewFromConfig(*c.awsConfig)
	}
	return c.sqsClient
}

// GetAWSConfigLegacy is a deprecated alias for LegacyGetAWSConfig in adapter.go
// It's kept here temporarily for backward compatibility
func GetAWSConfigLegacy(ctx context.Context, cfg LegacyAuthConfig) (any, error) {
	// Convert from legacy config to new config format
	newCfg := AuthConfig{
		Region:     cfg.Region,
		Endpoint:   cfg.Endpoint,
		AssumeRole: "", // Not present in legacy format
	}
	return LegacyGetAWSConfig(ctx, newCfg)
}

// NewRDSClient creates a new RDS client
func NewRDSClient(ctx context.Context, cfg RDSConfig) (*RDSClient, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, err
	}

	return &RDSClient{
		Config: &awsConfig,
	}, nil
}

// IsIRSAEnabledLegacy is a deprecated alias for IsIRSAEnabled in auth.go
// It's kept here temporarily for backward compatibility
func IsIRSAEnabledLegacy() bool {
	return IsIRSAEnabled()
}

package aws

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// AuthConfig holds the AWS authentication configuration options
type AuthConfig struct {
	Region    string `mapstructure:"region"`
	Endpoint  string `mapstructure:"endpoint"`
	AssumeRole string `mapstructure:"assume_role"`
}

// GetAWSConfig creates an AWS SDK configuration with IRSA support
func GetAWSConfig(ctx context.Context, cfg AuthConfig) (aws.Config, error) {
	// Create the AWS config options
	options := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	// Add custom endpoint if specified (for local development or testing)
	if cfg.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				HostnameImmutable: true,
				SigningRegion:     cfg.Region,
			}, nil
		})
		options = append(options, config.WithEndpointResolverWithOptions(customResolver))
	}

	// Check if running in a Kubernetes pod with a service account
	// The AWS SDK automatically detects and uses IRSA if the required environment variables are set
	// These environment variables are set automatically by the EKS Pod Identity Agent
	_, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
	_, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")

	if hasWebIdentityTokenFile && hasRoleArn {
		log.Println("Using IRSA authentication for AWS services")
	} else {
		log.Println("Using standard AWS credential provider chain")
	}

	// Load the AWS configuration
	return config.LoadDefaultConfig(ctx, options...)
}

// IsIRSAEnabled checks if IRSA is configured and available
func IsIRSAEnabled() bool {
	_, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
	_, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")
	return hasWebIdentityTokenFile && hasRoleArn
}

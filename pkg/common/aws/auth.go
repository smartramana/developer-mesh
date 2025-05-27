package aws

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	stscreds "github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AuthConfig holds the AWS authentication configuration options
type AuthConfig struct {
	Region     string `mapstructure:"region"`
	Endpoint   string `mapstructure:"endpoint"`
	AssumeRole string `mapstructure:"assume_role"`
}

// GetAWSConfig creates an AWS SDK configuration with IRSA support
// If AssumeRole is set, it will use STS to assume the specified role and return a config with temporary credentials
func GetAWSConfig(ctx context.Context, cfg AuthConfig) (aws.Config, error) {
	// Create the AWS config options
	options := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	// Add custom endpoint if specified (for local development or testing)
	if cfg.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
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

	// If AssumeRole is set, use STS to assume the role and inject credentials
	if cfg.AssumeRole != "" {
		log.Printf("Assuming IAM role: %s", cfg.AssumeRole)
		awsCfg, err := config.LoadDefaultConfig(ctx, options...)
		if err != nil {
			return aws.Config{}, err
		}
		stsClient := sts.NewFromConfig(awsCfg)
		creds := stscreds.NewAssumeRoleProvider(stsClient, cfg.AssumeRole)
		awsCfg.Credentials = aws.NewCredentialsCache(creds)
		return awsCfg, nil
	}
	// Load the AWS configuration (default provider chain, no assume role)
	return config.LoadDefaultConfig(ctx, options...)
}

// AssumeRoleProvider uses STS to assume the specified IAM role and returns a credentials provider
func AssumeRoleProvider(ctx context.Context, awsCfg aws.Config, roleArn string) aws.CredentialsProvider {
	stsClient := sts.NewFromConfig(awsCfg)
	return aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsClient, roleArn))
}

// IsIRSAEnabled checks if IRSA is configured and available
func IsIRSAEnabled() bool {
	_, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
	_, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")
	return hasWebIdentityTokenFile && hasRoleArn
}

package aws

import (
	"context"
	"fmt"

	storage "github.com/S-Corkum/devops-mcp/pkg/storage"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config is now an alias for storage.S3Config; see storage package for definition.
type S3Config = storage.S3Config

// AWSConfig is an alias for storage.AWSConfig, allowing external packages to use aws.AWSConfig
// for consistency and encapsulation.
type AWSConfig = storage.AWSConfig

// NewS3Client creates a new S3 client with IRSA support, returning the canonical storage.S3Client

func NewS3Client(ctx context.Context, cfg S3Config) (*storage.S3Client, error) {
	// Always use IAM authentication if enabled, and pass AssumeRole if set
	if cfg.AWSConfig.UseIAMAuth {
		_, err := GetAWSConfig(ctx, AuthConfig{
			Region:     cfg.AWSConfig.Region,
			Endpoint:   cfg.AWSConfig.Endpoint,
			AssumeRole: cfg.AWSConfig.AssumeRole, // This now supports role_arn
		})
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config with IAM auth: %w", err)
		}
	}

	// Create S3 client with options
	s3Options := []func(*s3.Options){}
	if cfg.ForcePathStyle {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	// Use the canonical storage.NewS3Client constructor to create the S3 client.
	s3Client, err := storage.NewS3Client(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}
	return s3Client, nil
}

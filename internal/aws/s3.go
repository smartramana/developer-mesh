package aws

import (
	"context"
	"fmt"
	"log"
	"os"




	"github.com/aws/aws-sdk-go-v2/service/s3"
	storage "github.com/S-Corkum/mcp-server/internal/storage"
)

// S3Config is now an alias for storage.S3Config; see storage package for definition.
type S3Config = storage.S3Config
// AWSConfig is an alias for storage.AWSConfig, allowing external packages to use aws.AWSConfig
// for consistency and encapsulation.
type AWSConfig = storage.AWSConfig





// NewS3Client creates a new S3 client with IRSA support, returning the canonical storage.S3Client


func NewS3Client(ctx context.Context, cfg S3Config) (*storage.S3Client, error) {
	// Get AWS configuration with IRSA support when IAM auth is enabled
	var err error
	
	// Always try to use IAM authentication by default, unless explicitly disabled
	// This ensures we follow the principle of least privilege
	useIAM := cfg.AWSConfig.UseIAMAuth
	
	if useIAM {
		_, err = GetAWSConfig(ctx, AuthConfig{
			Region:     cfg.AWSConfig.Region,
			Endpoint:   cfg.AWSConfig.Endpoint,
			AssumeRole: cfg.AWSConfig.AssumeRole,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config with IAM auth: %w", err)
		}
		
		if IsIRSAEnabled() {
			log.Println("Using IRSA authentication for S3 with role:", os.Getenv("AWS_ROLE_ARN"))
		} else {
			log.Println("Using standard IAM authentication for S3 (IRSA not detected)")
		}
	} else {
		// Only fall back to basic AWS config when IAM auth is explicitly disabled.
		// If IAM auth is disabled, just proceed to create the S3 client using the storage package.
	}

	// Create S3 client with options
	s3Options := []func(*s3.Options){}
	// Force path style if required (for LocalStack or other S3-compatible services)
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


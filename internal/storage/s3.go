package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
)

// S3Client is a client for AWS S3
type Uploader interface {
	Upload(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

type Downloader interface {
	Download(ctx context.Context, w io.WriterAt, params *s3.GetObjectInput, optFns ...func(*manager.Downloader)) (int64, error)
}

type S3API interface {
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type S3Client struct {
	client     S3API
	uploader   Uploader
	downloader Downloader
	config     S3Config
}

// GetBucketName returns the bucket name from the configuration
func (c *S3Client) GetBucketName() string {
	return c.config.Bucket
}

// AWSConfig holds AWS-specific configuration for authentication
type AWSConfig struct {
	UseIAMAuth bool   `mapstructure:"use_iam_auth"`
	Region     string `mapstructure:"region"`
	Endpoint   string `mapstructure:"endpoint"`
	AssumeRole string `mapstructure:"assume_role"`
}

// S3Config holds configuration for the S3 client
type S3Config struct {
	Region           string        `mapstructure:"region"`
	Bucket           string        `mapstructure:"bucket"`
	Endpoint         string        `mapstructure:"endpoint"`
	ForcePathStyle   bool          `mapstructure:"force_path_style"`
	UploadPartSize   int64         `mapstructure:"upload_part_size"`
	DownloadPartSize int64         `mapstructure:"download_part_size"`
	Concurrency      int           `mapstructure:"concurrency"`
	RequestTimeout   time.Duration `mapstructure:"request_timeout"`
	AWSConfig        AWSConfig     `mapstructure:"aws_config"`
}

// NewS3Client creates a new S3 client with IRSA support
func NewS3Client(ctx context.Context, cfg S3Config) (*S3Client, error) {
	// Create config options
	var options []func(*config.LoadOptions) error
	
	// Use region from AWS config if specified, otherwise use the S3 config region
	region := cfg.Region
	if cfg.AWSConfig.Region != "" {
		region = cfg.AWSConfig.Region
	}
	options = append(options, config.WithRegion(region))
	
	// Add custom endpoint if specified (for LocalStack or other S3 compatible services)
	endpoint := cfg.Endpoint
	if cfg.AWSConfig.Endpoint != "" {
		endpoint = cfg.AWSConfig.Endpoint
	}
	
	if endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               endpoint,
				HostnameImmutable: true,
				SigningRegion:     region,
			}, nil
		})
		options = append(options, config.WithEndpointResolverWithOptions(customResolver))
	}
	
	// Load AWS configuration - IRSA will be automatically detected if AWS_WEB_IDENTITY_TOKEN_FILE 
	// and AWS_ROLE_ARN environment variables are set by the EKS Pod Identity Agent
	awsCfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	
	// Log IRSA detection if enabled
	if cfg.AWSConfig.UseIAMAuth {
		// Check if IRSA environment variables are set
		_, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
		_, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")
		
		if hasWebIdentityTokenFile && hasRoleArn {
			fmt.Println("Using IRSA (IAM Roles for Service Accounts) authentication for S3")
		} else {
			fmt.Println("Warning: IAM authentication is enabled but IRSA environment variables are not set")
		}
	}
	
	// Create S3 client options
	s3Options := []func(*s3.Options){}
	
	// Force path style if required (for LocalStack)
	if cfg.ForcePathStyle {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, s3Options...)

	// Create uploader and downloader with optimized settings
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = cfg.UploadPartSize
		u.Concurrency = cfg.Concurrency
	})

	downloader := manager.NewDownloader(client, func(d *manager.Downloader) {
		d.PartSize = cfg.DownloadPartSize
		d.Concurrency = cfg.Concurrency
	})

	return &S3Client{
		client:     client, // *s3.Client implements S3API
		uploader:   uploader, // manager.Uploader implements Uploader
		downloader: downloader, // manager.Downloader implements Downloader
		config:     cfg,
	}, nil
}

// UploadFile uploads a file to S3
func (c *S3Client) UploadFile(ctx context.Context, key string, data []byte, contentType string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	
	if len(data) == 0 {
		return fmt.Errorf("data cannot be empty")
	}

	// Create upload input
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.config.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}

	// Upload with context timeout
	ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()

	_, err := c.uploader.Upload(ctx, input)
	return err
}

// DownloadFile downloads a file from S3
func (c *S3Client) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	// Create buffer to write the file to
	buf := manager.NewWriteAtBuffer([]byte{})

	// Create download input
	input := &s3.GetObjectInput{
		Bucket: aws.String(c.config.Bucket),
		Key:    aws.String(key),
	}

	// Download with context timeout
	ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()

	_, err := c.downloader.Download(ctx, buf, input)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeleteFile deletes a file from S3
func (c *S3Client) DeleteFile(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Create delete input
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(c.config.Bucket),
		Key:    aws.String(key),
	}

	// Delete with context timeout
	ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()

	_, err := c.client.DeleteObject(ctx, input)
	return err
}

// ListFiles lists files in S3 with a given prefix
func (c *S3Client) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	// prefix can be empty - it would list all objects in the bucket

	// Create list input
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.config.Bucket),
		Prefix: aws.String(prefix),
	}

	// List with context timeout
	ctx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()

	// Get the list of objects
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(c.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}

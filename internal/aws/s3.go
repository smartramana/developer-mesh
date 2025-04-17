package aws

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Config holds configuration for S3
type S3Config struct {
	AuthConfig        AuthConfig   `mapstructure:"auth"`
	Bucket            string       `mapstructure:"bucket"`
	Region            string       `mapstructure:"region"`
	Endpoint          string       `mapstructure:"endpoint"`
	ForcePathStyle    bool         `mapstructure:"force_path_style"`
	UseIAMAuth        bool         `mapstructure:"use_iam_auth"`
	UploadPartSize    int64        `mapstructure:"upload_part_size"`
	DownloadPartSize  int64        `mapstructure:"download_part_size"`
	Concurrency       int          `mapstructure:"concurrency"`
	RequestTimeout    time.Duration `mapstructure:"request_timeout"`
	ServerSideEncryption string    `mapstructure:"server_side_encryption"`
	EncryptionKeyID   string       `mapstructure:"encryption_key_id"`
}

// S3Client is a client for AWS S3
type S3Client struct {
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	config     S3Config
}

// GetBucketName returns the bucket name from the configuration
func (c *S3Client) GetBucketName() string {
	return c.config.Bucket
}

// NewS3Client creates a new S3 client with IRSA support
func NewS3Client(ctx context.Context, cfg S3Config) (*S3Client, error) {
	// Get AWS configuration with IRSA support when IAM auth is enabled
	var awsCfg aws.Config
	var err error
	
	// Always try to use IAM authentication by default, unless explicitly disabled
	// This ensures we follow the principle of least privilege
	useIAM := cfg.UseIAMAuth
	
	if useIAM {
		awsCfg, err = GetAWSConfig(ctx, cfg.AuthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config with IAM auth: %w", err)
		}
		
		if IsIRSAEnabled() {
			log.Println("Using IRSA authentication for S3 with role:", os.Getenv("AWS_ROLE_ARN"))
		} else {
			log.Println("Using standard IAM authentication for S3 (IRSA not detected)")
		}
	} else {
		// Only fall back to basic AWS config when IAM auth is explicitly disabled
		awsCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
		if err != nil {
			return nil, fmt.Errorf("failed to load basic AWS config: %w", err)
		}
		log.Println("Using basic AWS authentication for S3 (IAM auth explicitly disabled)")
	}

	// Create S3 client with options
	s3Options := []func(*s3.Options){}
	
	// Force path style if required (for LocalStack or other S3-compatible services)
	if cfg.ForcePathStyle {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, s3Options...)

	// Create uploader and downloader with optimized settings
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		if cfg.UploadPartSize > 0 {
			u.PartSize = cfg.UploadPartSize
		}
		if cfg.Concurrency > 0 {
			u.Concurrency = cfg.Concurrency
		}
	})

	downloader := manager.NewDownloader(client, func(d *manager.Downloader) {
		if cfg.DownloadPartSize > 0 {
			d.PartSize = cfg.DownloadPartSize
		}
		if cfg.Concurrency > 0 {
			d.Concurrency = cfg.Concurrency
		}
	})

	return &S3Client{
		client:     client,
		uploader:   uploader,
		downloader: downloader,
		config:     cfg,
	}, nil
}

// UploadFile uploads a file to S3 with optional encryption
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

	// Add server-side encryption if configured
	if c.config.ServerSideEncryption != "" {
		input.ServerSideEncryption = types.ServerSideEncryption(c.config.ServerSideEncryption)
		
		// Add KMS key ID if using SSE-KMS and a key ID is provided
		if c.config.ServerSideEncryption == string(types.ServerSideEncryptionAwsKms) && 
		   c.config.EncryptionKeyID != "" {
			input.SSEKMSKeyId = aws.String(c.config.EncryptionKeyID)
		}
	}

	// Upload with context timeout
	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

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
	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

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
	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

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
	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

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

// GetObjectAttributes gets attributes of an S3 object
func (c *S3Client) GetObjectAttributes(ctx context.Context, key string) (*s3.HeadObjectOutput, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	// Create head input
	input := &s3.HeadObjectInput{
		Bucket: aws.String(c.config.Bucket),
		Key:    aws.String(key),
	}

	// Head with context timeout
	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	return c.client.HeadObject(ctx, input)
}

// CheckBucketExists checks if the configured S3 bucket exists
func (c *S3Client) CheckBucketExists(ctx context.Context) (bool, error) {
	// Create head bucket input
	input := &s3.HeadBucketInput{
		Bucket: aws.String(c.config.Bucket),
	}

	// Head with context timeout
	if c.config.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}

	_, err := c.client.HeadBucket(ctx, input)
	if err != nil {
		// TODO: Check if this is a "bucket not found" error specifically
		return false, nil
	}

	return true, nil
}

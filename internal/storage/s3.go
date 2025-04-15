package aws

import (
	"bytes"
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

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
}

// NewS3Client creates a new S3 client
func NewS3Client(ctx context.Context, cfg S3Config) (*S3Client, error) {
	// Create config options
	var options []func(*config.LoadOptions) error
	options = append(options, config.WithRegion(cfg.Region))
	
	// Add custom endpoint if specified (for LocalStack or other S3 compatible services)
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
	
	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
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
		client:     client,
		uploader:   uploader,
		downloader: downloader,
		config:     cfg,
	}, nil
}

// UploadFile uploads a file to S3
func (c *S3Client) UploadFile(ctx context.Context, key string, data []byte, contentType string) error {
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
			keys = append(keys, *obj.Key)
		}
	}

	return keys, nil
}

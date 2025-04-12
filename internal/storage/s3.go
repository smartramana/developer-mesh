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

// S3Config holds configuration for the S3 client
type S3Config struct {
	Region           string        `mapstructure:"region"`
	Bucket           string        `mapstructure:"bucket"`
	UploadPartSize   int64         `mapstructure:"upload_part_size"`
	DownloadPartSize int64         `mapstructure:"download_part_size"`
	Concurrency      int           `mapstructure:"concurrency"`
	RequestTimeout   time.Duration `mapstructure:"request_timeout"`
}

// NewS3Client creates a new S3 client
func NewS3Client(ctx context.Context, cfg S3Config) (*S3Client, error) {
	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, err
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg)

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

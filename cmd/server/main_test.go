package main

import (
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/internal/api"
	"github.com/S-Corkum/mcp-server/internal/aws"
	"github.com/S-Corkum/mcp-server/internal/config"
	"github.com/S-Corkum/mcp-server/internal/database"
	"github.com/stretchr/testify/assert"
)

// TestValidateConfiguration tests the validateConfiguration function
func TestValidateConfiguration(t *testing.T) {
	// Create a test configuration
	useAWS := false
	useIAM := false
	
	cfg := &config.Config{
		Database: database.Config{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Username: "testuser",
			Password: "testpass",
			Database: "testdb",
			UseAWS:   &useAWS,
			UseIAM:   &useIAM,
		},
		API: api.Config{
			ListenAddress: ":8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
			Auth: api.AuthConfig{
				JWTSecret: "test-secret",
				APIKeys: map[string]string{
					"admin": "test-admin-key",
				},
			},
			RateLimit: api.RateLimitConfig{
				Enabled:     true,
				Limit:       100,
				Period:      time.Minute,
				BurstFactor: 3,
			},
			Webhooks: api.WebhookConfig{
				GitHub: api.WebhookEndpointConfig{
					Enabled: true,
					Path:    "/github",
					Secret:  "github-secret",
				},
			},
		},
		AWS: config.AWSConfig{
			RDS: aws.RDSConfig{
				UseIAMAuth: false,
			},
		},
	}

	// Test the validation function
	err := validateConfiguration(cfg)
	assert.NoError(t, err, "Validation should pass with valid configuration")

	// Test with missing database configuration
	invalidCfg := &config.Config{
		API: cfg.API,
		Database: database.Config{
			Driver: "postgres",
			// Missing host, port, and database
			UseAWS: &useAWS,
			UseIAM: &useIAM,
		},
		AWS: config.AWSConfig{
			RDS: aws.RDSConfig{
				UseIAMAuth: false,
			},
		},
	}
	err = validateConfiguration(invalidCfg)
	assert.Error(t, err, "Validation should fail with invalid database configuration")

	// Test with missing API timeouts
	invalidCfg = &config.Config{
		Database: cfg.Database,
		API: api.Config{
			// Missing timeouts
			Auth: cfg.API.Auth,
			Webhooks: cfg.API.Webhooks,
		},
		AWS: config.AWSConfig{
			RDS: aws.RDSConfig{
				UseIAMAuth: false,
			},
		},
	}
	err = validateConfiguration(invalidCfg)
	assert.Error(t, err, "Validation should fail with missing API timeouts")

	// Test with AWS RDS IAM auth enabled
	useIAMAuth := true
	awsAuthCfg := &config.Config{
		Database: cfg.Database,
		API: cfg.API,
		AWS: config.AWSConfig{
			RDS: aws.RDSConfig{
				UseIAMAuth: useIAMAuth,
				Host:       "test-rds-host.amazonaws.com",
			},
		},
	}
	err = validateConfiguration(awsAuthCfg)
	assert.NoError(t, err, "Validation should pass with AWS RDS IAM auth enabled")

	// Test with enabled webhook without a secret (should pass but log a warning)
	webhookNoSecretCfg := &config.Config{
		Database: cfg.Database,
		API: api.Config{
			ListenAddress: ":8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
			Auth: cfg.API.Auth,
			Webhooks: api.WebhookConfig{
				GitHub: api.WebhookEndpointConfig{
					Enabled: true,
					Path:    "/github",
					Secret:  "", // Empty secret
				},
			},
		},
		AWS: cfg.AWS,
	}
	err = validateConfiguration(webhookNoSecretCfg)
	assert.NoError(t, err, "Validation should pass with webhook enabled without a secret")
}

// TestInitSecureRandom tests the secure random initialization function
func TestInitSecureRandom(t *testing.T) {
	// Test secure random initialization
	initSecureRandom()
	// No assertion needed, just make sure it doesn't panic
}

// TestBuildS3ClientConfig tests building S3 client configuration
func TestBuildS3ClientConfig(t *testing.T) {
	// Create test configuration for S3
	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type: "s3",
			S3: aws.S3Config{
				Region:         "us-west-2",
				Bucket:         "test-bucket",
				Endpoint:       "http://localhost:4566",
				ForcePathStyle: true,
				UploadPartSize: 5242880,
				DownloadPartSize: 5242880,
				Concurrency:    5,
				RequestTimeout: 30 * time.Second,
			},
			ContextStorage: config.ContextStorage{
				Provider:    "s3",
				S3PathPrefix: "contexts",
			},
		},
		AWS: config.AWSConfig{
			S3: aws.S3Config{
				AuthConfig: aws.AuthConfig{
					Region:   "us-west-2",
					Endpoint: "http://localhost:4566",
				},
				Bucket:     "test-bucket",
				UseIAMAuth: true,
			},
		},
	}

	// Test building S3 client configuration
	s3Config := buildS3ClientConfig(cfg)
	
	// Assert that the configuration was built correctly
	assert.Equal(t, "us-west-2", s3Config.Region)
	assert.Equal(t, "test-bucket", s3Config.Bucket)
	assert.Equal(t, "http://localhost:4566", s3Config.Endpoint)
	assert.True(t, s3Config.ForcePathStyle)
	assert.Equal(t, int64(5242880), s3Config.UploadPartSize)
	assert.Equal(t, int64(5242880), s3Config.DownloadPartSize)
	assert.Equal(t, 5, s3Config.Concurrency)
	assert.Equal(t, 30*time.Second, s3Config.RequestTimeout)
	assert.True(t, s3Config.AWSConfig.UseIAMAuth)
	assert.Equal(t, "us-west-2", s3Config.AWSConfig.Region)
}

// Test with additional AWS configurations
func TestBuildS3ClientConfigWithDifferentAWSSettings(t *testing.T) {
	// Create test configuration with different AWS settings
	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type: "s3",
			S3: aws.S3Config{
				Region:         "us-east-1",
				Bucket:         "prod-bucket",
				Endpoint:       "",
				ForcePathStyle: false,
				UploadPartSize: 10485760,
				DownloadPartSize: 10485760,
				Concurrency:    10,
				RequestTimeout: 60 * time.Second,
			},
			ContextStorage: config.ContextStorage{
				Provider:    "s3",
				S3PathPrefix: "prod-contexts",
			},
		},
		AWS: config.AWSConfig{
			S3: aws.S3Config{
				AuthConfig: aws.AuthConfig{
					Region:   "us-east-1",
					Endpoint: "",
					AssumeRole: "arn:aws:iam::123456789012:role/s3-access-role",
				},
				Bucket:     "prod-bucket",
				UseIAMAuth: true,
			},
		},
	}

	// Test building S3 client configuration
	s3Config := buildS3ClientConfig(cfg)
	
	// Assert that the configuration was built correctly
	assert.Equal(t, "us-east-1", s3Config.Region)
	assert.Equal(t, "prod-bucket", s3Config.Bucket)
	assert.Equal(t, "", s3Config.Endpoint)
	assert.False(t, s3Config.ForcePathStyle)
	assert.Equal(t, int64(10485760), s3Config.UploadPartSize)
	assert.Equal(t, int64(10485760), s3Config.DownloadPartSize)
	assert.Equal(t, 10, s3Config.Concurrency)
	assert.Equal(t, 60*time.Second, s3Config.RequestTimeout)
	assert.True(t, s3Config.AWSConfig.UseIAMAuth)
	assert.Equal(t, "us-east-1", s3Config.AWSConfig.Region)
	assert.Equal(t, "arn:aws:iam::123456789012:role/s3-access-role", s3Config.AWSConfig.AssumeRole)
}

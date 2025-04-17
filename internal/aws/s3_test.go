package aws

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewS3Client(t *testing.T) {
	// Save original environment variables
	originalWebIdentityTokenFile, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
	originalRoleArn, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")
	
	// Clean up environment variables when the test completes
	defer func() {
		os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
		os.Unsetenv("AWS_ROLE_ARN")
		
		// Restore original environment variables if they existed
		if hasWebIdentityTokenFile {
			os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", originalWebIdentityTokenFile)
		}
		if hasRoleArn {
			os.Setenv("AWS_ROLE_ARN", originalRoleArn)
		}
	}()
	
	// Test with standard config (no IRSA)
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Unsetenv("AWS_ROLE_ARN")
	
	cfg := S3Config{
		AuthConfig: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		Bucket:          "test-bucket",
		ForcePathStyle:  true,
		RequestTimeout:  5 * time.Second,
		UseIAMAuth:      false,
		UploadPartSize:  5 * 1024 * 1024, // 5MB
		DownloadPartSize: 5 * 1024 * 1024, // 5MB
		Concurrency:     5,
	}
	
	// Test creation with standard config
	client, err := NewS3Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3Client returned error: %v", err)
	}
	
	if client.GetBucketName() != "test-bucket" {
		t.Errorf("Expected bucket name to be test-bucket, got %s", client.GetBucketName())
	}
	
	// Test with IRSA enabled
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token")
	os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
	
	cfg.UseIAMAuth = true
	
	client, err = NewS3Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3Client with IRSA returned error: %v", err)
	}
	
	if client.GetBucketName() != "test-bucket" {
		t.Errorf("Expected bucket name to be test-bucket, got %s", client.GetBucketName())
	}
}

func TestS3ClientWithIRSA(t *testing.T) {
	// Save original environment variables
	originalWebIdentityTokenFile, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
	originalRoleArn, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")
	
	// Clean up environment variables when the test completes
	defer func() {
		os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
		os.Unsetenv("AWS_ROLE_ARN")
		
		// Restore original environment variables if they existed
		if hasWebIdentityTokenFile {
			os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", originalWebIdentityTokenFile)
		}
		if hasRoleArn {
			os.Setenv("AWS_ROLE_ARN", originalRoleArn)
		}
	}()
	
	// Enable IRSA
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token")
	os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
	
	// Create config with IAM auth enabled
	cfg := S3Config{
		AuthConfig: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		Bucket:          "test-bucket",
		ForcePathStyle:  true,
		RequestTimeout:  5 * time.Second,
		UseIAMAuth:      true,
		UploadPartSize:  5 * 1024 * 1024, // 5MB
		DownloadPartSize: 5 * 1024 * 1024, // 5MB
		Concurrency:     5,
	}
	
	// Verify that IRSA is enabled
	if !IsIRSAEnabled() {
		t.Fatal("IRSA should be enabled")
	}
	
	// Create client - this should use IRSA
	client, err := NewS3Client(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3Client with IRSA returned error: %v", err)
	}
	
	// Verify the client was created with the correct bucket
	if client.GetBucketName() != "test-bucket" {
		t.Errorf("Expected bucket name to be test-bucket, got %s", client.GetBucketName())
	}
}

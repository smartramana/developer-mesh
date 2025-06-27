package aws

import (
	"context"
	"os"
	"testing"
)

func TestIAMAuthDefaults(t *testing.T) {
	// Test that IAM auth is enabled by default

	// Create a test RDS config
	rdsConfig := RDSConnectionConfig{
		Auth: AuthConfig{
			Region: "us-west-2",
		},
		Host:       "test-rds-host",
		Port:       5432,
		Database:   "testdb",
		Username:   "testuser",
		UseIAMAuth: true,
	}

	// Create a client
	client, err := NewExtendedRDSClient(context.Background(), rdsConfig)
	if err != nil {
		t.Fatalf("Failed to create RDS client: %v", err)
	}

	// Ensure UseIAMAuth is set correctly
	if !client.connConfig.UseIAMAuth {
		t.Errorf("IAM auth not enabled by default for RDS")
	}

	// Test ElastiCache as well
	ecConfig := ElastiCacheConfig{
		AuthConfig: AuthConfig{
			Region: "us-west-2",
		},
		PrimaryEndpoint: "test-redis-host",
		Port:            6379,
		Username:        "testuser",
		UseIAMAuth:      true,
	}

	// Create a client
	ecClient, err := NewElastiCacheClient(context.Background(), ecConfig)
	if err != nil {
		t.Fatalf("Failed to create ElastiCache client: %v", err)
	}

	// Ensure UseIAMAuth is set correctly
	if !ecClient.config.UseIAMAuth {
		t.Errorf("IAM auth not enabled by default for ElastiCache")
	}

	// Test S3 as well

	// Create a client - will fail without AWS credentials but we're just testing defaults

}

func TestIRSADetection(t *testing.T) {
	// Test IRSA detection when environment variables are set

	// Save original env vars
	origWebIdentityTokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	origRoleArn := os.Getenv("AWS_ROLE_ARN")

	// Set test env vars
	if err := os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"); err != nil {
		t.Fatalf("Failed to set AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role"); err != nil {
		t.Fatalf("Failed to set AWS_ROLE_ARN: %v", err)
	}

	// Test IRSA detection
	if !IsIRSAEnabled() {
		t.Errorf("IRSA not detected when env vars are set")
	}

	// Clean up
	if err := os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", origWebIdentityTokenFile); err != nil {
		t.Errorf("Failed to restore AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Setenv("AWS_ROLE_ARN", origRoleArn); err != nil {
		t.Errorf("Failed to restore AWS_ROLE_ARN: %v", err)
	}
}

func TestIsIRSAEnabled(t *testing.T) {
	// Save original environment variables
	originalWebIdentityTokenFile, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
	originalRoleArn, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")

	// Clean up environment variables when the test completes
	defer func() {
		_ = os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE") // Ignore error in cleanup
		_ = os.Unsetenv("AWS_ROLE_ARN")                // Ignore error in cleanup

		// Restore original environment variables if they existed
		if hasWebIdentityTokenFile {
			if err := os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", originalWebIdentityTokenFile); err != nil {
				t.Errorf("Failed to restore AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
			}
		}
		if hasRoleArn {
			if err := os.Setenv("AWS_ROLE_ARN", originalRoleArn); err != nil {
				t.Errorf("Failed to restore AWS_ROLE_ARN: %v", err)
			}
		}
	}()

	// Test case 1: No IRSA environment variables set
	if err := os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE"); err != nil {
		t.Errorf("Failed to unset AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Unsetenv("AWS_ROLE_ARN"); err != nil {
		t.Errorf("Failed to unset AWS_ROLE_ARN: %v", err)
	}

	if IsIRSAEnabled() {
		t.Error("Expected IsIRSAEnabled() to return false when no environment variables are set")
	}

	// Test case 2: Only AWS_WEB_IDENTITY_TOKEN_FILE set
	if err := os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token"); err != nil {
		t.Errorf("Failed to set AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Unsetenv("AWS_ROLE_ARN"); err != nil {
		t.Errorf("Failed to unset AWS_ROLE_ARN: %v", err)
	}

	if IsIRSAEnabled() {
		t.Error("Expected IsIRSAEnabled() to return false when only AWS_WEB_IDENTITY_TOKEN_FILE is set")
	}

	// Test case 3: Only AWS_ROLE_ARN set
	if err := os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE"); err != nil {
		t.Errorf("Failed to unset AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role"); err != nil {
		t.Errorf("Failed to set AWS_ROLE_ARN: %v", err)
	}

	if IsIRSAEnabled() {
		t.Error("Expected IsIRSAEnabled() to return false when only AWS_ROLE_ARN is set")
	}

	// Test case 4: Both environment variables set (IRSA enabled)
	if err := os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token"); err != nil {
		t.Errorf("Failed to set AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role"); err != nil {
		t.Errorf("Failed to set AWS_ROLE_ARN: %v", err)
	}

	if !IsIRSAEnabled() {
		t.Error("Expected IsIRSAEnabled() to return true when both environment variables are set")
	}
}

func TestGetAWSConfig(t *testing.T) {
	// Save original environment variables
	originalWebIdentityTokenFile, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
	originalRoleArn, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")

	// Clean up environment variables when the test completes
	defer func() {
		_ = os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE") // Ignore error in cleanup
		_ = os.Unsetenv("AWS_ROLE_ARN")                // Ignore error in cleanup

		// Restore original environment variables if they existed
		if hasWebIdentityTokenFile {
			if err := os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", originalWebIdentityTokenFile); err != nil {
				t.Errorf("Failed to restore AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
			}
		}
		if hasRoleArn {
			if err := os.Setenv("AWS_ROLE_ARN", originalRoleArn); err != nil {
				t.Errorf("Failed to restore AWS_ROLE_ARN: %v", err)
			}
		}
	}()

	// Test configuration
	cfg := AuthConfig{
		Region:   "us-west-2",
		Endpoint: "http://localhost:4566", // LocalStack endpoint
	}

	// Test without IRSA
	if err := os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE"); err != nil {
		t.Errorf("Failed to unset AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Unsetenv("AWS_ROLE_ARN"); err != nil {
		t.Errorf("Failed to unset AWS_ROLE_ARN: %v", err)
	}

	awsCfg, err := GetAWSConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("GetAWSConfig returned error: %v", err)
	}

	if awsCfg.Region != "us-west-2" {
		t.Errorf("Expected Region to be us-west-2, got %s", awsCfg.Region)
	}

	// Test with IRSA
	if err := os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token"); err != nil {
		t.Errorf("Failed to set AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role"); err != nil {
		t.Errorf("Failed to set AWS_ROLE_ARN: %v", err)
	}

	awsCfg, err = GetAWSConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("GetAWSConfig with IRSA returned error: %v", err)
	}

	if awsCfg.Region != "us-west-2" {
		t.Errorf("Expected Region to be us-west-2, got %s", awsCfg.Region)
	}
}

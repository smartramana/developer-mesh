package aws

import (
	"context"
	"os"
	"testing"
)

func TestIsIRSAEnabled(t *testing.T) {
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
	
	// Test case 1: No IRSA environment variables set
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Unsetenv("AWS_ROLE_ARN")
	
	if IsIRSAEnabled() {
		t.Error("Expected IsIRSAEnabled() to return false when no environment variables are set")
	}
	
	// Test case 2: Only AWS_WEB_IDENTITY_TOKEN_FILE set
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token")
	os.Unsetenv("AWS_ROLE_ARN")
	
	if IsIRSAEnabled() {
		t.Error("Expected IsIRSAEnabled() to return false when only AWS_WEB_IDENTITY_TOKEN_FILE is set")
	}
	
	// Test case 3: Only AWS_ROLE_ARN set
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
	
	if IsIRSAEnabled() {
		t.Error("Expected IsIRSAEnabled() to return false when only AWS_ROLE_ARN is set")
	}
	
	// Test case 4: Both environment variables set (IRSA enabled)
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token")
	os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
	
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
	
	// Test configuration
	cfg := AuthConfig{
		Region:   "us-west-2",
		Endpoint: "http://localhost:4566", // LocalStack endpoint
	}
	
	// Test without IRSA
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Unsetenv("AWS_ROLE_ARN")
	
	awsCfg, err := GetAWSConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("GetAWSConfig returned error: %v", err)
	}
	
	if awsCfg.Region != "us-west-2" {
		t.Errorf("Expected Region to be us-west-2, got %s", awsCfg.Region)
	}
	
	// Test with IRSA
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token")
	os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
	
	awsCfg, err = GetAWSConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("GetAWSConfig with IRSA returned error: %v", err)
	}
	
	if awsCfg.Region != "us-west-2" {
		t.Errorf("Expected Region to be us-west-2, got %s", awsCfg.Region)
	}
}

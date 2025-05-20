package aws

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewRDSClient(t *testing.T) {
	cfg := RDSConfig{
		AuthConfig: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		Host:              "localhost",
		Port:              5432,
		Database:          "testdb",
		Username:          "testuser",
		Password:          "testpass",
		UseIAMAuth:        false,
		TokenExpiration:   15, // 15 minutes
		MaxOpenConns:      10,
		MaxIdleConns:      5,
		ConnMaxLifetime:   5 * time.Minute,
		EnablePooling:     true,
		MinPoolSize:       1,
		MaxPoolSize:       5,
		ConnectionTimeout: 5,
	}
	
	client, err := NewRDSClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewRDSClient returned error: %v", err)
	}
	
	if client == nil {
		t.Fatal("Expected non-nil RDSClient")
	}
}

func TestBuildPostgresConnectionString(t *testing.T) {
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
	
	// Test standard connection string (no IRSA)
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Unsetenv("AWS_ROLE_ARN")
	
	cfg := RDSConfig{
		AuthConfig: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		Host:              "localhost",
		Port:              5432,
		Database:          "testdb",
		Username:          "testuser",
		Password:          "testpass",
		UseIAMAuth:        false,
		TokenExpiration:   15, // 15 minutes
		MaxOpenConns:      10,
		MaxIdleConns:      5,
		ConnMaxLifetime:   5 * time.Minute,
		EnablePooling:     true,
		MinPoolSize:       1,
		MaxPoolSize:       5,
		ConnectionTimeout: 5,
	}
	
	client, err := NewRDSClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewRDSClient returned error: %v", err)
	}
	
	dsn, err := client.BuildPostgresConnectionString(context.Background())
	if err != nil {
		t.Fatalf("BuildPostgresConnectionString returned error: %v", err)
	}
	
	// Verify the connection string
	expectedParts := []string{
		"host=localhost",
		"port=5432",
		"dbname=testdb",
		"user=testuser",
		"password=testpass",
		"sslmode=require",
		"pool=true",
		"min_pool_size=1",
		"max_pool_size=5",
	}
	
	for _, part := range expectedParts {
		if !strings.Contains(dsn, part) {
			t.Errorf("Expected connection string to contain %s, got %s", part, dsn)
		}
	}
	
	// Test with IRSA enabled
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token")
	os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
	
	cfg.UseIAMAuth = true
	
	client, err = NewRDSClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewRDSClient with IRSA returned error: %v", err)
	}
	
	dsn, err = client.BuildPostgresConnectionString(context.Background())
	if err != nil {
		t.Fatalf("BuildPostgresConnectionString with IRSA returned error: %v", err)
	}
	
	// Verify the connection string has the token instead of the password
	iamParts := []string{
		"host=localhost",
		"port=5432",
		"dbname=testdb",
		"user=testuser",
		"password=mock-auth-token", // From the mock implementation in rds.go
		"sslmode=require",
	}
	
	for _, part := range iamParts {
		if !strings.Contains(dsn, part) {
			t.Errorf("Expected IAM connection string to contain %s, got %s", part, dsn)
		}
	}
}

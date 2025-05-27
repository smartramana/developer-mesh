package aws

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewExtendedRDSClient(t *testing.T) {
	cfg := RDSConnectionConfig{
		Auth: AuthConfig{
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

	client, err := NewExtendedRDSClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewExtendedRDSClient returned error: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil ExtendedRDSClient")
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

	cfg := RDSConnectionConfig{
		Auth: AuthConfig{
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

	client, err := NewExtendedRDSClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewExtendedRDSClient returned error: %v", err)
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

	cfg2 := RDSConnectionConfig{
		Auth: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		Host:              "localhost",
		Port:              5432,
		Database:          "testdb",
		Username:          "testuser",
		Password:          "", // No password for IAM auth
		UseIAMAuth:        true,
		TokenExpiration:   15, // 15 minutes
		MaxOpenConns:      10,
		MaxIdleConns:      5,
		ConnMaxLifetime:   5 * time.Minute,
		EnablePooling:     true,
		MinPoolSize:       1,
		MaxPoolSize:       5,
		ConnectionTimeout: 5,
	}

	client2, err := NewExtendedRDSClient(context.Background(), cfg2)
	if err != nil {
		t.Fatalf("NewExtendedRDSClient with IAM returned error: %v", err)
	}

	// This should use IAM auth token
	dsnIAM, err := client2.BuildPostgresConnectionString(context.Background())
	if err != nil {
		t.Fatalf("BuildPostgresConnectionString with IAM returned error: %v", err)
	}

	// Should contain basic connection params but not a regular password
	if !strings.Contains(dsnIAM, "host=localhost") {
		t.Errorf("Expected connection string to contain host=localhost")
	}

	// For localhost, it should use mock token
	if !strings.Contains(dsnIAM, "password=mock-auth-token-local") {
		t.Errorf("Expected connection string to contain mock token for localhost")
	}
}

func TestGetAuthToken(t *testing.T) {
	cfg := RDSConnectionConfig{
		Auth: AuthConfig{
			Region: "us-west-2",
		},
		Host:       "prod-database.cluster-abc123.us-west-2.rds.amazonaws.com",
		Port:       5432,
		Database:   "mydb",
		Username:   "myuser",
		UseIAMAuth: true,
	}

	client, err := NewExtendedRDSClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewExtendedRDSClient returned error: %v", err)
	}

	// For non-localhost, this would normally use real IAM auth
	// but since we don't have AWS credentials, it returns a mock token
	token, err := client.GetAuthToken(context.Background())
	if err != nil {
		t.Fatalf("GetAuthToken returned error: %v", err)
	}

	// Should return a mock token for now
	if token != "iam-auth-token" {
		t.Errorf("Expected mock IAM token, got %s", token)
	}

	// Test with localhost (should return local mock token)
	cfgLocal := RDSConnectionConfig{
		Auth: AuthConfig{
			Region: "us-west-2",
		},
		Host:       "localhost",
		Port:       5432,
		Database:   "testdb",
		Username:   "testuser",
		UseIAMAuth: true,
	}

	clientLocal, err := NewExtendedRDSClient(context.Background(), cfgLocal)
	if err != nil {
		t.Fatalf("NewExtendedRDSClient returned error: %v", err)
	}

	tokenLocal, err := clientLocal.GetAuthToken(context.Background())
	if err != nil {
		t.Fatalf("GetAuthToken for localhost returned error: %v", err)
	}

	if tokenLocal != "mock-auth-token-local" {
		t.Errorf("Expected local mock token, got %s", tokenLocal)
	}

	// Test with IAM auth disabled
	cfgNoIAM := RDSConnectionConfig{
		Auth: AuthConfig{
			Region: "us-west-2",
		},
		Host:       "localhost",
		Port:       5432,
		Database:   "testdb",
		Username:   "testuser",
		Password:   "mypassword",
		UseIAMAuth: false,
	}

	clientNoIAM, err := NewExtendedRDSClient(context.Background(), cfgNoIAM)
	if err != nil {
		t.Fatalf("NewExtendedRDSClient returned error: %v", err)
	}

	tokenNoIAM, err := clientNoIAM.GetAuthToken(context.Background())
	if err != nil {
		t.Fatalf("GetAuthToken without IAM returned error: %v", err)
	}

	// Should return the password when IAM is disabled
	if tokenNoIAM != "mypassword" {
		t.Errorf("Expected password when IAM disabled, got %s", tokenNoIAM)
	}
}
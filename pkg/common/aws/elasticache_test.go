package aws

import (
	"context"
	"os"
	"testing"
	"time"

	securitytls "github.com/S-Corkum/devops-mcp/pkg/security/tls"
)

func TestNewElastiCacheClient(t *testing.T) {
	cfg := ElastiCacheConfig{
		AuthConfig: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		PrimaryEndpoint:    "localhost",
		Port:               6379,
		Username:           "default",
		Password:           "password",
		UseIAMAuth:         false,
		ClusterMode:        false,
		ClusterDiscovery:   false,
		TLS:                nil, // TLS disabled for this test
		MaxRetries:         3,
		MinIdleConnections: 5,
		PoolSize:           10,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		PoolTimeout:        4,
		TokenExpiration:    15, // 15 minutes
	}

	client, err := NewElastiCacheClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewElastiCacheClient returned error: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil ElastiCacheClient")
	}
}

func TestBuildRedisOptions(t *testing.T) {
	// Save original environment variables
	originalWebIdentityTokenFile, hasWebIdentityTokenFile := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
	originalRoleArn, hasRoleArn := os.LookupEnv("AWS_ROLE_ARN")

	// Clean up environment variables when the test completes
	defer func() {
		if err := os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE"); err != nil {
			t.Errorf("Failed to unset AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
		}
		if err := os.Unsetenv("AWS_ROLE_ARN"); err != nil {
			t.Errorf("Failed to unset AWS_ROLE_ARN: %v", err)
		}

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

	// Test standard options (no IRSA)
	if err := os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE"); err != nil {
		t.Errorf("Failed to unset AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Unsetenv("AWS_ROLE_ARN"); err != nil {
		t.Errorf("Failed to unset AWS_ROLE_ARN: %v", err)
	}

	cfg := ElastiCacheConfig{
		AuthConfig: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		PrimaryEndpoint:    "localhost",
		Port:               6379,
		Username:           "default",
		Password:           "password",
		UseIAMAuth:         false,
		ClusterMode:        false,
		ClusterDiscovery:   false,
		TLS:                nil, // TLS disabled for this test
		MaxRetries:         3,
		MinIdleConnections: 5,
		PoolSize:           10,
		DialTimeout:        5,
		ReadTimeout:        3,
		WriteTimeout:       3,
		PoolTimeout:        4,
		TokenExpiration:    15, // 15 minutes
	}

	client, err := NewElastiCacheClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewElastiCacheClient returned error: %v", err)
	}

	options, err := client.BuildRedisOptions(context.Background())
	if err != nil {
		t.Fatalf("BuildRedisOptions returned error: %v", err)
	}

	// Verify the options
	addr, ok := options["addr"].(string)
	if !ok || addr != "localhost:6379" {
		t.Errorf("Expected addr to be localhost:6379, got %v", options["addr"])
	}

	clusterMode, ok := options["clusterMode"].(bool)
	if !ok || clusterMode != false {
		t.Errorf("Expected clusterMode to be false, got %v", options["clusterMode"])
	}

	username, ok := options["username"].(string)
	if !ok || username != "default" {
		t.Errorf("Expected username to be default, got %v", options["username"])
	}

	password, ok := options["password"].(string)
	if !ok || password != "password" {
		t.Errorf("Expected password to be password, got %v", options["password"])
	}

	// Test with IRSA enabled
	if err := os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token"); err != nil {
		t.Errorf("Failed to set AWS_WEB_IDENTITY_TOKEN_FILE: %v", err)
	}
	if err := os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role"); err != nil {
		t.Errorf("Failed to set AWS_ROLE_ARN: %v", err)
	}

	cfg.UseIAMAuth = true

	client, err = NewElastiCacheClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewElastiCacheClient with IRSA returned error: %v", err)
	}

	options, err = client.BuildRedisOptions(context.Background())
	if err != nil {
		t.Fatalf("BuildRedisOptions with IRSA returned error: %v", err)
	}

	// Verify the options have the token instead of the password
	username, ok = options["username"].(string)
	if !ok || username != "default" {
		t.Errorf("Expected username to be default, got %v", options["username"])
	}

	password, ok = options["password"].(string)
	if !ok || (password != "mock-auth-token" && password != "mock-auth-token-local") {
		// Accept either token value since our implementation now differentiates between local and non-local endpoints
		t.Errorf("Expected password to be an auth token, got %v", options["password"])
	}
}

func TestElastiCacheClusterMode(t *testing.T) {
	// Test with cluster mode enabled
	cfg := ElastiCacheConfig{
		AuthConfig: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		PrimaryEndpoint:  "primary.cache.amazonaws.com",
		ReaderEndpoint:   "reader.cache.amazonaws.com",
		Port:             6379,
		Username:         "default",
		Password:         "password",
		UseIAMAuth:       false,
		ClusterMode:      true,
		ClusterDiscovery: false,
		TLS: &securitytls.Config{
			Enabled:            true,
			InsecureSkipVerify: false,
			MinVersion:         "1.3",
		},
		MaxRetries:         3,
		MinIdleConnections: 5,
		PoolSize:           10,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		PoolTimeout:        4,
		TokenExpiration:    15, // 15 minutes
	}

	client, err := NewElastiCacheClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewElastiCacheClient returned error: %v", err)
	}

	options, err := client.BuildRedisOptions(context.Background())
	if err != nil {
		t.Fatalf("BuildRedisOptions returned error: %v", err)
	}

	// Verify cluster mode settings
	clusterMode, ok := options["clusterMode"].(bool)
	if !ok || clusterMode != true {
		t.Errorf("Expected clusterMode to be true, got %v", options["clusterMode"])
	}

	addrs, ok := options["addrs"].([]string)
	if !ok {
		t.Fatalf("Expected addrs to be a string slice, got %T", options["addrs"])
	}

	expectedAddrs := []string{
		"primary.cache.amazonaws.com:6379",
		"reader.cache.amazonaws.com:6379",
	}

	if len(addrs) != len(expectedAddrs) {
		t.Fatalf("Expected %d addresses, got %d", len(expectedAddrs), len(addrs))
	}

	for i, addr := range addrs {
		if addr != expectedAddrs[i] {
			t.Errorf("Expected address at index %d to be %s, got %s", i, expectedAddrs[i], addr)
		}
	}

	// Verify TLS settings
	_, ok = options["tls"]
	if !ok {
		t.Error("Expected TLS config to be present")
	}
}

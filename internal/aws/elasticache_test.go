package aws

import (
	"context"
	"os"
	"testing"
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
		UseTLS:             false,
		InsecureSkipVerify: true,
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
	
	// Test standard options (no IRSA)
	os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	os.Unsetenv("AWS_ROLE_ARN")
	
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
		UseTLS:             false,
		InsecureSkipVerify: true,
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
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/token")
	os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
	
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
	if !ok || password != "mock-auth-token" { // From the mock implementation in elasticache.go
		t.Errorf("Expected password to be mock-auth-token, got %v", options["password"])
	}
}

func TestElastiCacheClusterMode(t *testing.T) {
	// Test with cluster mode enabled
	cfg := ElastiCacheConfig{
		AuthConfig: AuthConfig{
			Region:   "us-west-2",
			Endpoint: "http://localhost:4566", // LocalStack endpoint
		},
		PrimaryEndpoint:    "primary.cache.amazonaws.com",
		ReaderEndpoint:     "reader.cache.amazonaws.com",
		Port:               6379,
		Username:           "default",
		Password:           "password",
		UseIAMAuth:         false,
		ClusterMode:        true,
		ClusterDiscovery:   false,
		UseTLS:             true,
		InsecureSkipVerify: false,
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

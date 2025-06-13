package shared

import (
	"os"
	"fmt"
	"time"
	
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// ServiceConfig holds test service endpoints
type ServiceConfig struct {
	WebSocketURL  string
	RestAPIURL    string
	MockServerURL string
	logger        observability.Logger
}

// GetTestConfig returns test configuration following CLAUDE.md patterns
func GetTestConfig() *ServiceConfig {
	logger := observability.NewLogger("test-config")
	
	config := &ServiceConfig{
		WebSocketURL:  getEnvOrDefault("MCP_WEBSOCKET_URL", "ws://localhost:8080/ws"),
		RestAPIURL:    getEnvOrDefault("REST_API_URL", "http://localhost:8081"),
		MockServerURL: getEnvOrDefault("MOCKSERVER_URL", "http://localhost:8082"),
		logger:        logger,
	}
	
	// Log configuration for debugging (following CLAUDE.md)
	logger.Info("Test configuration loaded", map[string]interface{}{
		"websocket_url": config.WebSocketURL,
		"rest_api_url":  config.RestAPIURL,
		"mock_server":   config.MockServerURL,
	})
	
	return config
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetAuthHeaders returns common auth headers for tests
func GetAuthHeaders(apiKey string) map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", apiKey),
		"Content-Type": "application/json",
	}
}

// GetTestAPIKey returns the test API key for the given tenant
func GetTestAPIKey(tenantID string) string {
	// Map tenant IDs to their test API keys
	keys := map[string]string{
		"test-tenant-1": "test-key-tenant-1",
		"test-tenant-2": "test-key-tenant-2",
		"dev-tenant": "dev-admin-key-1234567890",
	}
	
	if key, exists := keys[tenantID]; exists {
		return key
	}
	
	// Default key for unknown tenants
	return "test-key-default"
}

// GetTestTimeout returns the standard test timeout
func GetTestTimeout() time.Duration {
	if timeout := os.Getenv("TEST_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			return d
		}
	}
	return 30 * time.Second
}
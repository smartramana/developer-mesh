package functional_test

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/joho/godotenv"
)

// Global test configuration
var (
	ServerURL     string
	APIKey        string
	MockServerURL string
)

func TestFunctional(t *testing.T) {
	// Load .env file if present (best practice for local/test environments)
	_ = godotenv.Load()

	// Register Gomega fail handler
	RegisterFailHandler(Fail)

	// Initialize test configuration from environment variables
	initTestConfig()

	// Run the test suite
	RunSpecs(t, "MCP Server Functional Test Suite")
}

// initTestConfig initializes test configuration from environment variables
func initTestConfig() {
	ServerURL = getEnvOrDefault("MCP_SERVER_URL", "http://localhost:8080")
	APIKey = getEnvOrDefault("MCP_API_KEY", "test-admin-api-key")
	MockServerURL = getEnvOrDefault("MOCKSERVER_URL", "http://localhost:8081")
}

// getEnvOrDefault retrieves environment variable value or returns default if not set
func getEnvOrDefault(key, defaultValue string) string {
	val := ""
	// Use standard os.Getenv here, not GinkgoT().Getenv which is not supported in this version
	val = os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return val
}

// SetupTimeout is the global timeout for setup operations
const SetupTimeout = 30 * time.Second

// RequestTimeout is the global timeout for HTTP requests
const RequestTimeout = 10 * time.Second

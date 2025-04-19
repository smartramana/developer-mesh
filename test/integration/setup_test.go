//go:build integration

package integration

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

var (
	serverAddr = "http://localhost:8080"
	apiKey     = "local-admin-api-key"
)

// TestMain handles test suite setup and teardown
func TestMain(m *testing.M) {
	// Initialize test environment
	if err := setupTestEnvironment(); err != nil {
		log.Fatalf("Failed to set up test environment: %v", err)
	}

	// Run tests
	code := m.Run()

	// Clean up test environment
	cleanupTestEnvironment()

	os.Exit(code)
}

// setupTestEnvironment initializes the test environment
func setupTestEnvironment() error {
	log.Println("Setting up integration test environment...")

	// Check if we're running in CI or local environment
	if addr := os.Getenv("MCP_SERVER_ADDR"); addr != "" {
		serverAddr = addr
	}

	if key := os.Getenv("MCP_API_KEY"); key != "" {
		apiKey = key
	}

	// Wait for a short time to ensure services are fully up
	time.Sleep(2 * time.Second)

	// Verify that required services are accessible
	if err := waitForServicesReady(); err != nil {
		return fmt.Errorf("services not ready: %v", err)
	}

	log.Println("Test environment ready")
	return nil
}

// waitForServicesReady checks if required services are accessible
func waitForServicesReady() error {
	// TODO: Implement service readiness checks
	return nil
}

// cleanupTestEnvironment performs cleanup after tests
func cleanupTestEnvironment() {
	log.Println("Cleaning up test environment...")
	// Nothing to clean up for now, but can add database cleanup if needed
}

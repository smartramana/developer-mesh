package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"

	_ "github.com/S-Corkum/devops-mcp/test/e2e/scenarios"
	"github.com/S-Corkum/devops-mcp/test/e2e/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	// Command-line flags
	configFile   = flag.String("config", "", "Path to configuration file")
	skipSetup    = flag.Bool("skip-setup", false, "Skip test setup")
	skipTeardown = flag.Bool("skip-teardown", false, "Skip test teardown")
	debug        = flag.Bool("debug", false, "Enable debug logging")
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	// Configure Ginkgo
	suiteConfig, reporterConfig := GinkgoConfiguration()

	// Configure reporter
	reporterConfig.Verbose = *debug

	RunSpecs(t, "DevOps MCP E2E Test Suite", suiteConfig, reporterConfig)
}

func init() {
	// Parse command-line flags in init to ensure they're available
	testing.Init()
	flag.Parse()
}

var _ = BeforeSuite(func() {
	// Skip if no API key is provided
	if os.Getenv("E2E_API_KEY") == "" {
		Skip("E2E_API_KEY not set, skipping E2E tests. Set E2E_API_KEY environment variable to run tests.")
	}

	if *skipSetup {
		return
	}

	fmt.Println("=== E2E Test Suite Setup ===")

	// Load configuration
	config := utils.LoadConfig()
	if *configFile != "" {
		// Override with config file if provided
		// TODO: Implementation would load from file
		_ = config // suppress unused warning for now
	}

	// Set debug mode
	if *debug {
		_ = os.Setenv("E2E_DEBUG", "true")
	}

	// Verify connectivity
	fmt.Printf("Testing connectivity to MCP: %s\n", config.MCPBaseURL)
	fmt.Printf("Testing connectivity to API: %s\n", config.APIBaseURL)

	// Create test directories
	if err := os.MkdirAll(config.ReportDir, 0755); err != nil {
		Fail(fmt.Sprintf("Failed to create report directory: %v", err))
	}

	fmt.Println("=== Setup Complete ===")
})

var _ = AfterSuite(func() {
	if *skipTeardown {
		return
	}

	fmt.Println("=== E2E Test Suite Teardown ===")

	// Cleanup any remaining resources
	// This would include:
	// - Closing any open connections
	// - Cleaning up test data
	// - Generating final reports

	fmt.Println("=== Teardown Complete ===")
})

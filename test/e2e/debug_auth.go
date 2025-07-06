package main

import (
	"fmt"
	"os"

	"github.com/S-Corkum/devops-mcp/test/e2e/utils"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(".env"); err != nil {
		fmt.Printf("Warning: Could not load .env file: %v\n", err)
	}

	// Load config
	config := utils.LoadConfig()

	fmt.Println("=== E2E Configuration Debug ===")
	fmt.Printf("MCP_BASE_URL from env: %s\n", os.Getenv("MCP_BASE_URL"))
	fmt.Printf("API_BASE_URL from env: %s\n", os.Getenv("API_BASE_URL"))
	fmt.Printf("E2E_API_KEY from env: %s\n", os.Getenv("E2E_API_KEY"))
	fmt.Println()
	fmt.Printf("Config MCPBaseURL: %s\n", config.MCPBaseURL)
	fmt.Printf("Config APIBaseURL: %s\n", config.APIBaseURL)
	fmt.Printf("Config APIKey: %s\n", config.APIKey)
	fmt.Printf("Config TenantID: %s\n", config.TenantID)
}

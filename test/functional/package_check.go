package functional

import (
	"fmt"
)

// Variables that should be accessible to test files
var (
	ServerURL    = "http://localhost:8080"
	APIKey       = "test-admin-api-key"
	MockServerURL = "http://localhost:8081"
)

func init() {
	fmt.Println("Package functional initialized with ServerURL:", ServerURL)
}

package functional

import (
	"fmt"
)

// Variables that should be accessible to test files
var (
	ServerURL    = "http://localhost:8081"
	APIKey       = "test-admin-api-key"
	MockServerURL = "http://localhost:8082"
)

func init() {
	fmt.Println("Package functional initialized with ServerURL:", ServerURL)
}

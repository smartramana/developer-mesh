package api

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware is a deprecated function that should not be used
// This stub is provided to catch any code still trying to use the old middleware
func AuthMiddleware() gin.HandlerFunc {
	// Check if we're in test mode
	if os.Getenv("MCP_TEST_MODE") == "true" && os.Getenv("TEST_AUTH_ENABLED") == "true" {
		// In test mode, panic with a clear message to help identify the caller
		panic("AuthMiddleware() is deprecated. Use the centralized auth service via authMiddleware.GinMiddleware()")
	}

	// In production, panic with the exact error message we're seeing
	panic("Deprecated AuthMiddleware called outside of test mode. Use centralized auth service.")
}

// InitAPIKeys is deprecated - API keys are now managed by the centralized auth service
func InitAPIKeys(keys map[string]string) {
	panic(fmt.Sprintf("InitAPIKeys is deprecated. Configure API keys through the auth service. Attempted to init %d keys", len(keys)))
}

// InitJWT is deprecated - JWT is now managed by the centralized auth service
func InitJWT(secret string) {
	panic("InitJWT is deprecated. Configure JWT through the auth service.")
}

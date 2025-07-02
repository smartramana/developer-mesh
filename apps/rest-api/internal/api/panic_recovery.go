package api

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
)

// CustomRecoveryMiddleware provides enhanced panic recovery with detailed error messages
func CustomRecoveryMiddleware(logger observability.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				stackTrace := string(debug.Stack())
				logger.Error("Panic recovered", map[string]interface{}{
					"error":       fmt.Sprintf("%v", err),
					"path":        c.Request.URL.Path,
					"method":      c.Request.Method,
					"stack":       stackTrace,
					"test_mode":   os.Getenv("MCP_TEST_MODE"),
					"auth_mode":   os.Getenv("TEST_AUTH_ENABLED"),
					"environment": os.Getenv("ENVIRONMENT"),
				})

				// Check if this is a specific auth-related panic
				errStr := fmt.Sprintf("%v", err)
				if errStr == "Deprecated AuthMiddleware called outside of test mode. Use centralized auth service." {
					// This is our specific error - return it as-is
					c.JSON(http.StatusInternalServerError, gin.H{"error": errStr})
					c.Abort()
					return
				}

				// For other panics, return a generic error in production
				if os.Getenv("ENVIRONMENT") == "production" {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": "An internal server error occurred",
					})
				} else {
					// In non-production, include more details
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": fmt.Sprintf("Internal server error: %v", err),
						"type":  "panic",
					})
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

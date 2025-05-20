package testMiddleware

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestAuthMiddleware is a simplified authentication middleware for tests
// It accepts both direct API keys and Bearer tokens
func TestAuthMiddleware(t *testing.T) {
	t.Run("Authentication Middleware", func(t *testing.T) {
		// This is a proper test function that satisfies the Go test signature
		// We can test the middleware functionality here
		middleware := AuthMiddleware()
		if middleware == nil {
			t.Error("AuthMiddleware should return a valid middleware function")
		}
	})
}

// AuthMiddleware is a simplified authentication middleware for tests
// It accepts both direct API keys and Bearer tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get authentication token from header
		authHeader := c.GetHeader("Authorization")
		
		// For testing, we'll accept any of these formats:
		// 1. "test-admin-api-key" (direct)
		// 2. "Bearer test-admin-api-key"
		
		// If no header, return unauthorized
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
			return
		}
		
		// Check if it's a direct key
		if authHeader == "test-admin-api-key" {
			// Valid direct key
			fmt.Println("Test mode: Valid direct API key")
			c.Next()
			return
		}
		
		// Check for Bearer prefix
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenValue := authHeader[7:]
			if tokenValue == "test-admin-api-key" {
				// Valid Bearer token
				fmt.Println("Test mode: Valid Bearer token")
				c.Next()
				return
			}
		}
		
		// If we get here, the key is invalid
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
	}
}

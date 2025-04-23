package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// TestAuthMiddleware is a simplified authentication middleware for tests
// It accepts both direct API keys and Bearer tokens
func TestAuthMiddleware() gin.HandlerFunc {
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

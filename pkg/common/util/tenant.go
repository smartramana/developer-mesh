package util

import "github.com/gin-gonic/gin"

// GetTenantIDFromContext extracts the tenant ID from the Gin context (from AuthMiddleware)
func GetTenantIDFromContext(c *gin.Context) string {
	// First try direct tenant_id key (set by ExtractTenantContext middleware)
	if tenantID, exists := c.Get("tenant_id"); exists {
		if tid, ok := tenantID.(string); ok && tid != "" {
			return tid
		}
	}
	
	// Fallback to user object for backward compatibility
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(map[string]any); ok {
			if tid, ok := u["tenant_id"].(string); ok {
				return tid
			}
		}
	}
	return ""
}

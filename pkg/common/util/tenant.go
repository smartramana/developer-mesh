package util

import "github.com/gin-gonic/gin"

// GetTenantIDFromContext extracts the tenant ID from the Gin context (from AuthMiddleware)
func GetTenantIDFromContext(c *gin.Context) string {
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(map[string]interface{}); ok {
			if tid, ok := u["tenant_id"].(string); ok {
				return tid
			}
		}
	}
	return ""
}

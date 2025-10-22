package api

import (
	"net/http"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
)

// APIKeyHandler handles API key management endpoints
type APIKeyHandler struct {
	authService *auth.Service
	logger      observability.Logger
	auditLogger *auth.AuditLogger
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(authService *auth.Service, logger observability.Logger, auditLogger *auth.AuditLogger) *APIKeyHandler {
	return &APIKeyHandler{
		authService: authService,
		logger:      logger,
		auditLogger: auditLogger,
	}
}

// RegisterRoutes registers the API key routes
func (h *APIKeyHandler) RegisterRoutes(router *gin.RouterGroup) {
	apiKeys := router.Group("/api-keys")
	{
		apiKeys.POST("", h.CreateAPIKey)
		apiKeys.GET("", h.ListAPIKeys)
		apiKeys.DELETE("/:id", h.RevokeAPIKey)
	}
}

// CreateAPIKeyRequest represents the request to create an API key
type CreateAPIKeyRequest struct {
	Name    string        `json:"name" binding:"required"`
	KeyType auth.KeyType  `json:"key_type" binding:"required"`
	Scopes  []string      `json:"scopes,omitempty"`
}

// CreateAPIKey creates a new API key for the authenticated user
// @Summary Create a new API key
// @Description Creates a new API key for the authenticated user
// @Tags api-keys
// @Accept json
// @Produce json
// @Param request body CreateAPIKeyRequest true "API key creation request"
// @Success 201 {object} map[string]interface{} "API key created successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/api-keys [post]
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	// Get authenticated user from context
	user, exists := auth.GetUserFromContext(c)
	if !exists || user == nil {
		h.logger.Warn("No authenticated user found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid API key creation request", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	// Validate key type
	validTypes := []auth.KeyType{auth.KeyTypeUser, auth.KeyTypeAdmin, auth.KeyTypeAgent}
	isValid := false
	for _, vt := range validTypes {
		if req.KeyType == vt {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid key type. Allowed types: user, admin, agent",
		})
		return
	}

	// Create API key
	apiKey, err := h.authService.CreateAPIKeyWithType(c.Request.Context(), auth.CreateAPIKeyRequest{
		Name:     req.Name,
		TenantID: user.TenantID.String(),
		UserID:   user.ID.String(),
		KeyType:  req.KeyType,
		Scopes:   req.Scopes,
	})
	if err != nil {
		h.logger.Error("Failed to create API key", map[string]interface{}{
			"error":     err.Error(),
			"user_id":   user.ID.String(),
			"tenant_id": user.TenantID.String(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create API key",
		})
		return
	}

	// Audit log
	h.auditLogger.LogAPIKeyCreated(c.Request.Context(), user.ID.String(), user.TenantID.String(), apiKey.KeyPrefix)

	h.logger.Info("API key created successfully", map[string]interface{}{
		"user_id":    user.ID.String(),
		"tenant_id":  user.TenantID.String(),
		"key_type":   req.KeyType,
		"key_prefix": apiKey.KeyPrefix,
	})

	// Return the API key (this is the ONLY time it will be visible)
	c.JSON(http.StatusCreated, gin.H{
		"message": "API key created successfully. Save this key - it will not be shown again!",
		"api_key": apiKey.Key,
		"info": gin.H{
			"key_prefix": apiKey.KeyPrefix,
			"name":       apiKey.Name,
			"key_type":   apiKey.KeyType,
			"scopes":     apiKey.Scopes,
			"created_at": apiKey.CreatedAt,
		},
	})
}

// ListAPIKeys lists all API keys for the authenticated user
// @Summary List API keys
// @Description Returns all API keys for the authenticated user (without the actual key values)
// @Tags api-keys
// @Produce json
// @Success 200 {object} map[string]interface{} "List of API keys"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/api-keys [get]
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	// Get authenticated user from context
	user, exists := auth.GetUserFromContext(c)
	if !exists || user == nil {
		h.logger.Warn("No authenticated user found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	keys, err := h.authService.ListUserAPIKeys(
		c.Request.Context(),
		user.TenantID.String(),
		user.ID.String(),
	)
	if err != nil {
		h.logger.Error("Failed to list API keys", map[string]interface{}{
			"error":     err.Error(),
			"user_id":   user.ID.String(),
			"tenant_id": user.TenantID.String(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list API keys",
		})
		return
	}

	h.logger.Debug("Listed API keys", map[string]interface{}{
		"user_id":   user.ID.String(),
		"tenant_id": user.TenantID.String(),
		"count":     len(keys),
	})

	c.JSON(http.StatusOK, gin.H{
		"api_keys": keys,
		"count":    len(keys),
	})
}

// RevokeAPIKey revokes (deactivates) an API key
// @Summary Revoke an API key
// @Description Revokes an API key by marking it as inactive
// @Tags api-keys
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]interface{} "API key revoked successfully"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "API key not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/api-keys/{id} [delete]
func (h *APIKeyHandler) RevokeAPIKey(c *gin.Context) {
	// Get authenticated user from context
	user, exists := auth.GetUserFromContext(c)
	if !exists || user == nil {
		h.logger.Warn("No authenticated user found in context", nil)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "API key ID is required",
		})
		return
	}

	err := h.authService.RevokeAPIKeyByID(
		c.Request.Context(),
		user.TenantID.String(),
		user.ID.String(),
		keyID,
	)
	if err != nil {
		if err.Error() == "API key not found or not authorized" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "API key not found or you don't have permission to revoke it",
			})
			return
		}

		h.logger.Error("Failed to revoke API key", map[string]interface{}{
			"error":     err.Error(),
			"user_id":   user.ID.String(),
			"tenant_id": user.TenantID.String(),
			"key_id":    keyID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to revoke API key",
		})
		return
	}

	// Audit log
	h.auditLogger.LogAPIKeyRevoked(c.Request.Context(), user.ID.String(), user.TenantID.String(), keyID)

	h.logger.Info("API key revoked successfully", map[string]interface{}{
		"user_id":   user.ID.String(),
		"tenant_id": user.TenantID.String(),
		"key_id":    keyID,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "API key revoked successfully",
		"key_id":  keyID,
	})
}

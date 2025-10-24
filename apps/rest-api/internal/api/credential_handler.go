package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	pkgrepository "github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/services"
)

// CredentialHandler handles user credential management endpoints
type CredentialHandler struct {
	credService   *services.CredentialService
	logger        observability.Logger
	metricsClient observability.MetricsClient
	auditLogger   *auth.AuditLogger
	templateRepo  pkgrepository.ToolTemplateRepository
	orgToolRepo   pkgrepository.OrganizationToolRepository
}

// NewCredentialHandler creates a new credential handler
func NewCredentialHandler(
	credService *services.CredentialService,
	logger observability.Logger,
	metricsClient observability.MetricsClient,
	auditLogger *auth.AuditLogger,
	templateRepo pkgrepository.ToolTemplateRepository,
	orgToolRepo pkgrepository.OrganizationToolRepository,
) *CredentialHandler {
	return &CredentialHandler{
		credService:   credService,
		logger:        logger,
		metricsClient: metricsClient,
		auditLogger:   auditLogger,
		templateRepo:  templateRepo,
		orgToolRepo:   orgToolRepo,
	}
}

// RegisterRoutes registers all credential API routes
func (h *CredentialHandler) RegisterRoutes(router *gin.RouterGroup) {
	credentials := router.Group("/credentials")
	{
		// User-facing endpoints (protected by auth middleware)
		credentials.POST("", h.StoreCredentials)
		credentials.GET("", h.ListCredentials)
		credentials.DELETE("/:serviceType", h.DeleteCredentials)
		credentials.POST("/:serviceType/validate", h.ValidateCredential)
	}

	// Internal endpoint for edge-mcp to fetch user credentials
	router.GET("/internal/users/:userId/credentials", h.GetAllUserCredentials)
	router.GET("/internal/users/:userId/credentials/:serviceType", h.GetUserCredential)
}

// StoreCredentials stores encrypted credentials for a user
// @Summary Store service credentials
// @Description Store encrypted credentials for third-party service integrations
// @Tags Credentials
// @Accept json
// @Produce json
// @Param request body models.CredentialPayload true "Credentials to store"
// @Success 201 {object} models.CredentialResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/credentials [post]
func (h *CredentialHandler) StoreCredentials(c *gin.Context) {
	start := time.Now()

	// Get tenant and user from context (set by auth middleware)
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if tenantID == "" || userID == "" {
		h.recordMetric(c, "credentials.store.error", 1, map[string]string{"error": "missing_auth"})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Parse request
	var payload models.CredentialPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.recordMetric(c, "credentials.store.error", 1, map[string]string{"error": "invalid_request"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Validate credentials before storing
	if err := h.credService.ValidateCredentials(c.Request.Context(), payload.ServiceType, payload.Credentials); err != nil {
		h.recordMetric(c, "credentials.store.error", 1, map[string]string{"error": "validation_failed"})
		c.JSON(http.StatusBadRequest, gin.H{"error": "Credential validation failed", "details": err.Error()})
		return
	}

	// Get client info for audit trail
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Store credentials
	response, err := h.credService.StoreCredentials(
		c.Request.Context(),
		tenantID,
		userID,
		&payload,
		ipAddress,
		userAgent,
	)
	if err != nil {
		h.logger.Error("Failed to store credentials", map[string]interface{}{
			"error":        err.Error(),
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": payload.ServiceType,
		})
		h.recordMetric(c, "credentials.store.error", 1, map[string]string{"error": "storage_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store credentials"})
		return
	}

	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.LogAuthAttempt(c.Request.Context(), auth.AuditEvent{
			EventType: "credential.stored",
			TenantID:  tenantID,
			UserID:    userID,
			AuthType:  "credential",
			Success:   true,
			IPAddress: ipAddress,
			UserAgent: userAgent,
			Metadata: map[string]interface{}{
				"service_type": payload.ServiceType,
			},
		})
	}

	// Auto-create organization tool if this is a standard service (GitHub, Harness, etc.)
	if h.shouldAutoCreateOrgTool(payload.ServiceType) {
		if err := h.autoCreateOrganizationTool(c.Request.Context(), tenantID, userID, payload.ServiceType); err != nil {
			h.logger.Warn("Failed to auto-create organization tool", map[string]interface{}{
				"error":        err.Error(),
				"tenant_id":    tenantID,
				"user_id":      userID,
				"service_type": payload.ServiceType,
			})
			// Don't fail the request - credentials were stored successfully
		}
	}

	h.recordMetric(c, "credentials.store.success", 1, map[string]string{"service_type": string(payload.ServiceType)})
	h.recordDuration(c, "credentials.store.duration", time.Since(start))

	c.JSON(http.StatusCreated, response)
}

// ListCredentials returns all configured credentials for a user
// @Summary List user credentials
// @Description List all configured service credentials (without sensitive data)
// @Tags Credentials
// @Produce json
// @Success 200 {array} models.CredentialResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/credentials [get]
func (h *CredentialHandler) ListCredentials(c *gin.Context) {
	start := time.Now()

	// Get tenant and user from context
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if tenantID == "" || userID == "" {
		h.recordMetric(c, "credentials.list.error", 1, map[string]string{"error": "missing_auth"})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Fetch credentials
	credentials, err := h.credService.ListCredentials(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("Failed to list credentials", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
			"user_id":   userID,
		})
		h.recordMetric(c, "credentials.list.error", 1, map[string]string{"error": "fetch_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list credentials"})
		return
	}

	h.recordMetric(c, "credentials.list.success", 1, map[string]string{"count": string(rune(len(credentials)))})
	h.recordDuration(c, "credentials.list.duration", time.Since(start))

	c.JSON(http.StatusOK, gin.H{
		"credentials": credentials,
		"count":       len(credentials),
	})
}

// DeleteCredentials removes credentials for a specific service
// @Summary Delete service credentials
// @Description Remove stored credentials for a specific service
// @Tags Credentials
// @Produce json
// @Param serviceType path string true "Service type (github, harness, aws, etc.)"
// @Success 204 "No Content"
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/credentials/{serviceType} [delete]
func (h *CredentialHandler) DeleteCredentials(c *gin.Context) {
	start := time.Now()

	// Get tenant and user from context
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if tenantID == "" || userID == "" {
		h.recordMetric(c, "credentials.delete.error", 1, map[string]string{"error": "missing_auth"})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get service type from path
	serviceType := models.ServiceType(c.Param("serviceType"))

	// Get client info for audit trail
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Delete credentials
	if err := h.credService.DeleteCredentials(
		c.Request.Context(),
		tenantID,
		userID,
		serviceType,
		ipAddress,
		userAgent,
	); err != nil {
		h.logger.Error("Failed to delete credentials", map[string]interface{}{
			"error":        err.Error(),
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": serviceType,
		})
		h.recordMetric(c, "credentials.delete.error", 1, map[string]string{"error": "delete_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete credentials"})
		return
	}

	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.LogAuthAttempt(c.Request.Context(), auth.AuditEvent{
			EventType: "credential.deleted",
			TenantID:  tenantID,
			UserID:    userID,
			AuthType:  "credential",
			Success:   true,
			IPAddress: ipAddress,
			UserAgent: userAgent,
			Metadata: map[string]interface{}{
				"service_type": serviceType,
			},
		})
	}

	h.recordMetric(c, "credentials.delete.success", 1, map[string]string{"service_type": string(serviceType)})
	h.recordDuration(c, "credentials.delete.duration", time.Since(start))

	c.Status(http.StatusNoContent)
}

// GetAllUserCredentials retrieves all decrypted credentials for a user (internal endpoint for edge-mcp)
// @Summary Get all user credentials (internal)
// @Description Retrieve all decrypted credentials for a user - for edge-mcp use only
// @Tags Internal
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]models.DecryptedCredentials
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/internal/users/{userId}/credentials [get]
func (h *CredentialHandler) GetAllUserCredentials(c *gin.Context) {
	start := time.Now()

	// Get tenant from context (must be authenticated)
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		h.recordMetric(c, "credentials.get_all.error", 1, map[string]string{"error": "missing_auth"})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get user ID from path
	userID := c.Param("userId")

	// Fetch all credentials
	credentials, err := h.credService.GetAllUserCredentials(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.Error("Failed to get all credentials", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
			"user_id":   userID,
		})
		h.recordMetric(c, "credentials.get_all.error", 1, map[string]string{"error": "fetch_failed"})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch credentials"})
		return
	}

	h.recordMetric(c, "credentials.get_all.success", 1, map[string]string{"count": string(rune(len(credentials)))})
	h.recordDuration(c, "credentials.get_all.duration", time.Since(start))

	c.JSON(http.StatusOK, credentials)
}

// GetUserCredential retrieves a specific decrypted credential (internal endpoint for edge-mcp)
// @Summary Get user credential (internal)
// @Description Retrieve decrypted credential for a specific service - for edge-mcp use only
// @Tags Internal
// @Produce json
// @Param userId path string true "User ID"
// @Param serviceType path string true "Service type"
// @Success 200 {object} models.DecryptedCredentials
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/internal/users/{userId}/credentials/{serviceType} [get]
func (h *CredentialHandler) GetUserCredential(c *gin.Context) {
	start := time.Now()

	// Get tenant from context
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		h.recordMetric(c, "credentials.get.error", 1, map[string]string{"error": "missing_auth"})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get parameters
	userID := c.Param("userId")
	serviceType := models.ServiceType(c.Param("serviceType"))

	// Fetch credential
	credential, err := h.credService.GetCredentials(c.Request.Context(), tenantID, userID, serviceType)
	if err != nil {
		h.logger.Error("Failed to get credential", map[string]interface{}{
			"error":        err.Error(),
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": serviceType,
		})
		h.recordMetric(c, "credentials.get.error", 1, map[string]string{"error": "fetch_failed"})
		c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
		return
	}

	h.recordMetric(c, "credentials.get.success", 1, map[string]string{"service_type": string(serviceType)})
	h.recordDuration(c, "credentials.get.duration", time.Since(start))

	c.JSON(http.StatusOK, credential)
}

// ValidateCredential tests if stored credentials are valid by making a test API call
// @Summary Validate service credentials
// @Description Test if stored credentials work by making a test API call to the service
// @Tags Credentials
// @Produce json
// @Param serviceType path string true "Service type (github, harness, aws, etc.)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/credentials/{serviceType}/validate [post]
func (h *CredentialHandler) ValidateCredential(c *gin.Context) {
	start := time.Now()

	// Get tenant and user from context
	tenantID := c.GetString("tenant_id")
	userID := c.GetString("user_id")
	if tenantID == "" || userID == "" {
		h.recordMetric(c, "credentials.validate.error", 1, map[string]string{"error": "missing_auth"})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get service type from path
	serviceType := models.ServiceType(c.Param("serviceType"))

	// Get and decrypt credential
	decrypted, err := h.credService.GetCredentials(
		c.Request.Context(),
		tenantID,
		userID,
		serviceType,
	)
	if err != nil {
		h.logger.Error("Failed to get credential for validation", map[string]interface{}{
			"error":        err.Error(),
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": serviceType,
		})
		h.recordMetric(c, "credentials.validate.error", 1, map[string]string{"error": "not_found"})
		c.JSON(http.StatusNotFound, gin.H{"error": "Credential not found"})
		return
	}

	// Validate credentials based on service type
	valid, validationErr := h.validateServiceCredential(c.Request.Context(), serviceType, decrypted)

	// Get client info for audit trail
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Log audit trail
	if h.auditLogger != nil {
		h.auditLogger.LogAuthAttempt(c.Request.Context(), auth.AuditEvent{
			EventType: "credential.validated",
			TenantID:  tenantID,
			UserID:    userID,
			AuthType:  "credential",
			Success:   valid,
			IPAddress: ipAddress,
			UserAgent: userAgent,
			Metadata: map[string]interface{}{
				"service_type": serviceType,
				"valid":        valid,
			},
		})
	}

	if validationErr != nil {
		h.logger.Warn("Credential validation failed", map[string]interface{}{
			"error":        validationErr.Error(),
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": serviceType,
		})
		h.recordMetric(c, "credentials.validate.failed", 1, map[string]string{"service_type": string(serviceType)})
		c.JSON(http.StatusOK, gin.H{
			"valid":        false,
			"service_type": serviceType,
			"error":        validationErr.Error(),
		})
		return
	}

	h.recordMetric(c, "credentials.validate.success", 1, map[string]string{"service_type": string(serviceType)})
	h.recordDuration(c, "credentials.validate.duration", time.Since(start))

	c.JSON(http.StatusOK, gin.H{
		"valid":        valid,
		"service_type": serviceType,
		"message":      "Credentials validated successfully",
	})
}

// validateServiceCredential performs actual validation by making test API calls
func (h *CredentialHandler) validateServiceCredential(ctx context.Context, serviceType models.ServiceType, creds *models.DecryptedCredentials) (bool, error) {
	// TODO: Implement actual validation for each service type
	// For now, just check that credentials exist
	switch serviceType {
	case models.ServiceTypeGitHub:
		token, ok := creds.Credentials["token"]
		if !ok || token == "" {
			return false, fmt.Errorf("github token is missing")
		}
		// TODO: Make test API call to GitHub to validate token
		// For example: GET https://api.github.com/user
		return true, nil

	case models.ServiceTypeJira:
		token, hasToken := creds.Credentials["token"]
		_, hasEmail := creds.Credentials["email"]
		if (!hasToken || token == "") && (!hasEmail) {
			return false, fmt.Errorf("jira credentials are missing")
		}
		// TODO: Make test API call to Jira
		return true, nil

	case models.ServiceTypeSonarQube:
		token, ok := creds.Credentials["token"]
		if !ok || token == "" {
			return false, fmt.Errorf("sonarqube token is missing")
		}
		// TODO: Make test API call to SonarQube
		return true, nil

	case models.ServiceTypeArtifactory:
		token, hasToken := creds.Credentials["token"]
		apiKey, hasApiKey := creds.Credentials["api_key"]
		if (!hasToken || token == "") && (!hasApiKey || apiKey == "") {
			return false, fmt.Errorf("artifactory credentials are missing")
		}
		// TODO: Make test API call to Artifactory
		return true, nil

	case models.ServiceTypeJenkins:
		username, hasUsername := creds.Credentials["username"]
		apiToken, hasToken := creds.Credentials["api_token"]
		if (!hasUsername || username == "") || (!hasToken || apiToken == "") {
			return false, fmt.Errorf("jenkins credentials are missing")
		}
		// TODO: Make test API call to Jenkins
		return true, nil

	case models.ServiceTypeGitLab:
		token, ok := creds.Credentials["token"]
		if !ok || token == "" {
			return false, fmt.Errorf("gitlab token is missing")
		}
		// TODO: Make test API call to GitLab
		return true, nil

	case models.ServiceTypeBitbucket:
		token, hasToken := creds.Credentials["token"]
		username, hasUsername := creds.Credentials["username"]
		appPassword, hasAppPassword := creds.Credentials["app_password"]

		if (!hasToken || token == "") && ((!hasUsername || username == "") || (!hasAppPassword || appPassword == "")) {
			return false, fmt.Errorf("bitbucket credentials are missing")
		}
		// TODO: Make test API call to Bitbucket
		return true, nil

	case models.ServiceTypeConfluence:
		email, hasEmail := creds.Credentials["email"]
		apiToken, hasToken := creds.Credentials["api_token"]
		if (!hasEmail || email == "") || (!hasToken || apiToken == "") {
			return false, fmt.Errorf("confluence credentials are missing")
		}
		// TODO: Make test API call to Confluence
		return true, nil

	default:
		// For generic services, just ensure credentials exist
		if len(creds.Credentials) == 0 {
			return false, fmt.Errorf("credentials are empty")
		}
		return true, nil
	}
}

// Helper methods for metrics
func (h *CredentialHandler) recordMetric(c *gin.Context, name string, value float64, tags map[string]string) {
	if h.metricsClient != nil {
		// Add common labels
		if tags == nil {
			tags = make(map[string]string)
		}
		tags["endpoint"] = c.Request.URL.Path
		tags["method"] = c.Request.Method

		h.metricsClient.IncrementCounterWithLabels(name, value, tags)
	}
}

func (h *CredentialHandler) recordDuration(c *gin.Context, name string, duration time.Duration) {
	if h.metricsClient != nil {
		tags := map[string]string{
			"unit":     "ms",
			"endpoint": c.Request.URL.Path,
			"method":   c.Request.Method,
		}
		h.metricsClient.RecordHistogram(name, float64(duration.Milliseconds()), tags)
	}
}

// shouldAutoCreateOrgTool determines if we should auto-create an org tool for this service type
func (h *CredentialHandler) shouldAutoCreateOrgTool(serviceType models.ServiceType) bool {
	// Standard services that have tool templates in the database
	standardServices := []models.ServiceType{
		models.ServiceTypeGitHub,
		models.ServiceTypeHarness,
		models.ServiceTypeJira,
		models.ServiceTypeGitLab,
		models.ServiceTypeBitbucket,
		models.ServiceTypeArtifactory,
		models.ServiceTypeJenkins,
		models.ServiceTypeSonarQube,
		models.ServiceTypeConfluence,
	}

	for _, svc := range standardServices {
		if serviceType == svc {
			return true
		}
	}
	return false
}

// autoCreateOrganizationTool creates an OrganizationTool entry for the given service type
func (h *CredentialHandler) autoCreateOrganizationTool(ctx context.Context, tenantID, userID string, serviceType models.ServiceType) error {
	// Get organization_id from context (set by auth middleware)
	organizationID := ctx.Value("organization_id")
	if organizationID == nil || organizationID == "" {
		h.logger.Warn("organization_id not found in context, skipping org tool creation", map[string]interface{}{
			"tenant_id":    tenantID,
			"user_id":      userID,
			"service_type": serviceType,
		})
		return nil // Don't fail the request, just skip tool creation
	}
	orgID := organizationID.(string)

	// Check if organization tool already exists for this tenant and service type
	existing, err := h.orgToolRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to check existing org tools: %w", err)
	}

	// Check if we already have a tool for this service type
	for _, tool := range existing {
		// Get the template to check its provider name
		template, err := h.templateRepo.GetByID(ctx, tool.TemplateID)
		if err == nil && template != nil {
			// Check if template provider name matches service type (e.g., "github" provider for "github" service)
			if string(serviceType) == template.ProviderName {
				h.logger.Info("Organization tool already exists", map[string]interface{}{
					"tenant_id":    tenantID,
					"service_type": serviceType,
					"tool_id":      tool.ID,
				})
				return nil // Already exists, no need to create
			}
		}
	}

	// Find the tool template for this service type
	// Template provider names match service types (e.g., "github", "harness")
	template, err := h.templateRepo.GetByProviderName(ctx, string(serviceType))
	if err != nil {
		return fmt.Errorf("failed to find tool template for %s: %w", serviceType, err)
	}
	if template == nil {
		return fmt.Errorf("tool template not found for %s", serviceType)
	}

	// Create organization tool
	userIDPtr := &userID
	orgTool := &models.OrganizationTool{
		ID:             uuid.New().String(),
		OrganizationID: orgID,
		TenantID:       tenantID,
		TemplateID:     template.ID,
		InstanceName:   fmt.Sprintf("%s-integration", serviceType),
		DisplayName:    fmt.Sprintf("%s Integration", template.DisplayName),
		Description:    fmt.Sprintf("Auto-created %s integration using user credentials", template.DisplayName),
		InstanceConfig: map[string]interface{}{
			"baseUrl":  template.DefaultConfig.BaseURL,
			"authType": template.DefaultConfig.AuthType,
		},
		IsActive:  true,
		Status:    "active",
		CreatedBy: userIDPtr,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.orgToolRepo.Create(ctx, orgTool); err != nil {
		return fmt.Errorf("failed to create organization tool: %w", err)
	}

	h.logger.Info("Auto-created organization tool", map[string]interface{}{
		"tenant_id":    tenantID,
		"service_type": serviceType,
		"tool_id":      orgTool.ID,
		"template_id":  template.ID,
	})

	return nil
}

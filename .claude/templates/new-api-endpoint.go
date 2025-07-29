// Template for new API endpoint in REST API service
// Usage: Copy this template when adding a new endpoint

package api

import (
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/gin-gonic/gin"
)

// Handle{Resource} handles {HTTP_METHOD} /api/v1/{resource}
// @Summary {Brief description}
// @Description {Detailed description}
// @Tags {Resource}
// @Accept json
// @Produce json
// @Param {param} {param_type} {data_type} {required} "{description}"
// @Success 200 {object} {ResponseType}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/{resource} [{method}]
func (api *{Resource}API) Handle{Resource}(c *gin.Context) {
	start := time.Now()
	tenantID := c.GetString("tenant_id")
	
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id required"})
		return
	}

	// TODO: Add input validation
	var req {RequestType}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Call service layer
	result, err := api.service.{Method}(c.Request.Context(), tenantID, req)
	if err != nil {
		api.logger.Error("Failed to {action}", map[string]interface{}{
			"tenant_id": tenantID,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to {action}"})
		return
	}

	// Record metrics
	if api.metricsClient != nil {
		api.metricsClient.RecordHistogram("api.{resource}.{method}.duration", 
			float64(time.Since(start).Milliseconds()), 
			map[string]string{"tenant_id": tenantID})
	}

	c.JSON(http.StatusOK, result)
}
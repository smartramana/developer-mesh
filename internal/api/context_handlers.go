package api

import (
	"net/http"

	"github.com/S-Corkum/mcp-server/pkg/mcp"
	"github.com/gin-gonic/gin"
)

// UpdateContextHandler handles PUT requests to update an existing context
func (s *Server) UpdateContextHandler(c *gin.Context) {
	// Get context ID from URL parameter
	contextID := c.Param("id")
	if contextID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing context id"})
		return
	}

	// Parse request body
	var updateRequest struct {
		Content []mcp.ContextItem `json:"content"`
	}

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Get the current context
	currentContext, err := s.engine.ContextManager.GetContext(c.Request.Context(), contextID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "context not found: " + err.Error()})
		return
	}

	// Update content if provided
	if updateRequest.Content != nil {
		currentContext.Content = updateRequest.Content
	}

	// Update the context in the database
	updatedContext, err := s.engine.ContextManager.UpdateContext(
		c.Request.Context(),
		contextID,
		currentContext,
		&mcp.ContextUpdateOptions{
			Truncate:         true,
			TruncateStrategy: "oldest_first", // Default truncation strategy
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update context: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedContext)
}

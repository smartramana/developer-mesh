package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ToolAPI handles the DevOps tool integration API endpoints
type ToolAPI struct {
	server *Server
	adapterBridge interface{}
}

// NewToolAPI creates a new tool API handler
func NewToolAPI(adapterBridge interface{}) *ToolAPI {
	return &ToolAPI{
		adapterBridge: adapterBridge,
	}
}

// SetServer sets the server reference
func (api *ToolAPI) SetServer(server *Server) {
	api.server = server
}

// RegisterRoutes registers all tool API routes
func (api *ToolAPI) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/adapters", api.listAdapters)
	router.GET("/adapters/:name", api.getAdapterInfo)
	router.POST("/adapters/:name/query", api.queryAdapter)
	router.POST("/adapters/:name/action", api.executeAction)
}

// listAdapters lists all available adapters
func (api *ToolAPI) listAdapters(c *gin.Context) {
	if api.server == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server not initialized"})
		return
	}
	
	adapters := api.server.engine.ListAdapters()
	c.JSON(http.StatusOK, gin.H{"adapters": adapters})
}

// getAdapterInfo returns information about a specific adapter
func (api *ToolAPI) getAdapterInfo(c *gin.Context) {
	if api.server == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server not initialized"})
		return
	}
	
	adapterName := c.Param("name")
	adapter, err := api.server.engine.GetAdapter(adapterName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":   adapterName,
		"health": adapter.Health(),
	})
}

// queryAdapter queries data from an adapter
func (api *ToolAPI) queryAdapter(c *gin.Context) {
	if api.server == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server not initialized"})
		return
	}
	
	adapterName := c.Param("name")
	
	var request struct {
		Query map[string]interface{} `json:"query" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	adapter, err := api.server.engine.GetAdapter(adapterName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	
	result, err := adapter.GetData(c.Request.Context(), request.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, result)
}

// executeAction executes an action on an adapter
func (api *ToolAPI) executeAction(c *gin.Context) {
	if api.server == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Server not initialized"})
		return
	}
	
	adapterName := c.Param("name")
	
	var request struct {
		ContextID string                 `json:"context_id"`
		Action    string                 `json:"action" binding:"required"`
		Params    map[string]interface{} `json:"params"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	result, err := api.server.engine.ExecuteAdapterAction(
		c.Request.Context(),
		adapterName,
		request.ContextID,
		request.Action,
		request.Params,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, result)
}

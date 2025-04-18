package api

import (
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/gin-gonic/gin"
)

// VersionedContextAPI handles versioned context API endpoints
type VersionedContextAPI struct {
	v1API *ContextAPI
	// Add v2 API when needed
}

// NewVersionedContextAPI creates a new versioned context API handler
func NewVersionedContextAPI(contextManager interfaces.ContextManager) *VersionedContextAPI {
	return &VersionedContextAPI{
		v1API: NewContextAPI(contextManager),
		// Initialize v2 API when needed
	}
}

// RegisterRoutes registers all versioned context API routes
func (api *VersionedContextAPI) RegisterRoutes(router *gin.RouterGroup) {
	// Register /contexts endpoints
	router.POST("/contexts", api.createContext)
	router.GET("/contexts/:id", api.getContext)
	router.PUT("/contexts/:id", api.updateContext)
	router.DELETE("/contexts/:id", api.deleteContext)
	router.GET("/contexts", api.listContexts)
	router.POST("/contexts/:id/search", api.searchContext)
	router.GET("/contexts/:id/summary", api.summarizeContext)
}

// createContext creates a new context
func (api *VersionedContextAPI) createContext(c *gin.Context) {
	// Route to appropriate version handler
	handlers := NewVersionedHandlers().
		Add(APIVersionV1, api.v1API.createContext).
		// Add V2 handler when needed
		AddDefault(api.v1API.createContext)
	
	handlers.Handle(c)
}

// getContext retrieves a context by ID
func (api *VersionedContextAPI) getContext(c *gin.Context) {
	// Route to appropriate version handler
	handlers := NewVersionedHandlers().
		Add(APIVersionV1, api.v1API.getContext).
		// Add V2 handler when needed
		AddDefault(api.v1API.getContext)
	
	handlers.Handle(c)
}

// updateContext updates an existing context
func (api *VersionedContextAPI) updateContext(c *gin.Context) {
	// Route to appropriate version handler
	handlers := NewVersionedHandlers().
		Add(APIVersionV1, api.v1API.updateContext).
		// Add V2 handler when needed
		AddDefault(api.v1API.updateContext)
	
	handlers.Handle(c)
}

// deleteContext deletes a context
func (api *VersionedContextAPI) deleteContext(c *gin.Context) {
	// Route to appropriate version handler
	handlers := NewVersionedHandlers().
		Add(APIVersionV1, api.v1API.deleteContext).
		// Add V2 handler when needed
		AddDefault(api.v1API.deleteContext)
	
	handlers.Handle(c)
}

// listContexts lists contexts for an agent
func (api *VersionedContextAPI) listContexts(c *gin.Context) {
	// Route to appropriate version handler
	handlers := NewVersionedHandlers().
		Add(APIVersionV1, api.v1API.listContexts).
		// Add V2 handler when needed
		AddDefault(api.v1API.listContexts)
	
	handlers.Handle(c)
}

// searchContext searches within a context
func (api *VersionedContextAPI) searchContext(c *gin.Context) {
	// Route to appropriate version handler
	handlers := NewVersionedHandlers().
		Add(APIVersionV1, api.v1API.searchContext).
		// Add V2 handler when needed
		AddDefault(api.v1API.searchContext)
	
	handlers.Handle(c)
}

// summarizeContext generates a summary of a context
func (api *VersionedContextAPI) summarizeContext(c *gin.Context) {
	// Route to appropriate version handler
	handlers := NewVersionedHandlers().
		Add(APIVersionV1, api.v1API.summarizeContext).
		// Add V2 handler when needed
		AddDefault(api.v1API.summarizeContext)
	
	handlers.Handle(c)
}

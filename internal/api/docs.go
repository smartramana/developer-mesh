package api

import (
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"github.com/gin-gonic/gin"
)

// @title MCP Server API
// @version 1.0
// @description Model Context Protocol (MCP) Server API for AI agents with advanced context management and DevOps tool integrations. Provides a RESTful interface for storing, retrieving, and manipulating conversation contexts, as well as interacting with DevOps tools such as GitHub.
// @termsOfService https://github.com/S-Corkum/mcp-server/blob/main/LICENSE

// @contact.name API Support
// @contact.url https://github.com/S-Corkum/mcp-server/issues
// @contact.email support@example.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API key authentication for programmatic access

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the JWT token for authenticated sessions

// @tag.name contexts
// @tag.description Context management endpoints for storing, retrieving, and manipulating AI agent conversation contexts

// @tag.name tools
// @tag.description DevOps tool integration endpoints for interacting with GitHub and other tools

// @tag.name vectors
// @tag.description Vector embedding endpoints for semantic search and similarity matching

// @tag.name webhooks
// @tag.description Webhook endpoints for receiving events from external systems

// SetupSwaggerDocs configures the swagger documentation
func SetupSwaggerDocs(router *gin.Engine) {
	// Use swagger middleware
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// Here are some example Swagger annotations:

// @Summary Create a new context
// @Description Create a new conversation context for an AI agent
// @Tags contexts
// @Accept json
// @Produce json
// @Param context body object true "Context to create"
// @Success 201 {object} object "Created context"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts [post]
// Example for creating a context

// @Summary Get a context by ID
// @Description Retrieve an existing context by its ID
// @Tags contexts
// @Accept json
// @Produce json
// @Param id path string true "Context ID"
// @Success 200 {object} object "Context data"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Context not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts/{id} [get]
// Example for retrieving a context

// @Summary Update a context
// @Description Update an existing context
// @Tags contexts
// @Accept json
// @Produce json
// @Param id path string true "Context ID"
// @Param request body object true "Update request"
// @Success 200 {object} object "Updated context"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Context not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts/{id} [put]
// Example for updating a context

// @Summary Delete a context
// @Description Delete an existing context
// @Tags contexts
// @Accept json
// @Produce json
// @Param id path string true "Context ID"
// @Success 200 {object} object "Deletion confirmation"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Context not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /contexts/{id} [delete]
// Example for deleting a context

// @Summary Store embedding
// @Description Store a vector embedding for a context
// @Tags vectors
// @Accept json
// @Produce json
// @Param request body object true "Embedding data"
// @Success 200 {object} object "Stored embedding"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /vectors/store [post]
// Example for storing an embedding

// @Summary Execute tool action
// @Description Execute an action using a specific tool
// @Tags tools
// @Accept json
// @Produce json
// @Param tool path string true "Tool name"
// @Param action path string true "Action name"
// @Param context_id query string true "Context ID"
// @Param params body object true "Action parameters"
// @Success 200 {object} object "Action result"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Tool or action not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /tools/{tool}/actions/{action} [post]
// Example for executing a tool action

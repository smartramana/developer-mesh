// Package api MCP Server API
//
// The MCP Server implements the Model Context Protocol specification for AI agent integration with DevOps tools.
//
//	Schemes: https, http
//	BasePath: /api/v1
//	Version: 1.0.0
//	Host: api.mcp-server.example.com
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
//	Security:
//	- api_key:
//	- bearer:
//
//	SecurityDefinitions:
//	api_key:
//	  type: apiKey
//	  name: Authorization
//	  in: header
//	  description: API key authentication (with or without 'Bearer' prefix)
//
//	bearer:
//	  type: apiKey
//	  name: Authorization
//	  in: header
//	  description: JWT Bearer token authentication
//
// swagger:meta
package api

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupSwaggerDocs configures Swagger documentation endpoints
func SetupSwaggerDocs(router *gin.Engine, basePath string) {
	// Serve the OpenAPI specification
	router.Static("/docs/swagger", "../../docs/swagger")

	// Swagger UI
	url := ginSwagger.URL(basePath + "/docs/swagger/openapi.yaml")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

	// ReDoc alternative UI
	router.GET("/redoc", func(c *gin.Context) {
		c.HTML(200, "redoc.html", gin.H{
			"title":   "MCP Server API Documentation",
			"specUrl": basePath + "/docs/swagger/openapi.yaml",
		})
	})
}

// @title MCP Server API
// @version 1.0.0
// @description Model Context Protocol Server for AI-powered DevOps automation
// @termsOfService https://github.com/developer-mesh/developer-mesh/blob/main/LICENSE

// @contact.name API Support
// @contact.url https://github.com/developer-mesh/developer-mesh/issues
// @contact.email support@developer-mesh.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host api.mcp-server.example.com
// @BasePath /api/v1
// @schemes https http

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description API key authentication (with or without 'Bearer' prefix)

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer token authentication

// @tag.name Health
// @tag.description Health check and monitoring endpoints

// @tag.name Contexts
// @tag.description MCP context management operations

// @tag.name Tools
// @tag.description DevOps tool integration endpoints

// @tag.name GitHub
// @tag.description GitHub-specific tool operations

// @tag.name Harness
// @tag.description Harness CI/CD operations

// @tag.name SonarQube
// @tag.description SonarQube code quality operations

// @tag.name Artifactory
// @tag.description JFrog Artifactory operations

// @tag.name Xray
// @tag.description JFrog Xray security scanning

// @tag.name Agents
// @tag.description AI agent management

// @tag.name Models
// @tag.description AI model configuration

// @tag.name Vectors
// @tag.description Vector storage and search operations

// @tag.name Search
// @tag.description Semantic search endpoints

// @tag.name Webhooks
// @tag.description Webhook endpoints for external integrations

// @tag.name Relationships
// @tag.description Entity relationship management

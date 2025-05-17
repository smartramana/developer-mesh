package server

import (
	"github.com/S-Corkum/devops-mcp/internal/adapters/github"
	"github.com/S-Corkum/devops-mcp/internal/observability"
	"github.com/gin-gonic/gin"
)

// MCPServer represents a Model Context Protocol server
type MCPServer struct {
	router        *gin.Engine
	logger        *observability.Logger
	githubAdapter *github.GitHubAdapter
}

// Config contains configuration for the MCP server
type Config struct {
	// EnableGitHubTools controls whether GitHub tools are enabled
	EnableGitHubTools bool
}

// NewMCPServer creates a new MCP server
func NewMCPServer(
	logger *observability.Logger,
	githubAdapter *github.GitHubAdapter,
	config *Config,
) *MCPServer {
	// Create router
	router := gin.New()
	
	// Use recovery middleware
	router.Use(gin.Recovery())
	
	// Create server
	server := &MCPServer{
		router:        router,
		logger:        logger,
		githubAdapter: githubAdapter,
	}
	
	// Register tools
	server.registerTools(config)
	
	return server
}

// registerTools registers all enabled tools with the server
func (s *MCPServer) registerTools(config *Config) {
	// Register GitHub tools if enabled
	if config.EnableGitHubTools && s.githubAdapter != nil {
		s.logger.Info("Registering GitHub tools", nil)
		RegisterGitHubTools(s.router, s.githubAdapter, s.logger)
	}
}

// GetRouter returns the underlying Gin router
func (s *MCPServer) GetRouter() *gin.Engine {
	return s.router
}

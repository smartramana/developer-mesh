package tools

import (
	githubtools "github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/tools/github"

	"github.com/developer-mesh/developer-mesh/pkg/adapters/github"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
)

// MCPToolsServer represents a Model Context Protocol tools server
type MCPToolsServer struct {
	router        *gin.Engine
	logger        observability.Logger
	githubAdapter *github.GitHubAdapter
}

// Config contains configuration for the MCP tools server
type Config struct {
	// EnableGitHubTools controls whether GitHub tools are enabled
	EnableGitHubTools bool
}

// NewMCPToolsServer creates a new MCP tools server
func NewMCPToolsServer(
	logger observability.Logger,
	githubAdapter *github.GitHubAdapter,
	config *Config,
) *MCPToolsServer {
	// Create router
	router := gin.New()

	// Use recovery middleware
	router.Use(gin.Recovery())

	// Create server
	server := &MCPToolsServer{
		router:        router,
		logger:        logger,
		githubAdapter: githubAdapter,
	}

	// Register tools
	server.registerTools(config)

	return server
}

// registerTools registers all enabled tools with the server
func (s *MCPToolsServer) registerTools(config *Config) {
	// Register GitHub tools if enabled
	if config.EnableGitHubTools && s.githubAdapter != nil {
		s.logger.Info("Registering GitHub tools", nil)
		githubtools.RegisterGitHubTools(s.router, s.githubAdapter, s.logger)
	}
}

// GetRouter returns the underlying Gin router
func (s *MCPToolsServer) GetRouter() *gin.Engine {
	return s.router
}

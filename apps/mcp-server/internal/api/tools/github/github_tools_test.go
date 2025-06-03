package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"mcp-server/internal/core/tool"
)

// MockGitHubAdapter is a mock implementation of the GitHub API for testing
type MockGitHubAdapter struct {
	mock.Mock
}

// ExecuteAction is a mock implementation of the ExecuteAction method
func (m *MockGitHubAdapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	args := m.Called(ctx, contextID, action, params)
	return args.Get(0), args.Error(1)
}

// Type is a mock implementation of the Type method
func (m *MockGitHubAdapter) Type() string {
	args := m.Called()
	return args.String(0)
}

// Version is a mock implementation of the Version method
func (m *MockGitHubAdapter) Version() string {
	args := m.Called()
	return args.String(0)
}

// Health is a mock implementation of the Health method
func (m *MockGitHubAdapter) Health() string {
	args := m.Called()
	return args.String(0)
}

// Close is a mock implementation of the Close method
func (m *MockGitHubAdapter) Close() error {
	args := m.Called()
	return args.Error(0)
}

// newTestGitHubToolsHandler creates a test-only constructor
func newTestGitHubToolsHandler(mockAdapter *MockGitHubAdapter, logger observability.Logger) *GitHubToolsHandler {
	registry := tool.NewToolRegistry()

	// Add some test tools to the registry
	err := registry.RegisterTool(&tool.Tool{
		Definition: tool.ToolDefinition{
			Name: "mcp0_get_repository",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"owner": {Type: "string"},
					"repo":  {Type: "string"},
				},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return mockAdapter.ExecuteAction(context.Background(), "default", "getRepository", params)
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to register test tool: %v", err))
	}

	// We're creating the handler directly here
	return &GitHubToolsHandler{
		registry: registry,
		logger:   logger,
	}
}

func TestGitHubToolsHandler_ListTools(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockAdapter := new(MockGitHubAdapter)
	// Create an observability logger directly
	logger := observability.NewLogger("test")

	// Create handler using our test constructor
	handler := newTestGitHubToolsHandler(mockAdapter, logger)

	// Create router
	router := gin.New()
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)

	// Create request
	req, _ := http.NewRequest("GET", "/api/v1/tools/github", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response ToolResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Validate response
	assert.True(t, response.Success)
	assert.NotNil(t, response.Data)

	// Tools should be in the response
	tools, ok := response.Data.([]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, tools)
}

func TestGitHubToolsHandler_GetToolSchema(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockAdapter := new(MockGitHubAdapter)
	logger := observability.NewLogger("test")

	// Create handler using our test constructor
	handler := newTestGitHubToolsHandler(mockAdapter, logger)

	// Create router
	router := gin.New()
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)

	// Create request
	req, _ := http.NewRequest("GET", "/api/v1/tools/github/mcp0_get_repository", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response ToolResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Validate response
	assert.True(t, response.Success)
	assert.NotNil(t, response.Data)

	// Tool definition should be in the response
	toolDef, ok := response.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "mcp0_get_repository", toolDef["name"])
}

func TestGitHubToolsHandler_ExecuteTool(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	mockAdapter := new(MockGitHubAdapter)
	logger := observability.NewLogger("test")

	// Setup mock expectations
	mockAdapter.On("ExecuteAction", mock.Anything, "default", "getRepository",
		map[string]interface{}{
			"owner": "octocat",
			"repo":  "hello-world",
		}).Return(map[string]interface{}{
		"id":        12345,
		"name":      "hello-world",
		"full_name": "octocat/hello-world",
	}, nil)

	// Create handler using our test constructor
	handler := newTestGitHubToolsHandler(mockAdapter, logger)

	// Create router
	router := gin.New()
	apiGroup := router.Group("/api/v1")
	handler.RegisterRoutes(apiGroup)

	// Create request body
	reqBody := ToolExecuteRequest{
		Parameters: map[string]interface{}{
			"owner": "octocat",
			"repo":  "hello-world",
		},
	}
	reqJSON, _ := json.Marshal(reqBody)

	// Create request
	req, _ := http.NewRequest("POST", "/api/v1/tools/github/mcp0_get_repository", bytes.NewBuffer(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response
	var response ToolResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Validate response
	assert.True(t, response.Success)
	assert.NotNil(t, response.Data)

	// Repository data should be in the response
	repoData, ok := response.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(12345), repoData["id"])
	assert.Equal(t, "hello-world", repoData["name"])
	assert.Equal(t, "octocat/hello-world", repoData["full_name"])

	// Verify mock expectations
	mockAdapter.AssertExpectations(t)
}

func TestToolRegistry_Integration(t *testing.T) {
	// Create registry
	registry := tool.NewToolRegistry()

	// Mock definition of a tool
	mockTool := &tool.Tool{
		Definition: tool.ToolDefinition{
			Name:        "test_tool",
			Description: "Test tool for integration",
			Parameters: tool.ParameterSchema{
				Type: "object",
				Properties: map[string]tool.PropertySchema{
					"name": {
						Type:        "string",
						Description: "Name parameter",
					},
				},
				Required: []string{"name"},
			},
		},
		Handler: func(params map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"result": "Success",
				"name":   params["name"],
			}, nil
		},
	}

	// Register tool
	err := registry.RegisterTool(mockTool)
	assert.NoError(t, err)

	// Get tool
	retrievedTool, err := registry.GetTool("test_tool")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedTool)

	// Validate tool
	assert.Equal(t, "test_tool", retrievedTool.Definition.Name)

	// Test parameter validation
	err = retrievedTool.ValidateParams(map[string]interface{}{
		"name": "test",
	})
	assert.NoError(t, err)

	// Test handler
	result, err := retrievedTool.Handler(map[string]interface{}{
		"name": "test",
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"result": "Success",
		"name":   "test",
	}, result)

	// Generate schema JSON
	schemaJSON, err := GenerateToolSchemaJSON(registry)
	assert.NoError(t, err)
	assert.NotEmpty(t, schemaJSON)
}

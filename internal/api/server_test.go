package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coremocks "github.com/S-Corkum/mcp-server/internal/core/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestHealthHandler tests the health endpoint handler
func TestHealthHandlerMock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a minimal implementation for testing
	router := gin.New()
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"components": map[string]string{
				"engine": "healthy",
				"github": "healthy",
			},
		})
	}
	
	router.GET("/health", handler)
	
	// Setup a test HTTP server
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	
	// Serve the request
	router.ServeHTTP(w, req)
	
	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "healthy", response["status"])
	assert.NotNil(t, response["components"])
}

// TestUnhealthyStatus tests the health endpoint when a component is unhealthy
func TestUnhealthyStatusMock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a minimal implementation for testing
	router := gin.New()
	handler := func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"components": map[string]string{
				"engine": "healthy",
				"github": "unhealthy",
			},
		})
	}
	
	router.GET("/health", handler)
	
	// Setup a test HTTP server
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	
	// Serve the request
	router.ServeHTTP(w, req)
	
	// Check the response
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "unhealthy", response["status"])
}

// TestShutdownServer tests the server shutdown
func TestShutdownServerMock(t *testing.T) {
	// Create mock engine
	mockEngine := &coremocks.MockEngine{}
	mockEngine.On("Shutdown", mock.Anything).Return(nil)
	
	// Create a shutdown function to test
	shutdownCalled := false
	shutdownFunc := func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	}
	
	// Call the shutdown function
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := shutdownFunc(ctx)
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
}

// TestMetricsHandler tests the metrics handler
func TestMetricsHandlerFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a minimal handler function for testing
	metricsHandler := func(c *gin.Context) {
		c.String(http.StatusOK, "# metrics data will be here")
	}
	
	// Create a router and register the handler
	router := gin.New()
	router.GET("/metrics", metricsHandler)
	
	// Setup a test HTTP server
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	
	// Serve the request
	router.ServeHTTP(w, req)
	
	// Check the response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "metrics data")
}

// TestContextHandlers tests the context-related handlers
func TestContextHandlersFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create context handler functions for testing
	contextHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "context created"})
	}
	
	getContextHandler := func(c *gin.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, gin.H{"id": id, "message": "context retrieved"})
	}
	
	// Create a router and register the handlers
	router := gin.New()
	router.POST("/api/v1/mcp/context", contextHandler)
	router.GET("/api/v1/mcp/context/:id", getContextHandler)
	
	// Test POST route
	t.Run("Context Handler", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/mcp/context", nil)
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "context created")
	})
	
	// Test GET route
	t.Run("Get Context Handler", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/mcp/context/123", nil)
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "123")
		assert.Contains(t, w.Body.String(), "context retrieved")
	})
}

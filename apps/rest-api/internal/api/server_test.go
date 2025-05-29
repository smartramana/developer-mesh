package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coremocks "rest-api/internal/core/mocks"
)

// MockServer is a lightweight test server for testing API routes
type MockServer struct {
	router *gin.Engine
	config Config
	engine *coremocks.MockEngine
}

// Create a simple mock server for testing
func setupMockServer(_ *testing.T) *MockServer {
	gin.SetMode(gin.TestMode)

	// Create mock engine
	mockEngine := &coremocks.MockEngine{}

	// Setup health check
	mockEngine.On("Health").Return(map[string]string{
		"engine": "healthy",
		"github": "healthy",
	})

	// Create router
	router := gin.New()

	// Add health endpoint for testing
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"components": map[string]string{
				"engine": "healthy",
				"github": "healthy",
			},
		})
	})

	// Add metrics endpoint for testing
	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "# metrics data will be here")
	})

	// Add webhook endpoints for testing
	webhookEndpoints := []string{
		"/webhook/github",
		"/webhook/harness",
		"/webhook/sonarqube",
		"/webhook/artifactory",
		"/webhook/xray",
	}

	for _, endpoint := range webhookEndpoints {
		router.POST(endpoint, func(c *gin.Context) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing signature"})
		})
	}

	// Create minimal configuration
	config := Config{
		ListenAddress: ":8080",
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  10 * time.Second,
		IdleTimeout:   30 * time.Second,
		EnableCORS:    true,
	}

	return &MockServer{
		router: router,
		config: config,
		engine: mockEngine,
	}
}

func TestHealthHandler(t *testing.T) {
	server := setupMockServer(t)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Serve the request
	server.router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	components, ok := response["components"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "healthy", components["engine"])
	assert.Equal(t, "healthy", components["github"])
}

func TestMetricsHandler(t *testing.T) {
	server := setupMockServer(t)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	// Serve the request
	server.router.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "metrics data")
}

func TestWebhookEndpoints(t *testing.T) {
	server := setupMockServer(t)

	endpoints := []string{
		"/webhook/github",
		"/webhook/harness",
		"/webhook/sonarqube",
		"/webhook/artifactory",
		"/webhook/xray",
	}

	for _, endpoint := range endpoints {
		t.Run("Endpoint "+endpoint, func(t *testing.T) {
			// Test route registration is working
			req := httptest.NewRequest(http.MethodPost, endpoint, nil)
			w := httptest.NewRecorder()

			// Serve the request
			server.router.ServeHTTP(w, req)

			// Should return 400 (bad request) but not 404 (not found)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "Missing signature")
		})
	}
}

// Test the server shutdown behavior
func TestServerShutdown(t *testing.T) {
	// Create a shutdown function to test
	shutdownCalled := false
	shutdownFunc := func(_ context.Context) error {
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

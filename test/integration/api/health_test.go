//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/S-Corkum/devops-mcp/test/integration/testutils"
)

// TestHealthEndpoint verifies that the health endpoint returns a successful response
func TestHealthEndpoint(t *testing.T) {
	// Create HTTP client for API calls
	client := testutils.NewHTTPClient("http://localhost:8080", "")

	// Call the health endpoint
	resp, err := client.Get("/health")
	require.NoError(t, err, "Request to health endpoint should not fail")
	defer resp.Body.Close()

	// Verify response status
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return 200 OK")

	// Parse response
	var healthResponse struct {
		Status     string            `json:"status"`
		Components map[string]string `json:"components"`
	}

	err = testutils.ParseJSONResponse(resp, &healthResponse)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify health status
	assert.Equal(t, "healthy", healthResponse.Status, "Health status should be 'healthy'")
	assert.NotEmpty(t, healthResponse.Components, "Components should not be empty")
}

// TestMetricsEndpoint verifies that the metrics endpoint returns a successful response
func TestMetricsEndpoint(t *testing.T) {
	// Skip this test for now as it requires a different type of authentication
	t.Skip("Metrics endpoint requires a different authentication mechanism")

	// Create HTTP client for API calls
	client := testutils.NewHTTPClient("http://localhost:8080", "")

	// Call the metrics endpoint
	resp, err := client.Get("/metrics")
	require.NoError(t, err, "Request to metrics endpoint should not fail")
	defer resp.Body.Close()

	// Verify response status
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Metrics endpoint should return 200 OK")
}

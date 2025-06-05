package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHandlerTestCase defines a test case for mock API handlers
type MockHandlerTestCase struct {
	name           string
	method         string
	path           string
	requestBody    string
	requestHeaders map[string]string
	expectedStatus int
	expectedBody   map[string]interface{}
	expectedFields []string
}

// TestGitHubMockHandler tests the GitHub mock API handler with table-driven tests
func TestGitHubMockHandler(t *testing.T) {
	// Define the handler for GitHub mock
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Special handling for rate limit endpoint that's used for health checks
		if r.URL.Path == "/mock-github/rate_limit" {
			response := map[string]interface{}{
				"resources": map[string]interface{}{
					"core": map[string]interface{}{
						"limit":     5000,
						"used":      0,
						"remaining": 5000,
						"reset":     time.Now().Add(1 * time.Hour).Unix(),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
			return
		}

		// Special handling for health endpoint
		if r.URL.Path == "/mock-github/health" {
			response := map[string]interface{}{
				"status":    "ok",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			_ = json.NewEncoder(w).Encode(response)
			return
		}

		// Default response for other endpoints
		response := map[string]interface{}{
			"success":   true,
			"message":   "Mock GitHub response",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(response)
	}

	// Define test cases using table-driven approach
	testCases := []MockHandlerTestCase{
		{
			name:           "Rate Limit Endpoint",
			method:         http.MethodGet,
			path:           "/mock-github/rate_limit",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"resources"},
		},
		{
			name:           "Health Endpoint",
			method:         http.MethodGet,
			path:           "/mock-github/health",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"status", "timestamp"},
		},
		{
			name:           "Default Response",
			method:         http.MethodGet,
			path:           "/mock-github/repos/owner/repo",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"success", "message", "timestamp"},
		},
		{
			name:           "POST Request",
			method:         http.MethodPost,
			path:           "/mock-github/repos/owner/repo",
			requestBody:    `{"key": "value"}`,
			expectedStatus: http.StatusOK,
			expectedFields: []string{"success", "message", "timestamp"},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var reqBody io.Reader
			if tc.requestBody != "" {
				reqBody = strings.NewReader(tc.requestBody)
			}

			req, err := http.NewRequest(tc.method, tc.path, reqBody)
			require.NoError(t, err, "Failed to create request")

			// Add request headers if specified
			if tc.requestHeaders != nil {
				for key, value := range tc.requestHeaders {
					req.Header.Set(key, value)
				}
			}

			// Set content type for POST/PUT/PATCH requests with body
			if tc.requestBody != "" && req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rr := httptest.NewRecorder()

			// Act
			http.HandlerFunc(handler).ServeHTTP(rr, req)

			// Assert
			assert.Equal(t, tc.expectedStatus, rr.Code, "HTTP status code mismatch")

			// Verify response is valid JSON
			var response map[string]interface{}
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err, "Response is not valid JSON")

			// Check expected fields
			for _, field := range tc.expectedFields {
				assert.Contains(t, response, field, "Response missing expected field: "+field)
			}

			// Special assertions for specific endpoints
			if tc.path == "/mock-github/rate_limit" {
				resources, ok := response["resources"].(map[string]interface{})
				assert.True(t, ok, "Resources field should be an object")

				core, ok := resources["core"].(map[string]interface{})
				assert.True(t, ok, "Core field should be an object")

				assert.Equal(t, float64(5000), core["limit"], "Rate limit should be 5000")
				assert.Equal(t, float64(0), core["used"], "Used count should be 0")
				assert.Equal(t, float64(5000), core["remaining"], "Remaining should be 5000")
				assert.NotZero(t, core["reset"], "Reset timestamp should not be zero")
			}

			if tc.path == "/mock-github/health" {
				assert.Equal(t, "ok", response["status"], "Health status should be 'ok'")
				assert.NotEmpty(t, response["timestamp"], "Timestamp should not be empty")
			}

			if strings.HasPrefix(tc.path, "/mock-github/repos") {
				assert.Equal(t, true, response["success"], "Success field should be true")
				assert.Equal(t, "Mock GitHub response", response["message"], "Message field should match")
			}
		})
	}
}

// TestMockHandlers tests all mock API handlers with a shared test framework
func TestMockHandlers(t *testing.T) {
	// Define the mock handlers mapping
	mockHandlers := map[string]http.HandlerFunc{
		"harness": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Special handling for health endpoint
			if r.URL.Path == "/mock-harness/health" {
				response := map[string]interface{}{
					"status":    "ok",
					"timestamp": time.Now().Format(time.RFC3339),
				}
				_ = json.NewEncoder(w).Encode(response)
				return
			}

			// Handle pipelines endpoint
			if r.URL.Path == "/mock-harness/pipelines" {
				response := map[string]interface{}{
					"pipelines": []map[string]interface{}{
						{
							"id":   "pipeline1",
							"name": "Deploy to Production",
							"type": "deployment",
						},
						{
							"id":   "pipeline2",
							"name": "Run Integration Tests",
							"type": "build",
						},
					},
				}
				_ = json.NewEncoder(w).Encode(response)
				return
			}

			// Default response for other endpoints
			response := map[string]interface{}{
				"success":   true,
				"message":   "Mock Harness response",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			_ = json.NewEncoder(w).Encode(response)
		},
		"sonarqube": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Special handling for health endpoint
			if r.URL.Path == "/mock-sonarqube/health" {
				response := map[string]interface{}{
					"status":    "ok",
					"timestamp": time.Now().Format(time.RFC3339),
				}
				_ = json.NewEncoder(w).Encode(response)
				return
			}

			// Handle quality gate endpoint
			if strings.Contains(r.URL.Path, "/qualitygates/project_status") {
				response := map[string]interface{}{
					"projectStatus": map[string]interface{}{
						"status": "OK",
						"conditions": []map[string]interface{}{
							{
								"status":         "OK",
								"metricKey":      "bugs",
								"comparator":     "LT",
								"errorThreshold": "10",
								"actualValue":    "0",
							},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(response)
				return
			}

			// Default response for other endpoints
			response := map[string]interface{}{
				"success":   true,
				"message":   "Mock SonarQube response",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			_ = json.NewEncoder(w).Encode(response)
		},
		"artifactory": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Special handling for health endpoint
			if r.URL.Path == "/mock-artifactory/health" {
				response := map[string]interface{}{
					"status":    "ok",
					"timestamp": time.Now().Format(time.RFC3339),
				}
				_ = json.NewEncoder(w).Encode(response)
				return
			}

			// Default response for other endpoints
			response := map[string]interface{}{
				"success":   true,
				"message":   "Mock Artifactory response",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			_ = json.NewEncoder(w).Encode(response)
		},
		"xray": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Special handling for health endpoint
			if r.URL.Path == "/mock-xray/health" {
				response := map[string]interface{}{
					"status":    "ok",
					"timestamp": time.Now().Format(time.RFC3339),
				}
				_ = json.NewEncoder(w).Encode(response)
				return
			}

			// Default response for other endpoints
			response := map[string]interface{}{
				"success":   true,
				"message":   "Mock Xray response",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			_ = json.NewEncoder(w).Encode(response)
		},
	}

	// Define test cases for each handler
	handlerTestCases := map[string][]MockHandlerTestCase{
		"harness": {
			{
				name:           "Health Endpoint",
				method:         http.MethodGet,
				path:           "/mock-harness/health",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"status", "timestamp"},
			},
			{
				name:           "Pipelines Endpoint",
				method:         http.MethodGet,
				path:           "/mock-harness/pipelines",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"pipelines"},
			},
			{
				name:           "Default Response",
				method:         http.MethodGet,
				path:           "/mock-harness/deployments",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"success", "message", "timestamp"},
			},
		},
		"sonarqube": {
			{
				name:           "Health Endpoint",
				method:         http.MethodGet,
				path:           "/mock-sonarqube/health",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"status", "timestamp"},
			},
			{
				name:           "Quality Gate Endpoint",
				method:         http.MethodGet,
				path:           "/mock-sonarqube/qualitygates/project_status",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"projectStatus"},
			},
			{
				name:           "Default Response",
				method:         http.MethodGet,
				path:           "/mock-sonarqube/projects",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"success", "message", "timestamp"},
			},
		},
		"artifactory": {
			{
				name:           "Health Endpoint",
				method:         http.MethodGet,
				path:           "/mock-artifactory/health",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"status", "timestamp"},
			},
			{
				name:           "Default Response",
				method:         http.MethodGet,
				path:           "/mock-artifactory/repositories",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"success", "message", "timestamp"},
			},
		},
		"xray": {
			{
				name:           "Health Endpoint",
				method:         http.MethodGet,
				path:           "/mock-xray/health",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"status", "timestamp"},
			},
			{
				name:           "Default Response",
				method:         http.MethodGet,
				path:           "/mock-xray/scans",
				expectedStatus: http.StatusOK,
				expectedFields: []string{"success", "message", "timestamp"},
			},
		},
	}

	// Run tests for each handler
	for handlerName, testCases := range handlerTestCases {
		t.Run(handlerName+" Mock Handler", func(t *testing.T) {
			handler := mockHandlers[handlerName]

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// Arrange
					var reqBody io.Reader
					if tc.requestBody != "" {
						reqBody = strings.NewReader(tc.requestBody)
					}

					req, err := http.NewRequest(tc.method, tc.path, reqBody)
					require.NoError(t, err, "Failed to create request")

					// Add request headers if specified
					if tc.requestHeaders != nil {
						for key, value := range tc.requestHeaders {
							req.Header.Set(key, value)
						}
					}

					// Set content type for POST/PUT/PATCH requests with body
					if tc.requestBody != "" && req.Header.Get("Content-Type") == "" {
						req.Header.Set("Content-Type", "application/json")
					}

					rr := httptest.NewRecorder()

					// Act
					http.HandlerFunc(handler).ServeHTTP(rr, req)

					// Assert
					assert.Equal(t, tc.expectedStatus, rr.Code, "HTTP status code mismatch")

					// Verify response is valid JSON
					var response map[string]interface{}
					err = json.Unmarshal(rr.Body.Bytes(), &response)
					require.NoError(t, err, "Response is not valid JSON")

					// Check expected fields
					for _, field := range tc.expectedFields {
						assert.Contains(t, response, field,
							"Response missing expected field: "+field)
					}

					// Special assertions for health endpoints
					if strings.HasSuffix(tc.path, "/health") {
						assert.Equal(t, "ok", response["status"], "Health status should be 'ok'")
						assert.NotEmpty(t, response["timestamp"], "Timestamp should not be empty")
					}

					// Special assertions for SonarQube quality gate endpoint
					if tc.path == "/mock-sonarqube/qualitygates/project_status" {
						projectStatus, ok := response["projectStatus"].(map[string]interface{})
						assert.True(t, ok, "ProjectStatus field should be an object")
						assert.Equal(t, "OK", projectStatus["status"], "Quality gate status should be OK")

						conditions, ok := projectStatus["conditions"].([]interface{})
						assert.True(t, ok, "Conditions field should be an array")
						assert.NotEmpty(t, conditions, "Conditions array should not be empty")
					}

					// Special assertions for Harness pipelines endpoint
					if tc.path == "/mock-harness/pipelines" {
						pipelines, ok := response["pipelines"].([]interface{})
						assert.True(t, ok, "Pipelines field should be an array")
						assert.NotEmpty(t, pipelines, "Pipelines array should not be empty")
						assert.Equal(t, 2, len(pipelines), "Should return 2 pipeline examples")
					}
				})
			}
		})
	}
}

// TestWebhookEndpoints tests all webhook endpoint handlers
func TestWebhookEndpoints(t *testing.T) {
	// Create a common webhook handler
	webhookHandler := func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST requests
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"error":"Method not allowed"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}

	// Define webhook paths to test
	webhookPaths := []string{
		"/api/v1/webhook/github",
		"/api/v1/webhook/harness",
		"/api/v1/webhook/sonarqube",
		"/api/v1/webhook/artifactory",
		"/api/v1/webhook/xray",
	}

	// Set up test cases for each path
	for _, path := range webhookPaths {
		// Test successful POST request
		t.Run("POST "+path, func(t *testing.T) {
			// Arrange
			reqBody := `{"event":"push","repository":"test-repo"}`
			req, err := http.NewRequest(http.MethodPost, path, strings.NewReader(reqBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			// Act
			http.HandlerFunc(webhookHandler).ServeHTTP(rr, req)

			// Assert
			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, `{"status":"ok"}`, rr.Body.String())
		})

		// Test method not allowed (GET)
		t.Run("GET "+path+" (should fail)", func(t *testing.T) {
			// Arrange
			req, err := http.NewRequest(http.MethodGet, path, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()

			// Act
			http.HandlerFunc(webhookHandler).ServeHTTP(rr, req)

			// Assert
			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
			assert.Equal(t, `{"error":"Method not allowed"}`, rr.Body.String())
		})
	}
}

// TestHealthCheckHandler tests the health check endpoint
func TestHealthCheckHandler(t *testing.T) {
	// Define the handler function
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Only accept GET requests
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}

	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET request",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"healthy"}`,
		},
		{
			name:           "POST request (should fail)",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   ``,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req, err := http.NewRequest(tc.method, "/health", nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()

			// Act
			http.HandlerFunc(handler).ServeHTTP(rr, req)

			// Assert
			assert.Equal(t, tc.expectedStatus, rr.Code)
			assert.Equal(t, tc.expectedBody, strings.TrimSpace(rr.Body.String()))
		})
	}
}

// TestRequestBodyProcessing tests proper handling of JSON request bodies
func TestRequestBodyProcessing(t *testing.T) {
	// Define a handler that echo's back the request body
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Read the request body
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Failed to read request body"}`))
			return
		}

		// Try to parse as JSON
		var requestData map[string]interface{}
		if err := json.Unmarshal(body, &requestData); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Invalid JSON"}`))
			return
		}

		// Add response wrapper
		response := map[string]interface{}{
			"success": true,
			"data":    requestData,
		}

		// Send response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}

	testCases := []struct {
		name           string
		requestBody    string
		expectedStatus int
		responseCheck  func(t *testing.T, response map[string]interface{})
	}{
		{
			name:           "Valid JSON object",
			requestBody:    `{"name":"test","value":123}`,
			expectedStatus: http.StatusOK,
			responseCheck: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				data := response["data"].(map[string]interface{})
				assert.Equal(t, "test", data["name"])
				assert.Equal(t, float64(123), data["value"])
			},
		},
		{
			name:           "Valid JSON array",
			requestBody:    `{"items":[1,2,3]}`,
			expectedStatus: http.StatusOK,
			responseCheck: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				data := response["data"].(map[string]interface{})
				items := data["items"].([]interface{})
				assert.Equal(t, 3, len(items))
				assert.Equal(t, float64(1), items[0])
				assert.Equal(t, float64(2), items[1])
				assert.Equal(t, float64(3), items[2])
			},
		},
		{
			name:           "Invalid JSON",
			requestBody:    `{"name":"test",}`, // Invalid JSON with trailing comma
			expectedStatus: http.StatusBadRequest,
			responseCheck: func(t *testing.T, response map[string]interface{}) {
				assert.Equal(t, "Invalid JSON", response["error"])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req, err := http.NewRequest(http.MethodPost, "/api/test",
				bytes.NewBufferString(tc.requestBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()

			// Act
			http.HandlerFunc(handler).ServeHTTP(rr, req)

			// Assert
			assert.Equal(t, tc.expectedStatus, rr.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err, "Response should be valid JSON")

			// Perform custom checks
			tc.responseCheck(t, response)
		})
	}
}

// TestContentTypeHandling tests proper handling of content types
func TestContentTypeHandling(t *testing.T) {
	// Define a handler that checks content types
	handler := func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")

		response := map[string]interface{}{
			"success":       true,
			"contentType":   contentType,
			"requestPath":   r.URL.Path,
			"requestMethod": r.Method,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}

	testCases := []struct {
		name         string
		method       string
		path         string
		contentType  string
		expectedBody string
	}{
		{
			name:        "JSON Content Type",
			method:      http.MethodPost,
			path:        "/api/data",
			contentType: "application/json",
		},
		{
			name:        "Form Content Type",
			method:      http.MethodPost,
			path:        "/api/form",
			contentType: "application/x-www-form-urlencoded",
		},
		{
			name:        "No Content Type",
			method:      http.MethodGet,
			path:        "/api/query",
			contentType: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			req, err := http.NewRequest(tc.method, tc.path, nil)
			require.NoError(t, err)

			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			rr := httptest.NewRecorder()

			// Act
			http.HandlerFunc(handler).ServeHTTP(rr, req)

			// Assert
			assert.Equal(t, http.StatusOK, rr.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err, "Response should be valid JSON")

			// Check content type in response
			assert.Equal(t, tc.contentType, response["contentType"])
			assert.Equal(t, tc.path, response["requestPath"])
			assert.Equal(t, tc.method, response["requestMethod"])
		})
	}
}

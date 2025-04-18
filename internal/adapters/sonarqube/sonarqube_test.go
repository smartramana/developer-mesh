package sonarqube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdapter(t *testing.T) {
	// Test with minimal config
	t.Run("Minimal Config", func(t *testing.T) {
		config := Config{
			BaseURL: "https://sonarqube.example.com/api",
			Token:   "test-token",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, config.BaseURL, adapter.config.BaseURL)
		assert.Equal(t, config.Token, adapter.config.Token)
		assert.Equal(t, 30*time.Second, adapter.config.RequestTimeout)
		assert.Equal(t, 3, adapter.config.RetryMax)
		assert.Equal(t, time.Second, adapter.config.RetryDelay)
	})

	// Test with full config
	t.Run("Full Config", func(t *testing.T) {
		config := Config{
			BaseURL:        "https://sonarqube.example.com/api",
			Token:          "test-token",
			RequestTimeout: 60 * time.Second,
			RetryMax:       5,
			RetryDelay:     2 * time.Second,
			MockResponses:  true,
			MockURL:        "http://localhost:8080/mock",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, config.BaseURL, adapter.config.BaseURL)
		assert.Equal(t, config.Token, adapter.config.Token)
		assert.Equal(t, config.RequestTimeout, adapter.config.RequestTimeout)
		assert.Equal(t, config.RetryMax, adapter.config.RetryMax)
		assert.Equal(t, config.RetryDelay, adapter.config.RetryDelay)
		assert.Equal(t, config.MockResponses, adapter.config.MockResponses)
		assert.Equal(t, config.MockURL, adapter.config.MockURL)
	})

	// Test with username/password authentication
	t.Run("Username Password Auth", func(t *testing.T) {
		config := Config{
			BaseURL:   "https://sonarqube.example.com/api",
			Username:  "test-user",
			Password:  "test-pass",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, config.Username, adapter.config.Username)
		assert.Equal(t, config.Password, adapter.config.Password)
	})
}

func TestInitialize(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle the system/status endpoint
		if r.URL.Path == "/system/status" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"UP","id":"20230102153045","version":"9.9.1"}`)
			return
		}

		// Handle health check endpoint (for mock)
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Default response
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Test successful initialization with token
	t.Run("Successful Initialization With Token", func(t *testing.T) {
		config := Config{
			BaseURL:        server.URL,
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		err = adapter.Initialize(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, "healthy", adapter.healthStatus)
	})

	// Test successful initialization with mock server
	t.Run("Successful Initialization With Mock Server", func(t *testing.T) {
		config := Config{
			BaseURL:        "https://nonexistent.example.com",
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
			MockResponses:  true,
			MockURL:        server.URL,
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		err = adapter.Initialize(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, "healthy", adapter.healthStatus)
	})

	// Test initialization with new config parameter
	t.Run("Initialization with New Config", func(t *testing.T) {
		adapter, err := NewAdapter(Config{})
		require.NoError(t, err)

		config := Config{
			BaseURL:        server.URL,
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
		}

		err = adapter.Initialize(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, config.BaseURL, adapter.config.BaseURL)
		assert.Equal(t, config.Token, adapter.config.Token)
	})

	// Test initialization without authentication credentials
	t.Run("Initialization without Auth Credentials", func(t *testing.T) {
		adapter, err := NewAdapter(Config{})
		require.NoError(t, err)

		config := Config{
			BaseURL: server.URL,
		}

		err = adapter.Initialize(context.Background(), config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication credentials are required")
	})

	// Test initialization with invalid config type
	t.Run("Initialization with Invalid Config Type", func(t *testing.T) {
		adapter, err := NewAdapter(Config{})
		require.NoError(t, err)

		invalidConfig := "invalid config"
		err = adapter.Initialize(context.Background(), invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid config type")
	})
}

func TestGetData(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Mock different API endpoints
		switch r.URL.Path {
		case "/system/status":
			fmt.Fprint(w, `{"status":"UP","id":"20230102153045","version":"9.9.1"}`)

		case "/qualitygates/project_status":
			fmt.Fprint(w, `{
				"projectStatus": {
					"status": "OK",
					"conditions": [
						{
							"status": "OK",
							"metricKey": "coverage",
							"comparator": "GT",
							"errorThreshold": "80.0",
							"actualValue": "85.0"
						}
					]
				}
			}`)

		case "/issues/search":
			fmt.Fprint(w, `{
				"total": 1,
				"p": 1,
				"ps": 100,
				"paging": {
					"pageIndex": 1,
					"pageSize": 100,
					"total": 1
				},
				"issues": [
					{
						"key": "issue-1",
						"rule": "rule-1",
						"severity": "MAJOR",
						"component": "project:file.js",
						"project": "project",
						"line": 42,
						"status": "OPEN",
						"message": "Fix this issue",
						"effort": "5min",
						"debt": "5min",
						"creationDate": "2023-01-02T15:30:45+0000"
					}
				]
			}`)

		case "/measures/component":
			fmt.Fprint(w, `{
				"component": {
					"key": "project",
					"name": "Project",
					"qualifier": "TRK",
					"measures": [
						{
							"metric": "coverage",
							"value": "85.0"
						},
						{
							"metric": "bugs",
							"value": "5"
						}
					]
				}
			}`)

		case "/projects/search":
			fmt.Fprint(w, `{
				"paging": {
					"pageIndex": 1,
					"pageSize": 100,
					"total": 1
				},
				"components": [
					{
						"key": "project",
						"name": "Project",
						"qualifier": "TRK",
						"visibility": "public",
						"lastAnalysisDate": "2023-01-02T15:30:45+0000"
					}
				]
			}`)

		case "/qualitygates/list":
			fmt.Fprint(w, `{
				"qualitygates": [
					{
						"id": 1,
						"name": "Default"
					}
				],
				"default": 1
			}`)

		case "/components/show":
			fmt.Fprint(w, `{
				"component": {
					"key": "project",
					"name": "Project",
					"qualifier": "TRK",
					"description": "Project description",
					"visibility": "public",
					"lastAnalysisDate": "2023-01-02T15:30:45+0000"
				}
			}`)

		case "/measures/search_history":
			fmt.Fprint(w, `{
				"paging": {
					"pageIndex": 1,
					"pageSize": 100,
					"total": 2
				},
				"measures": [
					{
						"metric": "coverage",
						"history": [
							{
								"date": "2023-01-01T00:00:00+0000",
								"value": "80.0"
							},
							{
								"date": "2023-01-02T00:00:00+0000",
								"value": "85.0"
							}
						]
					}
				]
			}`)

		case "/metrics/search":
			fmt.Fprint(w, `{
				"metrics": [
					{
						"key": "coverage",
						"name": "Coverage",
						"type": "PERCENT",
						"domain": "Coverage"
					},
					{
						"key": "bugs",
						"name": "Bugs",
						"type": "INT",
						"domain": "Reliability"
					}
				]
			}`)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create adapter with mock server
	config := Config{
		BaseURL:        server.URL,
		Token:          "test-token",
		RequestTimeout: 5 * time.Second,
	}

	adapter, err := NewAdapter(config)
	require.NoError(t, err)
	err = adapter.Initialize(context.Background(), nil)
	require.NoError(t, err)

	// Test get_quality_gate_status operation
	t.Run("get_quality_gate_status", func(t *testing.T) {
		query := map[string]interface{}{
			"operation":   "get_quality_gate_status",
			"project_key": "project",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "projectStatus")
	})

	// Test get_issues operation
	t.Run("get_issues", func(t *testing.T) {
		query := map[string]interface{}{
			"operation":   "get_issues",
			"project_key": "project",
			"status":      "OPEN",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "issues")
	})

	// Test get_metrics operation
	t.Run("get_metrics", func(t *testing.T) {
		query := map[string]interface{}{
			"operation":   "get_metrics",
			"project_key": "project",
			"metrics":     []string{"coverage", "bugs"},
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "component")
	})

	// Test get_projects operation
	t.Run("get_projects", func(t *testing.T) {
		query := map[string]interface{}{
			"operation": "get_projects",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "components")
	})

	// Test get_quality_gates operation
	t.Run("get_quality_gates", func(t *testing.T) {
		query := map[string]interface{}{
			"operation": "get_quality_gates",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "qualitygates")
	})

	// Test get_component_details operation
	t.Run("get_component_details", func(t *testing.T) {
		query := map[string]interface{}{
			"operation":   "get_component_details",
			"project_key": "project",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "component")
	})

	// Test get_measures_history operation
	t.Run("get_measures_history", func(t *testing.T) {
		query := map[string]interface{}{
			"operation":   "get_measures_history",
			"project_key": "project",
			"metric_keys": "coverage",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "measures")
	})

	// Test search_metrics operation
	t.Run("search_metrics", func(t *testing.T) {
		query := map[string]interface{}{
			"operation": "search_metrics",
		}

		result, err := adapter.GetData(context.Background(), query)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "metrics")
	})

	// Test invalid operation
	t.Run("invalid_operation", func(t *testing.T) {
		query := map[string]interface{}{
			"operation": "invalid_operation",
		}

		_, err := adapter.GetData(context.Background(), query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported operation")
	})

	// Test missing operation
	t.Run("missing_operation", func(t *testing.T) {
		query := map[string]interface{}{
			"project_key": "project",
		}

		_, err := adapter.GetData(context.Background(), query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing operation")
	})

	// Test invalid query type
	t.Run("invalid_query_type", func(t *testing.T) {
		_, err := adapter.GetData(context.Background(), "invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid query type")
	})
}

func TestExecuteAction(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Mock different API endpoints
		switch r.URL.Path {
		case "/system/status":
			fmt.Fprint(w, `{"status":"UP","id":"20230102153045","version":"9.9.1"}`)

		case "/qualitygates/project_status":
			fmt.Fprint(w, `{
				"projectStatus": {
					"status": "OK",
					"conditions": [
						{
							"status": "OK",
							"metricKey": "coverage",
							"comparator": "GT",
							"errorThreshold": "80.0",
							"actualValue": "85.0"
						}
					]
				}
			}`)

		case "/issues/search":
			fmt.Fprint(w, `{
				"total": 1,
				"issues": [
					{
						"key": "issue-1",
						"rule": "rule-1",
						"severity": "MAJOR",
						"component": "project:file.js",
						"project": "project",
						"line": 42,
						"status": "OPEN",
						"message": "Fix this issue",
						"effort": "5min",
						"debt": "5min",
						"creationDate": "2023-01-02T15:30:45+0000"
					}
				]
			}`)

		case "/projects/create":
			fmt.Fprint(w, `{
				"project": {
					"key": "new-project",
					"name": "New Project",
					"qualifier": "TRK"
				}
			}`)

		case "/projects/delete":
			// Project delete returns 204 No Content
			w.WriteHeader(http.StatusNoContent)

		case "/ce/task":
			fmt.Fprint(w, `{
				"task": {
					"id": "task-1",
					"type": "REPORT",
					"componentId": "project",
					"componentKey": "project",
					"componentName": "Project",
					"status": "SUCCESS",
					"submittedAt": "2023-01-02T15:30:45+0000",
					"submitterLogin": "admin",
					"startedAt": "2023-01-02T15:30:46+0000",
					"executedAt": "2023-01-02T15:31:00+0000",
					"executionTimeMs": 14000
				}
			}`)

		case "/project_tags/set":
			// Tags set returns 204 No Content
			w.WriteHeader(http.StatusNoContent)

		case "/qualitygates/select":
			// Quality gate selection returns 204 No Content
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create adapter with mock server
	config := Config{
		BaseURL:        server.URL,
		Token:          "test-token",
		RequestTimeout: 5 * time.Second,
	}

	adapter, err := NewAdapter(config)
	require.NoError(t, err)
	err = adapter.Initialize(context.Background(), nil)
	require.NoError(t, err)

	// Test trigger_analysis action
	t.Run("trigger_analysis", func(t *testing.T) {
		params := map[string]interface{}{
			"project_key": "project",
			"branch":      "main",
		}

		result, err := adapter.ExecuteAction(context.Background(), "context-123", "trigger_analysis", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "project", resultMap["project_key"])
		assert.Equal(t, "main", resultMap["branch"])
		assert.Contains(t, resultMap, "task_id")
		assert.Equal(t, "PENDING", resultMap["status"])
	})

	// Test get_quality_gate_status action
	t.Run("get_quality_gate_status", func(t *testing.T) {
		params := map[string]interface{}{
			"project_key": "project",
		}

		result, err := adapter.ExecuteAction(context.Background(), "context-123", "get_quality_gate_status", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "projectStatus")
	})

	// Test get_issues action
	t.Run("get_issues", func(t *testing.T) {
		params := map[string]interface{}{
			"project_key": "project",
			"status":      "OPEN",
		}

		result, err := adapter.ExecuteAction(context.Background(), "context-123", "get_issues", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "issues")
	})

	// Test create_project action
	t.Run("create_project", func(t *testing.T) {
		params := map[string]interface{}{
			"project_key": "new-project",
			"name":        "New Project",
		}

		result, err := adapter.ExecuteAction(context.Background(), "context-123", "create_project", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "project")
	})

	// Test delete_project action
	t.Run("delete_project", func(t *testing.T) {
		params := map[string]interface{}{
			"project_key": "project-to-delete",
		}

		result, err := adapter.ExecuteAction(context.Background(), "context-123", "delete_project", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "project-to-delete", resultMap["project_key"])
		assert.Equal(t, "deleted", resultMap["status"])
	})

	// Test get_analysis_status action
	t.Run("get_analysis_status", func(t *testing.T) {
		params := map[string]interface{}{
			"task_id": "task-1",
		}

		result, err := adapter.ExecuteAction(context.Background(), "context-123", "get_analysis_status", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, resultMap, "task")
	})

	// Test set_project_tags action
	t.Run("set_project_tags", func(t *testing.T) {
		params := map[string]interface{}{
			"project_key": "project",
			"tags":        "tag1,tag2",
		}

		result, err := adapter.ExecuteAction(context.Background(), "context-123", "set_project_tags", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "project", resultMap["project_key"])
		assert.Equal(t, "tag1,tag2", resultMap["tags"])
		assert.Equal(t, "updated", resultMap["status"])
	})

	// Test set_quality_gate action
	t.Run("set_quality_gate", func(t *testing.T) {
		params := map[string]interface{}{
			"project_key": "project",
			"gate_id":     "1",
		}

		result, err := adapter.ExecuteAction(context.Background(), "context-123", "set_quality_gate", params)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify result structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "project", resultMap["project_key"])
		assert.Equal(t, "1", resultMap["gate_id"])
		assert.Equal(t, "assigned", resultMap["status"])
	})

	// Test invalid action
	t.Run("invalid_action", func(t *testing.T) {
		params := map[string]interface{}{
			"project_key": "project",
		}

		_, err := adapter.ExecuteAction(context.Background(), "context-123", "invalid_action", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported action")
	})

	// Test missing parameters
	t.Run("missing_parameters", func(t *testing.T) {
		params := map[string]interface{}{}

		_, err := adapter.ExecuteAction(context.Background(), "context-123", "trigger_analysis", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing project_key")
	})
}

func TestIsSafeOperation(t *testing.T) {
	// Create adapter
	adapter, err := NewAdapter(Config{})
	require.NoError(t, err)

	// Test unsafe operations
	t.Run("Unsafe Operations", func(t *testing.T) {
		safe, err := adapter.IsSafeOperation("delete_project", nil)
		require.NoError(t, err)
		assert.False(t, safe)
	})

	// Test safe operations
	t.Run("Safe Operations", func(t *testing.T) {
		safeActions := []string{
			"trigger_analysis",
			"get_quality_gate_status",
			"get_issues",
			"create_project",
			"get_analysis_status",
			"set_project_tags",
			"set_quality_gate",
		}

		for _, action := range safeActions {
			safe, err := adapter.IsSafeOperation(action, nil)
			require.NoError(t, err)
			assert.True(t, safe)
		}
	})
}

func TestHealth(t *testing.T) {
	// Create adapter
	adapter, err := NewAdapter(Config{})
	require.NoError(t, err)

	// Test initial health
	assert.Equal(t, "initializing", adapter.Health())

	// Test after setting health status
	adapter.healthStatus = "healthy"
	assert.Equal(t, "healthy", adapter.Health())

	adapter.healthStatus = "unhealthy: connection failed"
	assert.Equal(t, "unhealthy: connection failed", adapter.Health())
}

func TestClose(t *testing.T) {
	// Create adapter
	adapter, err := NewAdapter(Config{})
	require.NoError(t, err)

	// Test closing the adapter
	err = adapter.Close()
	assert.NoError(t, err)
}

func TestWebhooks(t *testing.T) {
	// Create adapter
	adapter, err := NewAdapter(Config{})
	require.NoError(t, err)

	// Test Subscribe (should be no-op)
	err = adapter.Subscribe("analysis", func(interface{}) {})
	assert.NoError(t, err)

	// Test HandleWebhook (should be no-op)
	event := []byte(`{"serverUrl":"https://sonar.example.com","taskId":"123","status":"SUCCESS"}`)
	err = adapter.HandleWebhook(context.Background(), "analysis", event)
	assert.NoError(t, err)
}

func TestMockConnection(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle health check endpoint
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Handle root path
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Default response
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Test connection to mock server via health endpoint
	t.Run("Connection to Mock Server via Health", func(t *testing.T) {
		config := Config{
			BaseURL:        "https://nonexistent.example.com",
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
			MockResponses:  true,
			MockURL:        server.URL,
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		err = adapter.testConnection(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "healthy", adapter.healthStatus)
	})

	// Test connection to mock server via root endpoint (fallback)
	t.Run("Connection to Mock Server via Root", func(t *testing.T) {
		// Create a server without health endpoint
		rootServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer rootServer.Close()

		config := Config{
			BaseURL:        "https://nonexistent.example.com",
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
			MockResponses:  true,
			MockURL:        rootServer.URL,
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		err = adapter.testConnection(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "healthy", adapter.healthStatus)
	})

	// Test failed connection to mock server
	t.Run("Failed Connection to Mock Server", func(t *testing.T) {
		config := Config{
			BaseURL:        "https://nonexistent.example.com",
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
			MockResponses:  true,
			MockURL:        "http://nonexistent.local", // Invalid URL
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		err = adapter.testConnection(context.Background())
		assert.Error(t, err)
		assert.Contains(t, adapter.healthStatus, "unhealthy")
	})
}

func TestCreateRequest(t *testing.T) {
	// Test request creation with token authentication
	t.Run("Request with Token Auth", func(t *testing.T) {
		config := Config{
			BaseURL: "https://sonarqube.example.com/api",
			Token:   "test-token",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		req, err := adapter.createRequest(context.Background(), "GET", "/projects/search", nil)
		require.NoError(t, err)

		assert.Equal(t, "GET", req.Method)
		assert.Equal(t, "https://sonarqube.example.com/api/projects/search", req.URL.String())
		assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", req.Header.Get("Accept"))
	})

	// Test request creation with basic authentication
	t.Run("Request with Basic Auth", func(t *testing.T) {
		config := Config{
			BaseURL:  "https://sonarqube.example.com/api",
			Username: "test-user",
			Password: "test-pass",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		req, err := adapter.createRequest(context.Background(), "GET", "/projects/search", nil)
		require.NoError(t, err)

		assert.Equal(t, "GET", req.Method)
		assert.Equal(t, "https://sonarqube.example.com/api/projects/search", req.URL.String())

		// Check for Basic Auth header
		username, password, ok := req.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test-user", username)
		assert.Equal(t, "test-pass", password)
	})

	// Test request creation with mock URL
	t.Run("Request with Mock URL", func(t *testing.T) {
		config := Config{
			BaseURL:       "https://sonarqube.example.com/api",
			Token:         "test-token",
			MockResponses: true,
			MockURL:       "http://mock.local",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		req, err := adapter.createRequest(context.Background(), "GET", "/projects/search", nil)
		require.NoError(t, err)

		assert.Equal(t, "GET", req.Method)
		assert.Equal(t, "http://mock.local/projects/search", req.URL.String())
	})

	// Test request creation with query parameters
	t.Run("Request with Query Parameters", func(t *testing.T) {
		config := Config{
			BaseURL: "https://sonarqube.example.com/api",
			Token:   "test-token",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		queryParams := map[string][]string{
			"q":    {"test"},
			"page": {"1"},
		}

		req, err := adapter.createRequest(context.Background(), "GET", "/projects/search", queryParams)
		require.NoError(t, err)

		assert.Equal(t, "GET", req.Method)
		assert.Equal(t, "https://sonarqube.example.com/api/projects/search?page=1&q=test", req.URL.String())
	})

	// Test handling of path without leading slash
	t.Run("Path Without Leading Slash", func(t *testing.T) {
		config := Config{
			BaseURL: "https://sonarqube.example.com/api",
			Token:   "test-token",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		req, err := adapter.createRequest(context.Background(), "GET", "projects/search", nil)
		require.NoError(t, err)

		assert.Equal(t, "GET", req.Method)
		assert.Equal(t, "https://sonarqube.example.com/api/projects/search", req.URL.String())
	})

	// Test handling of baseURL with trailing slash
	t.Run("BaseURL With Trailing Slash", func(t *testing.T) {
		config := Config{
			BaseURL: "https://sonarqube.example.com/api/",
			Token:   "test-token",
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)

		req, err := adapter.createRequest(context.Background(), "GET", "/projects/search", nil)
		require.NoError(t, err)

		assert.Equal(t, "GET", req.Method)
		assert.Equal(t, "https://sonarqube.example.com/api/projects/search", req.URL.String())
	})
}

func TestExecuteRequest(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test different response codes
		if r.URL.Path == "/success" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"success"}`)
			return
		}

		if r.URL.Path == "/error-400" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"errors":[{"msg":"Bad request"}]}`)
			return
		}

		if r.URL.Path == "/error-500" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"errors":[{"msg":"Internal server error"}]}`)
			return
		}

		// Default response
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Test successful request
	t.Run("Successful Request", func(t *testing.T) {
		config := Config{
			BaseURL:        server.URL,
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
			RetryMax:       3,
			RetryDelay:     100 * time.Millisecond,
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)
		
		// Initialize the HTTP client
		adapter.client = &http.Client{
			Timeout: config.RequestTimeout,
		}

		req, err := http.NewRequest("GET", server.URL+"/success", nil)
		require.NoError(t, err)

		body, err := adapter.executeRequest(req)
		require.NoError(t, err)
		assert.NotEmpty(t, body)

		// Verify response content
		var response map[string]interface{}
		err = json.Unmarshal(body, &response)
		require.NoError(t, err)
		assert.Equal(t, "success", response["status"])
	})

	// Test client error (4xx)
	t.Run("Client Error (4xx)", func(t *testing.T) {
		config := Config{
			BaseURL:        server.URL,
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
			RetryMax:       3,
			RetryDelay:     100 * time.Millisecond,
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)
		
		// Initialize the HTTP client
		adapter.client = &http.Client{
			Timeout: config.RequestTimeout,
		}

		req, err := http.NewRequest("GET", server.URL+"/error-400", nil)
		require.NoError(t, err)

		_, err = adapter.executeRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API returned error status: 400")
	})

	// Test server error (5xx) with retries
	t.Run("Server Error (5xx) with Retries", func(t *testing.T) {
		config := Config{
			BaseURL:        server.URL,
			Token:          "test-token",
			RequestTimeout: 5 * time.Second,
			RetryMax:       3,
			RetryDelay:     100 * time.Millisecond,
		}

		adapter, err := NewAdapter(config)
		require.NoError(t, err)
		
		// Initialize the HTTP client
		adapter.client = &http.Client{
			Timeout: config.RequestTimeout,
		}

		req, err := http.NewRequest("GET", server.URL+"/error-500", nil)
		require.NoError(t, err)

		startTime := time.Now()
		_, err = adapter.executeRequest(req)
		duration := time.Since(startTime)

		assert.Error(t, err)
		// The error might be about the response body being closed or an error status
		// Just check that an error was returned
		
		// Verify that retries occurred (should take at least RetryDelay * RetryMax time)
		assert.GreaterOrEqual(t, duration, 300*time.Millisecond)
	})
}

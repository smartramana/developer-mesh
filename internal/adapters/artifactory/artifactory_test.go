package artifactory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAdapter(t *testing.T) {
	// Test with minimal config
	cfg := Config{
		BaseURL:        "https://artifactory.example.com",
		RequestTimeout: 30 * time.Second,
		RetryMax:       3,
		RetryDelay:     1 * time.Second,
	}

	adapter, err := NewAdapter(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, cfg.BaseURL, adapter.config.BaseURL)
	assert.Equal(t, cfg.RequestTimeout, adapter.config.RequestTimeout)
	assert.Equal(t, cfg.RetryMax, adapter.BaseAdapter.RetryMax)
	assert.Equal(t, cfg.RetryDelay, adapter.BaseAdapter.RetryDelay)
}

func TestSetAuthHeaders(t *testing.T) {
	testCases := []struct {
		name     string
		config   Config
		expected string
		header   string
	}{
		{
			name: "Bearer Token Prefixed Auth",
			config: Config{
				Token: "Bearer abc123",
			},
			expected: "Bearer abc123",
			header:   "Authorization",
		},
		{
			name: "JWT Token Auth",
			config: Config{
				Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			},
			expected: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			header:   "Authorization",
		},
		{
			name: "API Key Auth",
			config: Config{
				Token: "apikey123",
			},
			expected: "apikey123",
			header:   "X-JFrog-Art-Api",
		},
		{
			name: "Basic Auth",
			config: Config{
				Username: "user",
				Password: "pass",
			},
			header: "Authorization",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := NewAdapter(tc.config)
			req, _ := http.NewRequest("GET", "https://example.com", nil)
			adapter.setAuthHeaders(req)

			// For Basic Auth, just verify it's set
			if tc.config.Username != "" {
				assert.NotEmpty(t, req.Header.Get(tc.header))
			} else {
				assert.Equal(t, tc.expected, req.Header.Get(tc.header))
			}
		})
	}
}

func TestIsSafeOperation(t *testing.T) {
	adapter, _ := NewAdapter(Config{})

	// Test cases for safe operations
	safeOps := []string{
		"download_artifact",
		"get_artifact_info",
		"search_artifacts",
		"get_repository_info",
		"get_folder_content",
		"calculate_checksum",
	}

	for _, op := range safeOps {
		t.Run(op, func(t *testing.T) {
			safe, err := adapter.IsSafeOperation(op, nil)
			assert.True(t, safe)
			assert.NoError(t, err)
		})
	}

	// Test cases for unsafe operations
	unsafeOps := []string{
		"upload_artifact",
		"delete_artifact",
		"move_artifact",
		"delete_repository",
	}

	for _, op := range unsafeOps {
		t.Run(op, func(t *testing.T) {
			safe, err := adapter.IsSafeOperation(op, nil)
			assert.False(t, safe)
			assert.Error(t, err)
		})
	}
}

func TestGetData(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Check the request path and return appropriate mock response
		switch {
		case r.URL.Path == "/system/ping":
			w.WriteHeader(http.StatusOK)
			
		case r.URL.Path == "/storage/maven-local/org/example/app/1.0.0/app-1.0.0.jar":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"repo": "maven-local",
				"path": "/org/example/app/1.0.0",
				"name": "app-1.0.0.jar",
				"size": 1024,
				"created": "2023-01-01T00:00:00Z",
			})
			
		case r.URL.Path == "/search/aql":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"repo": "maven-local",
						"path": "org/example",
						"name": "example-lib-1.0.0.jar",
					},
				},
				"range": map[string]interface{}{
					"start_pos": 0,
					"end_pos": 1,
					"total": 1,
				},
			})
			
		case r.URL.Path == "/build/my-app/42":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"buildInfo": map[string]interface{}{
					"name": "my-app",
					"number": "42",
				},
			})
			
		case r.URL.Path == "/repositories":
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"key": "maven-local",
					"type": "local",
					"packageType": "maven",
				},
			})
			
		case r.URL.Path == "/storageinfo":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"binariesSummary": map[string]interface{}{
					"binariesCount": 100,
					"binariesSize": 1048576,
				},
			})
			
		case r.URL.Path == "/system":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"serverVersion": "7.0.0",
				"serverTime": "2023-01-01T00:00:00Z",
			})
			
		case r.URL.Path == "/system/version":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"version": "7.0.0",
				"revision": "12345",
			})
			
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create adapter with test server URL
	adapter, _ := NewAdapter(Config{
		BaseURL:        server.URL,
		RequestTimeout: 1 * time.Second,
	})
	
	// Initialize the adapter
	err := adapter.Initialize(context.Background(), nil)
	assert.NoError(t, err)

	testCases := []struct {
		name        string
		operation   string
		params      map[string]interface{}
		expectError bool
	}{
		{
			name: "Get Artifact Info",
			operation: "get_artifact_info",
			params: map[string]interface{}{
				"path": "maven-local/org/example/app/1.0.0/app-1.0.0.jar",
			},
			expectError: false,
		},
		{
			name: "Search Artifacts",
			operation: "search_artifacts",
			params: map[string]interface{}{
				"query": "example",
				"repo": "maven-local",
			},
			expectError: false,
		},
		{
			name: "Get Build Info",
			operation: "get_build_info",
			params: map[string]interface{}{
				"build_name": "my-app",
				"build_number": "42",
			},
			expectError: false,
		},
		{
			name: "Get Repositories",
			operation: "get_repositories",
			params: map[string]interface{}{
				"type": "local",
			},
			expectError: false,
		},
		{
			name: "Get Storage Info",
			operation: "get_storage_info",
			params: map[string]interface{}{},
			expectError: false,
		},
		{
			name: "Get System Info",
			operation: "get_system_info",
			params: map[string]interface{}{},
			expectError: false,
		},
		{
			name: "Get Version",
			operation: "get_version",
			params: map[string]interface{}{},
			expectError: false,
		},
		{
			name: "Invalid Operation",
			operation: "invalid_operation",
			params: map[string]interface{}{},
			expectError: true,
		},
		{
			name: "Missing Parameters",
			operation: "get_artifact_info",
			params: map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queryParams := map[string]interface{}{
				"operation": tc.operation,
			}
			
			// Add all params from the test case
			for k, v := range tc.params {
				queryParams[k] = v
			}
			
			result, err := adapter.GetData(context.Background(), queryParams)
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestExecuteAction(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Check the request path and return appropriate mock response
		switch {
		case r.URL.Path == "/system/ping":
			w.WriteHeader(http.StatusOK)
			
		case r.URL.Path == "/storage/maven-local/org/example/app/1.0.0/app-1.0.0.jar":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"repo": "maven-local",
				"path": "/org/example/app/1.0.0",
				"name": "app-1.0.0.jar",
				"size": 1024,
				"created": "2023-01-01T00:00:00Z",
			})
			
		case r.URL.Path == "/repositories/maven-local":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"key": "maven-local",
				"type": "local",
				"packageType": "maven",
			})
			
		case r.URL.String() == "/storage/maven-local?list=true&deep=1&listFolders=1":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"repo": "maven-local",
				"path": "/",
				"children": []map[string]interface{}{
					{
						"uri": "/org",
						"folder": true,
					},
				},
			})
			
		case r.URL.String() == "/storage/maven-local/org/example/app/1.0.0/app-1.0.0.jar?checksum":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"checksums": map[string]interface{}{
					"md5": "abcdef1234567890",
					"sha1": "1234567890abcdef",
					"sha256": "1234567890abcdef1234567890abcdef",
				},
			})
			
		case r.Method == "HEAD":
			// For download artifact HEAD request
			w.Header().Set("Content-Length", "1024")
			w.Header().Set("Content-Type", "application/java-archive")
			w.WriteHeader(http.StatusOK)
			
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create adapter with test server URL
	adapter, _ := NewAdapter(Config{
		BaseURL:        server.URL,
		RequestTimeout: 1 * time.Second,
	})
	
	// Initialize the adapter
	err := adapter.Initialize(context.Background(), nil)
	assert.NoError(t, err)

	testCases := []struct {
		name        string
		action      string
		params      map[string]interface{}
		expectError bool
	}{
		{
			name: "Get Artifact Info",
			action: "get_artifact_info",
			params: map[string]interface{}{
				"path": "maven-local/org/example/app/1.0.0/app-1.0.0.jar",
			},
			expectError: false,
		},
		{
			name: "Get Repository Info",
			action: "get_repository_info",
			params: map[string]interface{}{
				"repo_key": "maven-local",
			},
			expectError: false,
		},
		{
			name: "Get Folder Content",
			action: "get_folder_content",
			params: map[string]interface{}{
				"path": "maven-local",
			},
			expectError: false,
		},
		{
			name: "Calculate Checksum",
			action: "calculate_checksum",
			params: map[string]interface{}{
				"path": "maven-local/org/example/app/1.0.0/app-1.0.0.jar",
			},
			expectError: false,
		},
		{
			name: "Download Artifact",
			action: "download_artifact",
			params: map[string]interface{}{
				"path": "maven-local/org/example/app/1.0.0/app-1.0.0.jar",
			},
			expectError: false,
		},
		{
			name: "Invalid Action",
			action: "invalid_action",
			params: map[string]interface{}{},
			expectError: true,
		},
		{
			name: "Missing Parameters",
			action: "get_artifact_info",
			params: map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := adapter.ExecuteAction(context.Background(), "test-context-id", tc.action, tc.params)
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestHealth(t *testing.T) {
	// Create a healthy test server
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/system/ping" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer healthyServer.Close()

	// Create an unhealthy test server
	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthyServer.Close()

	// Create adapters
	healthyAdapter, _ := NewAdapter(Config{
		BaseURL:        healthyServer.URL,
		RequestTimeout: 1 * time.Second,
	})
	
	unhealthyAdapter, _ := NewAdapter(Config{
		BaseURL:        unhealthyServer.URL,
		RequestTimeout: 1 * time.Second,
	})
	
	// Initialize adapters
	healthyAdapter.Initialize(context.Background(), nil)
	unhealthyAdapter.Initialize(context.Background(), nil)
	
	// Test health status
	assert.Equal(t, "healthy", healthyAdapter.Health())
	assert.Contains(t, unhealthyAdapter.Health(), "unhealthy")
}

func TestClose(t *testing.T) {
	adapter, _ := NewAdapter(Config{})
	err := adapter.Close()
	assert.NoError(t, err)
}

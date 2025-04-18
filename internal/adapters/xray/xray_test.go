package xray

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
		BaseURL:        "https://xray.example.com",
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
		"scan_artifact",
		"scan_build",
		"get_vulnerabilities",
		"get_licenses",
		"generate_vulnerabilities_report",
		"get_component_details",
		"get_scan_status",
	}

	for _, op := range safeOps {
		t.Run(op, func(t *testing.T) {
			safe, err := adapter.IsSafeOperation(op, nil)
			assert.True(t, safe)
			assert.NoError(t, err)
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
			
		case r.URL.Path == "/summary/artifact" && r.Method == "POST":
			// For vulnerabilities or licenses query
			json.NewEncoder(w).Encode(map[string]interface{}{
				"artifacts": []map[string]interface{}{
					{
						"general": map[string]interface{}{
							"path": "com.example:library:1.0.0",
						},
						"issues": []map[string]interface{}{
							{
								"issue_id": "CVE-2023-12345",
								"summary": "Test vulnerability",
								"description": "A test vulnerability",
								"severity": "HIGH",
								"vulnerable_components": []map[string]interface{}{
									{
										"component_id": "com.example:library:1.0.0",
										"fixed_versions": []string{"1.0.1"},
									},
								},
							},
						},
						"licenses": []map[string]interface{}{
							{
								"license_key": "MIT",
								"full_name": "MIT License",
								"more_info_url": "https://opensource.org/licenses/MIT",
								"components": []map[string]interface{}{
									{
										"component_id": "com.example:library:1.0.0",
									},
								},
							},
						},
					},
				},
			})
			
		case r.URL.Path == "/component_summary" && r.Method == "POST":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"components": 10,
				"vulnerable_components": 2,
				"vulnerabilities": map[string]interface{}{
					"high": 1,
					"medium": 1,
					"low": 0,
					"unknown": 0,
				},
				"licenses": map[string]interface{}{
					"MIT": 5,
					"Apache-2.0": 3,
					"GPL-3.0": 2,
				},
				"top_vulnerable_components": []map[string]interface{}{
					{
						"component_id": "com.example:library:1.0.0",
						"vulnerabilities": map[string]interface{}{
							"high": 1,
							"medium": 0,
							"low": 0,
							"unknown": 0,
						},
					},
				},
			})
			
		case r.URL.Path == "/system/info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"server_id": "xray-server-123",
				"version": "3.0.0",
				"db_provider": "PostgreSQL",
			})
			
		case r.URL.Path == "/system/version":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"version": "3.0.0",
				"revision": "12345",
			})
			
		case r.URL.Path == "/watches":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"watches": []map[string]interface{}{
					{
						"id": "watch-123",
						"name": "Test Watch",
						"description": "A test watch",
					},
				},
			})
			
		case r.URL.Path == "/policies":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"policies": []map[string]interface{}{
					{
						"id": "policy-123",
						"name": "Test Policy",
						"description": "A test policy",
					},
				},
			})
			
		case r.URL.Path == "/summary/common":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"security_violations_count": 5,
				"license_violations_count": 2,
				"components_count": 100,
				"vulnerable_components_count": 10,
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
			name: "Get Vulnerabilities",
			operation: "get_vulnerabilities",
			params: map[string]interface{}{
				"path": "com.example:library:1.0.0",
			},
			expectError: false,
		},
		{
			name: "Get Vulnerabilities with Severity",
			operation: "get_vulnerabilities",
			params: map[string]interface{}{
				"path": "com.example:library:1.0.0",
				"severity": "HIGH",
			},
			expectError: false,
		},
		{
			name: "Get Licenses",
			operation: "get_licenses",
			params: map[string]interface{}{
				"path": "com.example:library:1.0.0",
			},
			expectError: false,
		},
		{
			name: "Get Component Summary",
			operation: "get_component_summary",
			params: map[string]interface{}{
				"path": "com.example:library:1.0.0",
			},
			expectError: false,
		},
		{
			name: "Get System Info",
			operation: "get_system_info",
			params: map[string]interface{}{},
			expectError: false,
		},
		{
			name: "Get System Version",
			operation: "get_system_version",
			params: map[string]interface{}{},
			expectError: false,
		},
		{
			name: "Get Watches",
			operation: "get_watches",
			params: map[string]interface{}{},
			expectError: false,
		},
		{
			name: "Get Policies",
			operation: "get_policies",
			params: map[string]interface{}{},
			expectError: false,
		},
		{
			name: "Get Summary",
			operation: "get_summary",
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
			name: "Missing Path Parameter",
			operation: "get_vulnerabilities",
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
			
		case r.URL.Path == "/component/scan" && r.Method == "POST":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"scan_id": "scan-123456",
				"status": "in_progress",
				"started_at": time.Now().Format(time.RFC3339),
			})
			
		case r.URL.Path == "/scanBuild" && r.Method == "POST":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"build_name": "my-app",
				"build_number": "42",
				"status": "in_progress",
			})
			
		case r.URL.Path == "/summary/artifact" && r.Method == "POST":
			// For vulnerabilities or licenses query
			json.NewEncoder(w).Encode(map[string]interface{}{
				"artifacts": []map[string]interface{}{
					{
						"general": map[string]interface{}{
							"path": "com.example:library:1.0.0",
						},
						"issues": []map[string]interface{}{
							{
								"issue_id": "CVE-2023-12345",
								"summary": "Test vulnerability",
								"description": "A test vulnerability",
								"severity": "HIGH",
								"vulnerable_components": []map[string]interface{}{
									{
										"component_id": "com.example:library:1.0.0",
										"fixed_versions": []string{"1.0.1"},
									},
								},
							},
						},
						"licenses": []map[string]interface{}{
							{
								"license_key": "MIT",
								"full_name": "MIT License",
								"more_info_url": "https://opensource.org/licenses/MIT",
								"components": []map[string]interface{}{
									{
										"component_id": "com.example:library:1.0.0",
									},
								},
							},
						},
					},
				},
			})
			
		case r.URL.Path == "/reports/vulnerabilities" && r.Method == "POST":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"report_id": "report-123",
				"status": "generated",
				"created_at": time.Now().Format(time.RFC3339),
			})
			
		case r.URL.Path == "/component/com.example:library:1.0.0":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"component_id": "com.example:library:1.0.0",
				"name": "library",
				"version": "1.0.0",
				"group": "com.example",
				"vulnerabilities": []map[string]interface{}{
					{
						"id": "CVE-2023-12345",
						"severity": "HIGH",
					},
				},
			})
			
		case r.URL.Path == "/scan/scan-123":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"scan_id": "scan-123",
				"status": "completed",
				"created_at": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
				"completed_at": time.Now().Format(time.RFC3339),
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
		action      string
		params      map[string]interface{}
		expectError bool
	}{
		{
			name: "Scan Artifact",
			action: "scan_artifact",
			params: map[string]interface{}{
				"path": "com.example:library:1.0.0",
			},
			expectError: false,
		},
		{
			name: "Scan Build",
			action: "scan_build",
			params: map[string]interface{}{
				"build_name": "my-app",
				"build_number": "42",
			},
			expectError: false,
		},
		{
			name: "Get Vulnerabilities",
			action: "get_vulnerabilities",
			params: map[string]interface{}{
				"path": "com.example:library:1.0.0",
			},
			expectError: false,
		},
		{
			name: "Get Licenses",
			action: "get_licenses",
			params: map[string]interface{}{
				"path": "com.example:library:1.0.0",
			},
			expectError: false,
		},
		{
			name: "Generate Vulnerabilities Report",
			action: "generate_vulnerabilities_report",
			params: map[string]interface{}{
				"name": "test-report",
				"repositories": []interface{}{"maven-local"},
			},
			expectError: false,
		},
		{
			name: "Get Component Details",
			action: "get_component_details",
			params: map[string]interface{}{
				"component_id": "com.example:library:1.0.0",
			},
			expectError: false,
		},
		{
			name: "Get Scan Status",
			action: "get_scan_status",
			params: map[string]interface{}{
				"scan_id": "scan-123",
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
			action: "scan_artifact",
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

func TestExtractVersionFromComponentID(t *testing.T) {
	testCases := []struct {
		name        string
		componentID string
		expected    string
	}{
		{
			name:        "Standard Format",
			componentID: "com.example:library:1.0.0",
			expected:    "1.0.0",
		},
		{
			name:        "No Version",
			componentID: "com.example:library",
			expected:    "library",
		},
		{
			name:        "Single Element",
			componentID: "library",
			expected:    "",
		},
		{
			name:        "Multiple Colons",
			componentID: "org:com.example:library:1.0.0:jar",
			expected:    "jar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractVersionFromComponentID(tc.componentID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestClose(t *testing.T) {
	adapter, _ := NewAdapter(Config{})
	err := adapter.Close()
	assert.NoError(t, err)
}

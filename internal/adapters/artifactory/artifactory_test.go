package artifactory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/S-Corkum/mcp-server/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestNewAdapter(t *testing.T) {
	// Test with minimal config
	cfg := Config{
		BaseURL:        "https://artifactory.example.com",
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
	}

	adapter, err := NewAdapter(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, cfg.BaseURL, adapter.config.BaseURL)
	assert.Equal(t, cfg.RequestTimeout, adapter.config.RequestTimeout)
	assert.Equal(t, cfg.MaxRetries, adapter.baseAdapter.RetryMax)
	assert.Equal(t, cfg.RetryDelay, adapter.baseAdapter.RetryDelay)
	assert.NotNil(t, adapter.subscribers)
	assert.NotNil(t, adapter.client)
}

func TestAddAuthHeader(t *testing.T) {
	testCases := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "Access Token Auth",
			config: Config{
				AccessToken: "abc123",
			},
			expected: "Bearer abc123",
		},
		{
			name: "API Key Auth",
			config: Config{
				ApiKey: "apikey123",
			},
			expected: "apikey123",
		},
		{
			name: "Basic Auth",
			config: Config{
				Username: "user",
				Password: "pass",
			},
			expected: "Basic dXNlcjpwYXNz", // "user:pass" in Base64
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := NewAdapter(tc.config)
			req, _ := http.NewRequest("GET", "https://example.com", nil)
			adapter.addAuthHeader(req)

			switch {
			case tc.config.AccessToken != "":
				assert.Equal(t, tc.expected, req.Header.Get("Authorization"))
			case tc.config.ApiKey != "":
				assert.Equal(t, tc.expected, req.Header.Get("X-JFrog-Art-Api"))
			default:
				assert.Equal(t, tc.expected, req.Header.Get("Authorization"))
			}
		})
	}
}

func TestBuildQueryParams(t *testing.T) {
	adapter, _ := NewAdapter(Config{BaseURL: "https://artifactory.example.com"})

	testCases := []struct {
		name           string
		query          models.ArtifactoryQuery
		expectedPath   string
		expectedParams string
	}{
		{
			name: "Repository Query",
			query: models.ArtifactoryQuery{
				Type:        models.ArtifactoryQueryTypeRepository,
				RepoType:    "local",
				PackageType: "maven",
			},
			expectedPath:   "/api/repositories",
			expectedParams: "packageType=maven&type=local",
		},
		{
			name: "Artifact Query",
			query: models.ArtifactoryQuery{
				Type:    models.ArtifactoryQueryTypeArtifact,
				RepoKey: "maven-local",
				Path:    "org/example/app/1.0.0/app-1.0.0.jar",
			},
			expectedPath:   "/api/storage/maven-local/org/example/app/1.0.0/app-1.0.0.jar",
			expectedParams: "properties=1&stats=1",
		},
		{
			name: "Build Query With Name and Number",
			query: models.ArtifactoryQuery{
				Type:        models.ArtifactoryQueryTypeBuild,
				BuildName:   "my-app",
				BuildNumber: "42",
			},
			expectedPath:   "/api/build/my-app/42",
			expectedParams: "",
		},
		{
			name: "Build Query Without Name",
			query: models.ArtifactoryQuery{
				Type: models.ArtifactoryQueryTypeBuild,
			},
			expectedPath:   "/api/build",
			expectedParams: "",
		},
		{
			name: "Storage Query",
			query: models.ArtifactoryQuery{
				Type: models.ArtifactoryQueryTypeStorage,
			},
			expectedPath:   "/api/storageinfo",
			expectedParams: "",
		},
		{
			name: "Unknown Query Type",
			query: models.ArtifactoryQuery{
				Type: "unknown",
			},
			expectedPath:   "/api/unknown",
			expectedParams: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, params := adapter.buildQueryParams(tc.query)
			assert.Equal(t, tc.expectedPath, path)
			
			// Compare query parameters regardless of order
			expectedParams := make(map[string]struct{})
			for _, param := range strings.Split(tc.expectedParams, "&") {
				if param != "" {
					expectedParams[param] = struct{}{}
				}
			}
			
			actualParams := make(map[string]struct{})
			for _, param := range strings.Split(params, "&") {
				if param != "" {
					actualParams[param] = struct{}{}
				}
			}
			
			assert.Equal(t, expectedParams, actualParams)
		})
	}
}

func TestIsSafeOperation(t *testing.T) {
	adapter, _ := NewAdapter(Config{})

	// Test cases for safe operations
	safeOps := []string{
		"get_artifact",
		"get_repository_info",
		"list_repositories",
		"search_artifacts",
	}

	for _, op := range safeOps {
		t.Run(op, func(t *testing.T) {
			safe, err := adapter.IsSafeOperation(op, nil)
			assert.True(t, safe)
			assert.NoError(t, err)
		})
	}

	// Test case for unsafe operation
	unsafe := "delete_repository"
	safe, err := adapter.IsSafeOperation(unsafe, nil)
	assert.False(t, safe)
	assert.NoError(t, err)
}

func TestHandleWebhook(t *testing.T) {
	adapter, _ := NewAdapter(Config{})

	// Mock subscriber
	receivedEvent := false
	adapter.Subscribe("artifact", func(event interface{}) {
		receivedEvent = true
	})

	// Create test webhook payload
	webhookEvent := models.ArtifactoryWebhookEvent{
		Domain:    "artifact",
		EventType: "created",
		Data: models.ArtifactoryWebhookData{
			RepoKey: "maven-local",
			Path:    "org/example/app",
			Name:    "app-1.0.0.jar",
		},
	}

	payload, _ := json.Marshal(webhookEvent)

	// Test handle webhook
	err := adapter.HandleWebhook(context.Background(), "webhook", payload)
	assert.NoError(t, err)

	// Verify the event was processed
	time.Sleep(10 * time.Millisecond) // Give time for the goroutine to execute
	assert.True(t, receivedEvent)
}

func TestDetermineEventType(t *testing.T) {
	adapter, _ := NewAdapter(Config{})

	// Test with valid event
	event := models.ArtifactoryWebhookEvent{
		Domain: "artifact",
	}
	eventType := adapter.determineEventType(event)
	assert.Equal(t, "artifact", eventType)

	// Test with invalid event type
	invalidEvent := "not an event"
	eventType = adapter.determineEventType(invalidEvent)
	assert.Equal(t, "unknown", eventType)
}

func TestParseWebhookEvent(t *testing.T) {
	adapter, _ := NewAdapter(Config{})

	// Create valid webhook payload
	webhookEvent := models.ArtifactoryWebhookEvent{
		Domain:    "artifact",
		EventType: "created",
		Data: models.ArtifactoryWebhookData{
			RepoKey: "maven-local",
			Path:    "org/example/app",
			Name:    "app-1.0.0.jar",
		},
	}

	payload, _ := json.Marshal(webhookEvent)

	// Test parsing valid payload
	event, err := adapter.parseWebhookEvent(payload)
	assert.NoError(t, err)
	assert.NotNil(t, event)

	parsedEvent, ok := event.(models.ArtifactoryWebhookEvent)
	assert.True(t, ok)
	assert.Equal(t, webhookEvent.Domain, parsedEvent.Domain)
	assert.Equal(t, webhookEvent.EventType, parsedEvent.EventType)
	assert.Equal(t, webhookEvent.Data.RepoKey, parsedEvent.Data.RepoKey)

	// Test parsing invalid payload
	_, err = adapter.parseWebhookEvent([]byte("not json"))
	assert.Error(t, err)
}

func TestGetData(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the request path and return appropriate mock response
		switch {
		case strings.Contains(r.URL.Path, "/api/repositories"):
			// Mock repositories response
			resp := models.ArtifactoryRepositories{
				Repositories: []models.ArtifactoryRepository{
					{
						Key:         "maven-local",
						Type:        "local",
						Description: "Local Maven repository",
						URL:         "https://artifactory.example.com/artifactory/maven-local",
						PackageType: "maven",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			
		case strings.Contains(r.URL.Path, "/api/storage"):
			// Mock artifact response
			resp := models.ArtifactoryArtifact{
				Repo:       "maven-local",
				Path:       "org/example/app/1.0.0",
				Name:       "app-1.0.0.jar",
				Size:       1024,
				Created:    "2023-01-01T00:00:00Z",
				Properties: map[string][]string{"build.name": {"my-app"}},
			}
			json.NewEncoder(w).Encode(resp)
			
		case strings.Contains(r.URL.Path, "/api/build"):
			// Mock build response
			resp := models.ArtifactoryBuild{}
			resp.BuildInfo.Name = "my-app"
			resp.BuildInfo.Number = "42"
			json.NewEncoder(w).Encode(resp)
			
		case strings.Contains(r.URL.Path, "/api/storageinfo"):
			// Mock storage response
			resp := models.ArtifactoryStorage{}
			resp.BinariesSummary.BinariesCount = 100
			resp.BinariesSummary.BinariesSize = 1048576
			json.NewEncoder(w).Encode(resp)
			
		default:
			// Return 404 for unknown paths
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create adapter with test server URL
	adapter, _ := NewAdapter(Config{
		BaseURL:        server.URL,
		RequestTimeout: 1 * time.Second,
	})

	testCases := []struct {
		name        string
		query       models.ArtifactoryQuery
		expectError bool
	}{
		{
			name: "Repository Query",
			query: models.ArtifactoryQuery{
				Type: models.ArtifactoryQueryTypeRepository,
			},
			expectError: false,
		},
		{
			name: "Artifact Query",
			query: models.ArtifactoryQuery{
				Type:    models.ArtifactoryQueryTypeArtifact,
				RepoKey: "maven-local",
				Path:    "org/example/app/1.0.0/app-1.0.0.jar",
			},
			expectError: false,
		},
		{
			name: "Build Query",
			query: models.ArtifactoryQuery{
				Type:        models.ArtifactoryQueryTypeBuild,
				BuildName:   "my-app",
				BuildNumber: "42",
			},
			expectError: false,
		},
		{
			name: "Storage Query",
			query: models.ArtifactoryQuery{
				Type: models.ArtifactoryQueryTypeStorage,
			},
			expectError: false,
		},
		{
			name: "Invalid Query Type",
			query: models.ArtifactoryQuery{
				Type: "invalid",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := adapter.GetData(context.Background(), tc.query)
			
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				
				// Verify the type of the result based on query type
				switch tc.query.Type {
				case models.ArtifactoryQueryTypeRepository:
					_, ok := result.(models.ArtifactoryRepositories)
					assert.True(t, ok)
				case models.ArtifactoryQueryTypeArtifact:
					_, ok := result.(models.ArtifactoryArtifact)
					assert.True(t, ok)
				case models.ArtifactoryQueryTypeBuild:
					_, ok := result.(models.ArtifactoryBuild)
					assert.True(t, ok)
				case models.ArtifactoryQueryTypeStorage:
					_, ok := result.(models.ArtifactoryStorage)
					assert.True(t, ok)
				}
			}
		})
	}

	// Test with invalid query type
	_, err := adapter.GetData(context.Background(), "invalid")
	assert.Error(t, err)
}

func TestDownloadArtifact(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for valid path
		if strings.Contains(r.URL.Path, "maven-local/org/example/app") {
			w.Write([]byte("mock artifact content"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create adapter with test server URL
	adapter, _ := NewAdapter(Config{
		BaseURL:        server.URL,
		RequestTimeout: 1 * time.Second,
	})

	// Test with valid path
	content, err := adapter.DownloadArtifact(context.Background(), "maven-local", "org/example/app/1.0.0/app-1.0.0.jar")
	assert.NoError(t, err)
	assert.Equal(t, []byte("mock artifact content"), content)

	// Test with invalid path
	_, err = adapter.DownloadArtifact(context.Background(), "invalid", "path")
	assert.Error(t, err)
}

func TestExecuteAction(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return different responses based on the request path
		switch {
		case strings.Contains(r.URL.Path, "maven-local/org/example/app"):
			w.Write([]byte("mock artifact content"))
		case strings.Contains(r.URL.Path, "/api/repositories"):
			json.NewEncoder(w).Encode(models.ArtifactoryRepositories{
				Repositories: []models.ArtifactoryRepository{
					{Key: "maven-local", Type: "local"},
				},
			})
		case strings.Contains(r.URL.Path, "/api/storage"):
			json.NewEncoder(w).Encode(models.ArtifactoryArtifact{
				Repo: "maven-local",
				Path: "org/example/app",
				Name: "app-1.0.0.jar",
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

	testCases := []struct {
		name        string
		action      string
		params      map[string]interface{}
		expectError bool
	}{
		{
			name:   "Get Artifact",
			action: "get_artifact",
			params: map[string]interface{}{
				"repo": "maven-local",
				"path": "org/example/app/1.0.0/app-1.0.0.jar",
			},
			expectError: false,
		},
		{
			name:   "Get Repository Info",
			action: "get_repository_info",
			params: map[string]interface{}{
				"repo": "maven-local",
			},
			expectError: false,
		},
		{
			name:   "List Repositories",
			action: "list_repositories",
			params: map[string]interface{}{
				"type":         "local",
				"package_type": "maven",
			},
			expectError: false,
		},
		{
			name:   "Search Artifacts",
			action: "search_artifacts",
			params: map[string]interface{}{
				"repo": "maven-local",
				"path": "org/example",
			},
			expectError: false,
		},
		{
			name:        "Unsupported Action",
			action:      "unsupported",
			params:      map[string]interface{}{},
			expectError: true,
		},
		{
			name:        "Missing Repo Parameter",
			action:      "get_artifact",
			params:      map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := adapter.ExecuteAction(context.Background(), "test-context", tc.action, tc.params)
			
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
		if r.URL.Path == "/api/system/ping" {
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

	// Test with healthy server
	healthyAdapter, _ := NewAdapter(Config{
		BaseURL:        healthyServer.URL,
		RequestTimeout: 1 * time.Second,
	})
	
	status := healthyAdapter.Health()
	assert.Equal(t, "healthy", status)

	// Test with unhealthy server
	unhealthyAdapter, _ := NewAdapter(Config{
		BaseURL:        unhealthyServer.URL,
		RequestTimeout: 1 * time.Second,
	})
	
	status = unhealthyAdapter.Health()
	assert.Contains(t, status, "unhealthy")
}

func TestClose(t *testing.T) {
	adapter, _ := NewAdapter(Config{})
	err := adapter.Close()
	assert.NoError(t, err)
}

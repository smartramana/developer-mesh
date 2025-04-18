package artifactory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
)

// Config holds configuration for the Artifactory adapter
type Config struct {
	BaseURL         string        `mapstructure:"base_url"`
	Token           string        `mapstructure:"token"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	RetryMax        int           `mapstructure:"retry_max"`
	RetryDelay      time.Duration `mapstructure:"retry_delay"`
	MockResponses   bool          `mapstructure:"mock_responses"`
	MockURL         string        `mapstructure:"mock_url"`
}

// Adapter implements the adapter interface for Artifactory
type Adapter struct {
	adapters.BaseAdapter
	config       Config
	client       *http.Client
	healthStatus string
}

// NewAdapter creates a new Artifactory adapter
func NewAdapter(config Config) (*Adapter, error) {
	// Set default values if not provided
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.RetryMax == 0 {
		config.RetryMax = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://artifactory.example.com/artifactory/api"
	}

	adapter := &Adapter{
		BaseAdapter: adapters.BaseAdapter{
			RetryMax:   config.RetryMax,
			RetryDelay: config.RetryDelay,
		},
		config: config,
		healthStatus: "initializing",
	}

	return adapter, nil
}

// Initialize sets up the adapter with the Artifactory client
func (a *Adapter) Initialize(ctx context.Context, cfg interface{}) error {
	// Parse config if provided
	if cfg != nil {
		config, ok := cfg.(Config)
		if !ok {
			return fmt.Errorf("invalid config type: %T", cfg)
		}
		a.config = config
	}

	// Validate configuration
	if a.config.Token == "" && (a.config.Username == "" || a.config.Password == "") {
		return fmt.Errorf("Artifactory authentication credentials are required")
	}

	// Create HTTP client
	a.client = &http.Client{
		Timeout: a.config.RequestTimeout,
	}

	// Test the connection
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		// Don't return error, just make the adapter usable in degraded mode
		log.Printf("Warning: Failed to connect to Artifactory API: %v", err)
	} else {
		a.healthStatus = "healthy"
	}

	return nil
}

// testConnection verifies connectivity to Artifactory
func (a *Adapter) testConnection(ctx context.Context) error {
	// If mock_responses is enabled, try connecting to the mock server instead
	if a.config.MockResponses && a.config.MockURL != "" {
		// Create a custom HTTP client for testing the mock connection
		httpClient := &http.Client{Timeout: a.config.RequestTimeout}
		
		// First try health endpoint which should be more reliable
		healthURL := a.config.MockURL + "/health"
		resp, err := httpClient.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			log.Println("Successfully connected to mock Artifactory API health endpoint")
			a.healthStatus = "healthy"
			return nil
		}
		
		// Fall back to regular mock URL
		resp, err = httpClient.Get(a.config.MockURL)
		if err != nil {
			a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to mock Artifactory API: %v", err)
			return fmt.Errorf("failed to connect to mock Artifactory API: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			a.healthStatus = fmt.Sprintf("unhealthy: mock Artifactory API returned status code: %d", resp.StatusCode)
			return fmt.Errorf("mock Artifactory API returned unexpected status code: %d", resp.StatusCode)
		}
		
		// Successfully connected to mock server
		log.Println("Successfully connected to mock Artifactory API")
		a.healthStatus = "healthy"
		return nil
	}
	
	// Simple ping to the Artifactory API
	// In a real implementation, this would verify with the system/ping endpoint
	// But for now we'll just return healthy status
	a.healthStatus = "healthy"
	return nil
}

// GetData retrieves data from Artifactory
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	// Parse the query
	queryMap, ok := query.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid query type: %T", query)
	}

	// Check the operation type
	operation, ok := queryMap["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("missing operation in query")
	}

	// Handle different operations
	switch operation {
	case "get_artifact_info":
		return a.getArtifactInfo(ctx, queryMap)
	case "search_artifacts":
		return a.searchArtifacts(ctx, queryMap)
	case "get_build_info":
		return a.getBuildInfo(ctx, queryMap)
	case "get_repositories":
		return a.getRepositories(ctx, queryMap)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Handle different actions
	switch action {
	case "download_artifact":
		return a.downloadArtifact(ctx, params)
	case "get_artifact_info":
		return a.getArtifactInfo(ctx, params)
	case "search_artifacts":
		return a.searchArtifacts(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// IsSafeOperation checks if an operation is safe to perform
func (a *Adapter) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	// Only read operations are considered safe for Artifactory
	// (we're implementing as a read-only adapter per requirements)
	safeActions := map[string]bool{
		"download_artifact": true,
		"get_artifact_info": true,
		"search_artifacts":  true,
	}

	// Check if the action is in the safe list
	if safe, ok := safeActions[action]; ok && safe {
		return true, nil
	}

	// Check for unsafe operations
	unsafeActions := map[string]bool{
		"upload_artifact":   false,
		"delete_artifact":   false,
		"move_artifact":     false,
		"delete_repository": false,
	}

	if unsafe, ok := unsafeActions[action]; ok && !unsafe {
		return false, fmt.Errorf("operation '%s' is unsafe and not allowed", action)
	}

	// Default to safe if unknown
	return true, nil
}

// getArtifactInfo gets information about a specific artifact
func (a *Adapter) getArtifactInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing path parameter")
	}

	// In a real implementation, this would call the Artifactory API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/storage/%s", a.config.MockURL, path)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		if a.config.Token != "" {
			req.Header.Set("X-JFrog-Art-Api", a.config.Token)
		} else {
			req.SetBasicAuth(a.config.Username, a.config.Password)
		}
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get artifact info from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Mock response for testing
	return map[string]interface{}{
		"uri":         path,
		"downloadUri": fmt.Sprintf("https://artifactory.example.com/artifactory/%s", path),
		"repo":        "libs-release",
		"path":        path,
		"created":     time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
		"createdBy":   "user",
		"size":        "15281024",
		"mimeType":    "application/java-archive",
		"checksums": map[string]interface{}{
			"md5":    "abcdef1234567890abcdef1234567890",
			"sha1":   "abcdef1234567890abcdef1234567890abcdef12",
			"sha256": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		"originalChecksums": map[string]interface{}{
			"md5":    "abcdef1234567890abcdef1234567890",
			"sha1":   "abcdef1234567890abcdef1234567890abcdef12",
			"sha256": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
	}, nil
}

// searchArtifacts searches for artifacts
func (a *Adapter) searchArtifacts(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	var query string
	if queryParam, ok := params["query"].(string); ok {
		query = queryParam
	} else {
		// Default to empty query (match all)
		query = ""
	}

	// Extract optional parameters
	repo := ""
	if repoParam, ok := params["repo"].(string); ok {
		repo = repoParam
	}

	// In a real implementation, this would call the Artifactory API search
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/search/artifact?name=%s", a.config.MockURL, query)
		if repo != "" {
			url += "&repos=" + repo
		}
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		if a.config.Token != "" {
			req.Header.Set("X-JFrog-Art-Api", a.config.Token)
		} else {
			req.SetBasicAuth(a.config.Username, a.config.Password)
		}
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to search artifacts from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Mock response for testing
	return map[string]interface{}{
		"results": []map[string]interface{}{
			{
				"uri":         "libs-release/org/example/app/1.0.0/app-1.0.0.jar",
				"downloadUri": "https://artifactory.example.com/artifactory/libs-release/org/example/app/1.0.0/app-1.0.0.jar",
				"repo":        "libs-release",
				"path":        "org/example/app/1.0.0/app-1.0.0.jar",
				"created":     time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
				"size":        "15281024",
			},
			{
				"uri":         "libs-release/org/example/app/1.0.1/app-1.0.1.jar",
				"downloadUri": "https://artifactory.example.com/artifactory/libs-release/org/example/app/1.0.1/app-1.0.1.jar",
				"repo":        "libs-release",
				"path":        "org/example/app/1.0.1/app-1.0.1.jar",
				"created":     time.Now().AddDate(0, 0, -15).Format(time.RFC3339),
				"size":        "15348224",
			},
			{
				"uri":         "libs-release-local/org/example/app/1.1.0/app-1.1.0.jar",
				"downloadUri": "https://artifactory.example.com/artifactory/libs-release-local/org/example/app/1.1.0/app-1.1.0.jar",
				"repo":        "libs-release-local",
				"path":        "org/example/app/1.1.0/app-1.1.0.jar",
				"created":     time.Now().AddDate(0, 0, -2).Format(time.RFC3339),
				"size":        "15782912",
			},
		},
		"range": map[string]interface{}{
			"start_pos": 0,
			"end_pos":   3,
			"total":     3,
		},
	}, nil
}

// getBuildInfo gets build information
func (a *Adapter) getBuildInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	buildName, ok := params["build_name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing build_name parameter")
	}

	buildNumber, ok := params["build_number"].(string)
	if !ok {
		return nil, fmt.Errorf("missing build_number parameter")
	}

	// In a real implementation, this would call the Artifactory API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/build/%s/%s", a.config.MockURL, buildName, buildNumber)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		if a.config.Token != "" {
			req.Header.Set("X-JFrog-Art-Api", a.config.Token)
		} else {
			req.SetBasicAuth(a.config.Username, a.config.Password)
		}
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get build info from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Mock response for testing
	buildStartDate := time.Now().AddDate(0, 0, -1).Format(time.RFC3339)
	return map[string]interface{}{
		"buildInfo": map[string]interface{}{
			"version":    "1.0.1",
			"name":       buildName,
			"number":     buildNumber,
			"started":    buildStartDate,
			"url":        fmt.Sprintf("https://ci.example.com/build/%s/%s", buildName, buildNumber),
			"vcsRevision": "abcdef1234567890",
			"modules": []map[string]interface{}{
				{
					"id":      "org.example:app:1.0.1",
					"type":    "maven",
					"artifacts": []map[string]interface{}{
						{
							"name":     "app-1.0.1.jar",
							"type":     "jar",
							"sha1":     "abcdef1234567890abcdef1234567890abcdef12",
							"md5":      "abcdef1234567890abcdef1234567890",
							"size":     15348224,
						},
						{
							"name":     "app-1.0.1-sources.jar",
							"type":     "java-source-jar",
							"sha1":     "abcdef1234567890abcdef1234567890abcdef12",
							"md5":      "abcdef1234567890abcdef1234567890",
							"size":     5243392,
						},
					},
					"dependencies": []map[string]interface{}{
						{
							"id":       "com.example:common:1.2.3",
							"type":     "jar",
							"scopes":   []string{"compile"},
							"sha1":     "abcdef1234567890abcdef1234567890abcdef12",
							"md5":      "abcdef1234567890abcdef1234567890",
						},
						{
							"id":       "org.springframework:spring-core:5.3.9",
							"type":     "jar",
							"scopes":   []string{"compile"},
							"sha1":     "abcdef1234567890abcdef1234567890abcdef12",
							"md5":      "abcdef1234567890abcdef1234567890",
						},
					},
				},
			},
		},
	}, nil
}

// getRepositories gets a list of repositories
func (a *Adapter) getRepositories(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	type_ := ""
	if typeParam, ok := params["type"].(string); ok {
		type_ = typeParam
	}

	// In a real implementation, this would call the Artifactory API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/repositories", a.config.MockURL)
		if type_ != "" {
			url += "?type=" + type_
		}
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		if a.config.Token != "" {
			req.Header.Set("X-JFrog-Art-Api", a.config.Token)
		} else {
			req.SetBasicAuth(a.config.Username, a.config.Password)
		}
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get repositories from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return map[string]interface{}{
			"repositories": result,
		}, nil
	}

	// Mock response for testing with different repo types
	var repos []map[string]interface{}
	
	// Always include local repos
	if type_ == "" || type_ == "local" {
		repos = append(repos, []map[string]interface{}{
			{
				"key":         "libs-release-local",
				"type":        "local",
				"description": "Local repository for release artifacts",
				"url":         "https://artifactory.example.com/artifactory/libs-release-local",
				"packageType": "maven",
			},
			{
				"key":         "libs-snapshot-local",
				"type":        "local",
				"description": "Local repository for snapshot artifacts",
				"url":         "https://artifactory.example.com/artifactory/libs-snapshot-local",
				"packageType": "maven",
			},
			{
				"key":         "docker-local",
				"type":        "local",
				"description": "Local Docker registry",
				"url":         "https://artifactory.example.com/artifactory/docker-local",
				"packageType": "docker",
			},
		}...)
	}
	
	// Include remote repos if requested
	if type_ == "" || type_ == "remote" {
		repos = append(repos, []map[string]interface{}{
			{
				"key":         "maven-central",
				"type":        "remote",
				"description": "Maven Central",
				"url":         "https://artifactory.example.com/artifactory/maven-central",
				"packageType": "maven",
				"remoteUrl":   "https://repo.maven.apache.org/maven2/",
			},
			{
				"key":         "npm-remote",
				"type":        "remote",
				"description": "NPM Registry",
				"url":         "https://artifactory.example.com/artifactory/npm-remote",
				"packageType": "npm",
				"remoteUrl":   "https://registry.npmjs.org/",
			},
		}...)
	}
	
	// Include virtual repos if requested
	if type_ == "" || type_ == "virtual" {
		repos = append(repos, []map[string]interface{}{
			{
				"key":         "libs-release",
				"type":        "virtual",
				"description": "Virtual repository for release artifacts",
				"url":         "https://artifactory.example.com/artifactory/libs-release",
				"packageType": "maven",
				"repositories": []string{
					"libs-release-local",
					"maven-central",
				},
			},
			{
				"key":         "libs-snapshot",
				"type":        "virtual",
				"description": "Virtual repository for snapshot artifacts",
				"url":         "https://artifactory.example.com/artifactory/libs-snapshot",
				"packageType": "maven",
				"repositories": []string{
					"libs-snapshot-local",
				},
			},
		}...)
	}

	return map[string]interface{}{
		"repositories": repos,
	}, nil
}

// downloadArtifact downloads an artifact (returns the download URL in this implementation)
func (a *Adapter) downloadArtifact(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing path parameter")
	}

	// In a real implementation, this would download the artifact or return a pre-signed URL
	// For now, we'll return a mock download URL

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/artifact/%s", a.config.MockURL, path)
		
		// Just return the URL (in a real implementation, we might follow redirects and download)
		return map[string]interface{}{
			"downloadUrl": url,
			"path":        path,
		}, nil
	}

	// Mock response for testing
	return map[string]interface{}{
		"downloadUrl": fmt.Sprintf("https://artifactory.example.com/artifactory/%s", path),
		"path":        path,
	}, nil
}

// Subscribe is a no-op since we're not using webhooks
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
	// No-op for API-only operation
	return nil
}

// HandleWebhook is a no-op since we're not using webhooks
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// No-op for API-only operation
	return nil
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	return a.healthStatus
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing specific to clean up
	return nil
}

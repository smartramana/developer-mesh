package artifactory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
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
	APIVersion      string        `mapstructure:"api_version"`
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
	if config.APIVersion == "" {
		config.APIVersion = "v1"
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
	
	// Test connectivity to real Artifactory API using system/ping endpoint
	pingURL := fmt.Sprintf("%s/system/ping", a.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", pingURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}
	
	// Set auth headers
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to Artifactory API: %v", err)
		return fmt.Errorf("failed to connect to Artifactory API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		a.healthStatus = fmt.Sprintf("unhealthy: Artifactory API returned status code: %d", resp.StatusCode)
		return fmt.Errorf("Artifactory API returned unexpected status code: %d", resp.StatusCode)
	}
	
	// Successfully connected
	a.healthStatus = "healthy"
	return nil
}

// setAuthHeaders sets the appropriate authentication headers for the request
func (a *Adapter) setAuthHeaders(req *http.Request) {
	if a.config.Token != "" {
		// Use auth token if available
		if strings.HasPrefix(a.config.Token, "Bearer ") {
			// Bearer token format
			req.Header.Set("Authorization", a.config.Token)
		} else if strings.HasPrefix(a.config.Token, "eyJ") {
			// Looks like a JWT token - use Bearer format
			req.Header.Set("Authorization", "Bearer "+a.config.Token)
		} else {
			// Use API key header format
			req.Header.Set("X-JFrog-Art-Api", a.config.Token)
		}
	} else {
		// Use basic auth
		req.SetBasicAuth(a.config.Username, a.config.Password)
	}
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
	case "get_storage_info":
		return a.getStorageInfo(ctx, queryMap)
	case "get_folder_info":
		return a.getFolderInfo(ctx, queryMap)
	case "get_system_info":
		return a.getSystemInfo(ctx, queryMap)
	case "get_version":
		return a.getVersion(ctx, queryMap)
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
	case "get_repository_info":
		return a.getRepositoryInfo(ctx, params)
	case "get_folder_content":
		return a.getFolderContent(ctx, params)
	case "calculate_checksum":
		return a.calculateChecksum(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// IsSafeOperation checks if an operation is safe to perform
func (a *Adapter) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	// Only read operations are considered safe for Artifactory
	// (we're implementing as a read-only adapter per requirements)
	safeActions := map[string]bool{
		"download_artifact":   true,
		"get_artifact_info":   true,
		"search_artifacts":    true,
		"get_repository_info": true,
		"get_folder_content":  true,
		"calculate_checksum":  true,
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
	// For now, we'll check if using mock server, otherwise return mock data
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/storage/%s", a.config.MockURL, path)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		a.setAuthHeaders(req)
		
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

	// Real API implementation
	apiURL := fmt.Sprintf("%s/storage/%s", a.config.BaseURL, path)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set auth headers
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
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
	
	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/search/artifact?name=%s", a.config.MockURL, url.QueryEscape(query))
		if repo != "" {
			url += "&repos=" + url.QueryEscape(repo)
		}
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		a.setAuthHeaders(req)
		
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

	// Real API implementation
	// Determine which search endpoint to use based on parameters
	var apiURL string
	if query != "" {
		// Use AQL (Artifactory Query Language) for more advanced searches
		apiURL = fmt.Sprintf("%s/search/aql", a.config.BaseURL)
		
		// Build AQL query
		aqlQuery := "items.find({"
		if repo != "" {
			aqlQuery += fmt.Sprintf("\"repo\":\"%s\",", repo)
		}
		aqlQuery += fmt.Sprintf("\"name\":{\"$match\":\"%s\"}", query)
		aqlQuery += "}).include(\"name\", \"repo\", \"path\", \"created\", \"modified\", \"updated\", \"size\")"
		
		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(aqlQuery))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		req.Header.Set("Content-Type", "text/plain")
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to search artifacts: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
		}
		
		var aqlResult struct {
			Results []map[string]interface{} `json:"results"`
			Range   map[string]interface{}  `json:"range"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&aqlResult); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		
		// Transform AQL result to standard format
		return map[string]interface{}{
			"results": aqlResult.Results,
			"range": aqlResult.Range,
		}, nil
	} else {
		// Use simple artifact search endpoint for basic queries
		apiURL = fmt.Sprintf("%s/search/artifact", a.config.BaseURL)
		
		// Add query parameters
		urlParams := url.Values{}
		if repo != "" {
			urlParams.Add("repos", repo)
		}
		
		if len(urlParams) > 0 {
			apiURL += "?" + urlParams.Encode()
		}
		
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to search artifacts: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
		}
		
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		
		return result, nil
	}
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

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/build/%s/%s", a.config.MockURL, buildName, buildNumber)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		a.setAuthHeaders(req)
		
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

	// Real API implementation
	apiURL := fmt.Sprintf("%s/build/%s/%s", a.config.BaseURL, url.PathEscape(buildName), url.PathEscape(buildNumber))
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get build info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getRepositories gets a list of repositories
func (a *Adapter) getRepositories(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	type_ := ""
	if typeParam, ok := params["type"].(string); ok {
		type_ = typeParam
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/repositories", a.config.MockURL)
		if type_ != "" {
			url += "?type=" + url.QueryEscape(type_)
		}
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		a.setAuthHeaders(req)
		
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

	// Real API implementation
	apiURL := fmt.Sprintf("%s/repositories", a.config.BaseURL)
	
	// Add query parameters
	urlParams := url.Values{}
	if type_ != "" {
		urlParams.Add("type", type_)
	}
	
	if len(urlParams) > 0 {
		apiURL += "?" + urlParams.Encode()
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get repositories: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return map[string]interface{}{
		"repositories": result,
	}, nil
}

// getStorageInfo gets storage information from Artifactory
func (a *Adapter) getStorageInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Optional parameter for repository
	repoKey := ""
	if repo, ok := params["repo"].(string); ok && repo != "" {
		repoKey = "/" + repo
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/storageinfo%s", a.config.MockURL, repoKey)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get storage info from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/storageinfo%s", a.config.BaseURL, repoKey)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getFolderInfo gets information about a folder
func (a *Adapter) getFolderInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing path parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/storage/%s?list=true&deep=1&listFolders=1", a.config.MockURL, path)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get folder info from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/storage/%s?list=true&deep=1&listFolders=1", a.config.BaseURL, path)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getSystemInfo gets system information from Artifactory
func (a *Adapter) getSystemInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/system", a.config.MockURL)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get system info from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/system", a.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get system info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getVersion gets Artifactory version info
func (a *Adapter) getVersion(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/system/version", a.config.MockURL)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get version from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/system/version", a.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getRepositoryInfo gets detailed information about a repository
func (a *Adapter) getRepositoryInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	repoKey, ok := params["repo_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo_key parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/repositories/%s", a.config.MockURL, repoKey)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository info from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/repositories/%s", a.config.BaseURL, url.PathEscape(repoKey))
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getFolderContent retrieves contents of a folder in Artifactory
func (a *Adapter) getFolderContent(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing path parameter")
	}

	// Get optional parameters with defaults
	deep := "1"
	if deepParam, ok := params["deep"].(string); ok {
		deep = deepParam
	}
	
	listFolders := "1"
	if listFoldersParam, ok := params["list_folders"].(string); ok {
		listFolders = listFoldersParam
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/storage/%s?list=true&deep=%s&listFolders=%s", 
			a.config.MockURL, path, deep, listFolders)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get folder content from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/storage/%s?list=true&deep=%s&listFolders=%s", 
		a.config.BaseURL, path, deep, listFolders)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder content: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// calculateChecksum calculates checksums for an artifact
func (a *Adapter) calculateChecksum(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing path parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/storage/%s?checksum", a.config.MockURL, path)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate checksum from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/storage/%s?checksum", a.config.BaseURL, path)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// downloadArtifact downloads an artifact (returns the download URL in this implementation)
func (a *Adapter) downloadArtifact(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing path parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/artifact/%s", a.config.MockURL, path)
		
		// Just return the URL (in a real implementation, we might follow redirects and download)
		return map[string]interface{}{
			"downloadUrl": url,
			"path":        path,
		}, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/../%s", a.config.BaseURL, path)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	// Create a HEAD request to check if the file exists and get metadata
	headReq, err := http.NewRequestWithContext(ctx, "HEAD", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HEAD request: %w", err)
	}
	
	a.setAuthHeaders(headReq)
	
	headResp, err := a.client.Do(headReq)
	if err != nil {
		return nil, fmt.Errorf("failed to check artifact: %w", err)
	}
	defer headResp.Body.Close()
	
	if headResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Artifact not found, status code: %d", headResp.StatusCode)
	}
	
	// In a real implementation, we might actually download the file here
	// For this adapter, we'll just return the direct download URL
	return map[string]interface{}{
		"downloadUrl": apiURL,
		"path":        path,
		"size":        headResp.Header.Get("Content-Length"),
		"contentType": headResp.Header.Get("Content-Type"),
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

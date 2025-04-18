package xray

import (
	"bytes"
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

// Config holds configuration for the JFrog Xray adapter
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

// Adapter implements the adapter interface for JFrog Xray
type Adapter struct {
	adapters.BaseAdapter
	config       Config
	client       *http.Client
	healthStatus string
}

// NewAdapter creates a new JFrog Xray adapter
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
		config.BaseURL = "https://xray.example.com/api/v1"
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

// Initialize sets up the adapter with the JFrog Xray client
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
		return fmt.Errorf("JFrog Xray authentication credentials are required")
	}

	// Create HTTP client
	a.client = &http.Client{
		Timeout: a.config.RequestTimeout,
	}

	// Test the connection
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		// Don't return error, just make the adapter usable in degraded mode
		log.Printf("Warning: Failed to connect to JFrog Xray API: %v", err)
	} else {
		a.healthStatus = "healthy"
	}

	return nil
}

// testConnection verifies connectivity to JFrog Xray
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
			log.Println("Successfully connected to mock JFrog Xray API health endpoint")
			a.healthStatus = "healthy"
			return nil
		}
		
		// Fall back to regular mock URL
		resp, err = httpClient.Get(a.config.MockURL)
		if err != nil {
			a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to mock JFrog Xray API: %v", err)
			return fmt.Errorf("failed to connect to mock JFrog Xray API: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			a.healthStatus = fmt.Sprintf("unhealthy: mock JFrog Xray API returned status code: %d", resp.StatusCode)
			return fmt.Errorf("mock JFrog Xray API returned unexpected status code: %d", resp.StatusCode)
		}
		
		// Successfully connected to mock server
		log.Println("Successfully connected to mock JFrog Xray API")
		a.healthStatus = "healthy"
		return nil
	}
	
	// Test connectivity to real Xray API using system/ping endpoint
	pingURL := fmt.Sprintf("%s/system/ping", a.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", pingURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}
	
	// Set auth headers
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to JFrog Xray API: %v", err)
		return fmt.Errorf("failed to connect to JFrog Xray API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		a.healthStatus = fmt.Sprintf("unhealthy: JFrog Xray API returned status code: %d", resp.StatusCode)
		return fmt.Errorf("JFrog Xray API returned unexpected status code: %d", resp.StatusCode)
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

// GetData retrieves data from JFrog Xray
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
	case "get_vulnerabilities":
		return a.getVulnerabilities(ctx, queryMap)
	case "get_licenses":
		return a.getLicenses(ctx, queryMap)
	case "get_component_summary":
		return a.getComponentSummary(ctx, queryMap)
	case "get_system_info":
		return a.getSystemInfo(ctx, queryMap)
	case "get_system_version":
		return a.getSystemVersion(ctx, queryMap)
	case "get_watches":
		return a.getWatches(ctx, queryMap)
	case "get_policies":
		return a.getPolicies(ctx, queryMap)
	case "get_summary":
		return a.getSummary(ctx, queryMap)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Handle different actions
	switch action {
	case "scan_artifact":
		return a.scanArtifact(ctx, params)
	case "scan_build":
		return a.scanBuild(ctx, params)
	case "get_vulnerabilities":
		return a.getVulnerabilities(ctx, params)
	case "get_licenses":
		return a.getLicenses(ctx, params)
	case "generate_vulnerabilities_report":
		return a.generateVulnerabilitiesReport(ctx, params)
	case "get_component_details":
		return a.getComponentDetails(ctx, params)
	case "get_scan_status":
		return a.getScanStatus(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// IsSafeOperation checks if an operation is safe to perform
func (a *Adapter) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	// All JFrog Xray operations are considered safe (read-only or scan requests)
	safeActions := map[string]bool{
		"scan_artifact":                true,
		"scan_build":                   true,
		"get_vulnerabilities":          true,
		"get_licenses":                 true,
		"generate_vulnerabilities_report": true,
		"get_component_details":        true,
		"get_scan_status":              true,
	}

	// Check if the action is in the safe list
	if safe, ok := safeActions[action]; ok && safe {
		return true, nil
	}

	// Default to safe if unknown
	return true, nil
}

// getVulnerabilities gets vulnerability information for an artifact
func (a *Adapter) getVulnerabilities(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	var path string
	if pathParam, ok := params["path"].(string); ok {
		path = pathParam
	} else {
		return nil, fmt.Errorf("missing path parameter")
	}

	// Extract optional parameters
	severity := ""
	if sevParam, ok := params["severity"].(string); ok {
		severity = sevParam
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		apiURL := fmt.Sprintf("%s/vulnerabilities?path=%s", a.config.MockURL, url.QueryEscape(path))
		if severity != "" {
			apiURL += "&severity=" + url.QueryEscape(severity)
		}
		
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get vulnerabilities from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	// We need to use the /summary/artifact endpoint with a POST request
	apiURL := fmt.Sprintf("%s/summary/artifact", a.config.BaseURL)
	requestData := map[string]interface{}{
		"paths": []string{path},
	}
	
	// Add filter for severity if specified
	if severity != "" {
		requestData["severity"] = []string{severity}
	}
	
	// Marshal the request data
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get vulnerabilities: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result struct {
		Artifacts []struct {
			General struct {
				Path string `json:"path"`
			} `json:"general"`
			Issues []struct {
				IssueID      string `json:"issue_id"`
				Summary      string `json:"summary"`
				Description  string `json:"description"`
				Severity     string `json:"severity"`
				VulnerableComponents []struct {
					ComponentID  string `json:"component_id"`
					FixedVersions []string `json:"fixed_versions"`
				} `json:"vulnerable_components"`
			} `json:"issues"`
		} `json:"artifacts"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Transform the result to match the expected format
	if len(result.Artifacts) == 0 {
		return map[string]interface{}{
			"path":           path,
			"total":          0,
			"vulnerabilities": []interface{}{},
		}, nil
	}
	
	// Extract vulnerabilities
	vulnerabilities := []map[string]interface{}{}
	for _, artifact := range result.Artifacts {
		for _, issue := range artifact.Issues {
			vulnComponents := []map[string]interface{}{}
			for _, comp := range issue.VulnerableComponents {
				vulnComponents = append(vulnComponents, map[string]interface{}{
					"name":    comp.ComponentID,
					"version": extractVersionFromComponentID(comp.ComponentID),
				})
			}
			
			vulnerabilities = append(vulnerabilities, map[string]interface{}{
				"id":          issue.IssueID,
				"summary":     issue.Summary,
				"description": issue.Description,
				"severity":    issue.Severity,
				"components":  vulnComponents,
			})
		}
	}
	
	return map[string]interface{}{
		"path":           path,
		"total":          len(vulnerabilities),
		"vulnerabilities": vulnerabilities,
	}, nil
}

// extractVersionFromComponentID extracts the version from a component ID
func extractVersionFromComponentID(componentID string) string {
	parts := strings.Split(componentID, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

// getLicenses gets license information for an artifact
func (a *Adapter) getLicenses(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	var path string
	if pathParam, ok := params["path"].(string); ok {
		path = pathParam
	} else {
		return nil, fmt.Errorf("missing path parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		apiURL := fmt.Sprintf("%s/licenses?path=%s", a.config.MockURL, url.QueryEscape(path))
		
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get licenses from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	// Similar to vulnerabilities, we'll use the /summary/artifact endpoint
	apiURL := fmt.Sprintf("%s/summary/artifact", a.config.BaseURL)
	requestData := map[string]interface{}{
		"paths": []string{path},
	}
	
	// Marshal the request data
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get licenses: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result struct {
		Artifacts []struct {
			General struct {
				Path string `json:"path"`
			} `json:"general"`
			Licenses []struct {
				LicenseKey  string `json:"license_key"`
				FullName    string `json:"full_name"`
				MoreInfoURL string `json:"more_info_url"`
				Components  []struct {
					ComponentID string `json:"component_id"`
				} `json:"components"`
			} `json:"licenses"`
		} `json:"artifacts"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Transform the result to match the expected format
	if len(result.Artifacts) == 0 {
		return map[string]interface{}{
			"path":     path,
			"licenses": []interface{}{},
		}, nil
	}
	
	// Extract licenses
	licenseMap := make(map[string]map[string]interface{})
	for _, artifact := range result.Artifacts {
		for _, license := range artifact.Licenses {
			if _, exists := licenseMap[license.LicenseKey]; !exists {
				licenseMap[license.LicenseKey] = map[string]interface{}{
					"name":      license.LicenseKey,
					"full_name": license.FullName,
					"url":       license.MoreInfoURL,
					"components": []map[string]interface{}{},
				}
			}
			
			licenseEntry := licenseMap[license.LicenseKey]
			components := licenseEntry["components"].([]map[string]interface{})
			
			for _, comp := range license.Components {
				componentName := comp.ComponentID
				componentVersion := extractVersionFromComponentID(comp.ComponentID)
				
				components = append(components, map[string]interface{}{
					"name":    componentName,
					"version": componentVersion,
				})
			}
			
			licenseEntry["components"] = components
			licenseMap[license.LicenseKey] = licenseEntry
		}
	}
	
	// Convert map to array
	licenses := []map[string]interface{}{}
	for _, license := range licenseMap {
		licenses = append(licenses, license)
	}
	
	return map[string]interface{}{
		"path":     path,
		"licenses": licenses,
	}, nil
}

// getComponentSummary gets a summary of components used in an artifact
func (a *Adapter) getComponentSummary(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	var path string
	if pathParam, ok := params["path"].(string); ok {
		path = pathParam
	} else {
		return nil, fmt.Errorf("missing path parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		apiURL := fmt.Sprintf("%s/components/summary?path=%s", a.config.MockURL, url.QueryEscape(path))
		
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set auth headers
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get component summary from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	// Use the /component_summary endpoint
	apiURL := fmt.Sprintf("%s/component_summary", a.config.BaseURL)
	requestData := map[string]interface{}{
		"artifacts": []map[string]string{
			{"path": path},
		},
	}
	
	// Marshal the request data
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get component summary: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result struct {
		Components int `json:"components"`
		VulnerableComponents int `json:"vulnerable_components"`
		Vulnerabilities struct {
			High    int `json:"high"`
			Medium  int `json:"medium"`
			Low     int `json:"low"`
			Unknown int `json:"unknown"`
		} `json:"vulnerabilities"`
		Licenses map[string]int `json:"licenses"`
		TopVulnerableComponents []struct {
			ComponentID string `json:"component_id"`
			Vulnerabilities struct {
				High    int `json:"high"`
				Medium  int `json:"medium"`
				Low     int `json:"low"`
				Unknown int `json:"unknown"`
			} `json:"vulnerabilities"`
		} `json:"top_vulnerable_components"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Format the top vulnerable components
	topVulnComponents := []map[string]interface{}{}
	for _, comp := range result.TopVulnerableComponents {
		topVulnComponents = append(topVulnComponents, map[string]interface{}{
			"name":    comp.ComponentID,
			"version": extractVersionFromComponentID(comp.ComponentID),
			"vulnerabilities": map[string]interface{}{
				"high":    comp.Vulnerabilities.High,
				"medium":  comp.Vulnerabilities.Medium,
				"low":     comp.Vulnerabilities.Low,
				"unknown": comp.Vulnerabilities.Unknown,
			},
		})
	}
	
	return map[string]interface{}{
		"path": path,
		"summary": map[string]interface{}{
			"component_count": result.Components,
			"vulnerable_components": result.VulnerableComponents,
			"vulnerability_summary": map[string]interface{}{
				"high":    result.Vulnerabilities.High,
				"medium":  result.Vulnerabilities.Medium,
				"low":     result.Vulnerabilities.Low,
				"unknown": result.Vulnerabilities.Unknown,
			},
			"license_summary": result.Licenses,
		},
		"top_vulnerable_components": topVulnComponents,
	}, nil
}

// getSystemInfo gets system information from Xray
func (a *Adapter) getSystemInfo(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/system/info", a.config.MockURL)
		
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
	apiURL := fmt.Sprintf("%s/system/info", a.config.BaseURL)
	
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

// getSystemVersion gets Xray version info
func (a *Adapter) getSystemVersion(ctx context.Context, params map[string]interface{}) (interface{}, error) {
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

// getWatches gets list of watches
func (a *Adapter) getWatches(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/watches", a.config.MockURL)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get watches from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/watches", a.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get watches: %w", err)
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

// getPolicies gets list of policies
func (a *Adapter) getPolicies(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/policies", a.config.MockURL)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get policies from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/policies", a.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get policies: %w", err)
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

// getSummary gets a summary of Xray statistics and configuration
func (a *Adapter) getSummary(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/summary/common", a.config.MockURL)
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get summary from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/summary/common", a.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary: %w", err)
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

// scanArtifact initiates a security scan on an artifact
func (a *Adapter) scanArtifact(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	var path string
	if pathParam, ok := params["path"].(string); ok {
		path = pathParam
	} else {
		return nil, fmt.Errorf("missing path parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/scan", a.config.MockURL)
		
		// Create request body
		requestBody, err := json.Marshal(map[string]interface{}{
			"path": path,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		a.setAuthHeaders(req)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to scan artifact on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	// For scanning an artifact in Xray, we use the /component/scan endpoint
	apiURL := fmt.Sprintf("%s/component/scan", a.config.BaseURL)
	requestData := map[string]interface{}{
		"component_id": path,
	}
	
	// Marshal the request data
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request data: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to scan artifact: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Generate a scan ID if not returned by the API
	scanID, ok := result["scan_id"].(string)
	if !ok || scanID == "" {
		scanID = fmt.Sprintf("scan-%d", time.Now().Unix())
		result["scan_id"] = scanID
	}
	
	// Add path if not present
	if _, ok := result["path"]; !ok {
		result["path"] = path
	}
	
	// Add status if not present
	if _, ok := result["status"]; !ok {
		result["status"] = "in_progress"
	}
	
	// Add timestamps if not present
	if _, ok := result["started_at"]; !ok {
		result["started_at"] = time.Now().Format(time.RFC3339)
	}
	
	return result, nil
}

// scanBuild initiates a scan on a build
func (a *Adapter) scanBuild(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	buildName, ok := params["build_name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing build_name parameter")
	}

	buildNumber, ok := params["build_number"].(string)
	if !ok {
		return nil, fmt.Errorf("missing build_number parameter")
	}

	// Get API version
	apiVersion := a.config.APIVersion
	if versionParam, ok := params["api_version"].(string); ok {
		apiVersion = versionParam
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		scanURL := fmt.Sprintf("%s/scanBuild", a.config.MockURL)
		if apiVersion == "v2" {
			scanURL = fmt.Sprintf("%s/scanBuild/v2", a.config.MockURL)
		}
		
		// Create request body
		requestBody, err := json.Marshal(map[string]string{
			"buildName":   buildName,
			"buildNumber": buildNumber,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", scanURL, bytes.NewReader(requestBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		a.setAuthHeaders(req)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to scan build on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	scanURL := fmt.Sprintf("%s/scanBuild", a.config.BaseURL)
	if apiVersion == "v2" {
		scanURL = fmt.Sprintf("%s/scanBuild/v2", a.config.BaseURL)
	}
	
	// Create request body
	requestBody, err := json.Marshal(map[string]string{
		"buildName":   buildName,
		"buildNumber": buildNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	
	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", scanURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	a.setAuthHeaders(req)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to scan build: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// generateVulnerabilitiesReport generates a vulnerabilities report
func (a *Adapter) generateVulnerabilitiesReport(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	name, ok := params["name"].(string)
	if !ok {
		name = fmt.Sprintf("report-%d", time.Now().Unix())
	}

	// Extract resources
	resources := make(map[string]interface{})
	
	// Add repositories if provided
	if repos, ok := params["repositories"].([]interface{}); ok && len(repos) > 0 {
		reposList := make([]map[string]string, 0, len(repos))
		for _, repo := range repos {
			if repoStr, ok := repo.(string); ok {
				reposList = append(reposList, map[string]string{"name": repoStr})
			}
		}
		if len(reposList) > 0 {
			resources["repositories"] = reposList
		}
	}
	
	// Add builds if provided
	if builds, ok := params["builds"].([]interface{}); ok && len(builds) > 0 {
		buildNames := make([]string, 0, len(builds))
		for _, build := range builds {
			if buildStr, ok := build.(string); ok {
				buildNames = append(buildNames, buildStr)
			}
		}
		if len(buildNames) > 0 {
			resources["builds"] = map[string]interface{}{
				"names": buildNames,
			}
		}
	}

	// Check if resources is empty
	if len(resources) == 0 {
		return nil, fmt.Errorf("at least one repository or build must be specified")
	}

	// Extract filters if provided
	filters := make(map[string]interface{})
	if filtersMap, ok := params["filters"].(map[string]interface{}); ok {
		for k, v := range filtersMap {
			filters[k] = v
		}
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		reportURL := fmt.Sprintf("%s/reports/vulnerabilities", a.config.MockURL)
		
		// Create request body
		requestData := map[string]interface{}{
			"name":      name,
			"resources": resources,
		}
		if len(filters) > 0 {
			requestData["filters"] = filters
		}
		
		requestBody, err := json.Marshal(requestData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", reportURL, bytes.NewReader(requestBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		a.setAuthHeaders(req)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to generate report on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	reportURL := fmt.Sprintf("%s/reports/vulnerabilities", a.config.BaseURL)
	
	// Create request body
	requestData := map[string]interface{}{
		"name":      name,
		"resources": resources,
	}
	if len(filters) > 0 {
		requestData["filters"] = filters
	}
	
	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	
	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", reportURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	a.setAuthHeaders(req)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate report: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getComponentDetails gets detailed information about a component
func (a *Adapter) getComponentDetails(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	componentID, ok := params["component_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing component_id parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/component/%s", a.config.MockURL, url.PathEscape(componentID))
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get component details from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/component/%s", a.config.BaseURL, url.PathEscape(componentID))
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get component details: %w", err)
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

// getScanStatus gets the status of a scan
func (a *Adapter) getScanStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	scanID, ok := params["scan_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing scan_id parameter")
	}

	// Check if using mock server
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/scan/%s", a.config.MockURL, url.PathEscape(scanID))
		
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		a.setAuthHeaders(req)
		
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get scan status from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Real API implementation
	apiURL := fmt.Sprintf("%s/scan/%s", a.config.BaseURL, url.PathEscape(scanID))
	
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	a.setAuthHeaders(req)
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get scan status: %w", err)
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

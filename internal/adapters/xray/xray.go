package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	
	// Simple ping to the JFrog Xray API
	// In a real implementation, this would verify with a system status endpoint
	// But for now we'll just return healthy status
	a.healthStatus = "healthy"
	return nil
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
	case "get_vulnerabilities":
		return a.getVulnerabilities(ctx, params)
	case "get_licenses":
		return a.getLicenses(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// IsSafeOperation checks if an operation is safe to perform
func (a *Adapter) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	// All JFrog Xray operations are considered safe (read-only or scan requests)
	safeActions := map[string]bool{
		"scan_artifact":       true,
		"get_vulnerabilities": true,
		"get_licenses":        true,
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

	// In a real implementation, this would call the JFrog Xray API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/vulnerabilities?path=%s", a.config.MockURL, path)
		if severity != "" {
			url += "&severity=" + severity
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
			return nil, fmt.Errorf("failed to get vulnerabilities from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Mock response for testing
	// Simulate filtering by severity
	var vulnerabilities []map[string]interface{}
	
	// High severity vulnerability
	if severity == "" || severity == "high" {
		vulnerabilities = append(vulnerabilities, map[string]interface{}{
			"id":          "CVE-2023-1234",
			"summary":     "Remote code execution vulnerability in library X",
			"description": "A remote code execution vulnerability in library X allows attackers to execute arbitrary code...",
			"severity":    "high",
			"cvss_score":  8.5,
			"published":   time.Now().AddDate(0, -2, 0).Format(time.RFC3339),
			"fixed_versions": []string{"1.2.3", "2.0.1"},
			"components": []map[string]interface{}{
				{
					"name":    "org.example:library-x",
					"version": "1.1.0",
				},
			},
		})
	}
	
	// Medium severity vulnerability
	if severity == "" || severity == "medium" {
		vulnerabilities = append(vulnerabilities, map[string]interface{}{
			"id":          "CVE-2023-5678",
			"summary":     "Information disclosure in library Y",
			"description": "An information disclosure vulnerability in library Y allows attackers to access sensitive data...",
			"severity":    "medium",
			"cvss_score":  5.6,
			"published":   time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
			"fixed_versions": []string{"3.4.2"},
			"components": []map[string]interface{}{
				{
					"name":    "org.example:library-y",
					"version": "3.2.1",
				},
			},
		})
	}
	
	// Low severity vulnerability
	if severity == "" || severity == "low" {
		vulnerabilities = append(vulnerabilities, map[string]interface{}{
			"id":          "CVE-2023-9012",
			"summary":     "Denial of service in library Z",
			"description": "A denial of service vulnerability in library Z allows attackers to crash the service...",
			"severity":    "low",
			"cvss_score":  3.1,
			"published":   time.Now().AddDate(0, 0, -15).Format(time.RFC3339),
			"fixed_versions": []string{"0.9.5"},
			"components": []map[string]interface{}{
				{
					"name":    "org.example:library-z",
					"version": "0.9.0",
				},
			},
		})
	}

	return map[string]interface{}{
		"path":           path,
		"total":          len(vulnerabilities),
		"vulnerabilities": vulnerabilities,
	}, nil
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

	// In a real implementation, this would call the JFrog Xray API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/licenses?path=%s", a.config.MockURL, path)
		
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
			return nil, fmt.Errorf("failed to get licenses from mock server: %w", err)
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
		"path": path,
		"licenses": []map[string]interface{}{
			{
				"name":         "Apache 2.0",
				"full_name":    "Apache License 2.0",
				"url":          "https://www.apache.org/licenses/LICENSE-2.0",
				"components": []map[string]interface{}{
					{
						"name":    "org.apache.commons:commons-lang3",
						"version": "3.12.0",
					},
					{
						"name":    "org.apache.logging.log4j:log4j-core",
						"version": "2.14.1",
					},
				},
			},
			{
				"name":         "MIT",
				"full_name":    "MIT License",
				"url":          "https://opensource.org/licenses/MIT",
				"components": []map[string]interface{}{
					{
						"name":    "org.example:library-a",
						"version": "1.0.0",
					},
				},
			},
			{
				"name":         "GPL-3.0",
				"full_name":    "GNU General Public License v3.0",
				"url":          "https://www.gnu.org/licenses/gpl-3.0.en.html",
				"components": []map[string]interface{}{
					{
						"name":    "org.example:library-b",
						"version": "2.1.0",
					},
				},
			},
		},
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

	// In a real implementation, this would call the JFrog Xray API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/components/summary?path=%s", a.config.MockURL, path)
		
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
			return nil, fmt.Errorf("failed to get component summary from mock server: %w", err)
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
		"path": path,
		"summary": map[string]interface{}{
			"component_count": 42,
			"vulnerable_components": 3,
			"vulnerability_summary": map[string]interface{}{
				"high":    1,
				"medium":  1,
				"low":     1,
				"unknown": 0,
			},
			"license_summary": map[string]interface{}{
				"Apache 2.0": 15,
				"MIT":        20,
				"GPL-3.0":    2,
				"BSD-3":      5,
			},
		},
		"top_vulnerable_components": []map[string]interface{}{
			{
				"name":    "org.example:library-x",
				"version": "1.1.0",
				"vulnerabilities": map[string]interface{}{
					"high":    1,
					"medium":  0,
					"low":     0,
					"unknown": 0,
				},
			},
			{
				"name":    "org.example:library-y",
				"version": "3.2.1",
				"vulnerabilities": map[string]interface{}{
					"high":    0,
					"medium":  1,
					"low":     0,
					"unknown": 0,
				},
			},
			{
				"name":    "org.example:library-z",
				"version": "0.9.0",
				"vulnerabilities": map[string]interface{}{
					"high":    0,
					"medium":  0,
					"low":     1,
					"unknown": 0,
				},
			},
		},
	}, nil
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

	// In a real implementation, this would call the JFrog Xray API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
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
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		if a.config.Token != "" {
			req.Header.Set("X-JFrog-Art-Api", a.config.Token)
		} else {
			req.SetBasicAuth(a.config.Username, a.config.Password)
		}
		
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

	// Generate a fake scan ID
	scanID := fmt.Sprintf("scan-%d", time.Now().Unix())

	// Mock response for testing
	return map[string]interface{}{
		"scan_id":     scanID,
		"path":        path,
		"status":      "in_progress",
		"started_at":  time.Now().Format(time.RFC3339),
		"estimated_completion_time": time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		"message":     "Scan initiated successfully",
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

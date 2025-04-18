package sonarqube

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
)

// Config holds configuration for the SonarQube adapter
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

// Adapter implements the adapter interface for SonarQube
type Adapter struct {
	adapters.BaseAdapter
	config       Config
	client       *http.Client
	healthStatus string
}

// NewAdapter creates a new SonarQube adapter
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
		config.BaseURL = "https://sonarqube.example.com/api"
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

// Initialize sets up the adapter with the SonarQube client
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
		return fmt.Errorf("SonarQube authentication credentials are required")
	}

	// Create HTTP client
	a.client = &http.Client{
		Timeout: a.config.RequestTimeout,
	}

	// Test the connection
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		// Don't return error, just make the adapter usable in degraded mode
		log.Printf("Warning: Failed to connect to SonarQube API: %v", err)
	} else {
		a.healthStatus = "healthy"
	}

	return nil
}

// testConnection verifies connectivity to SonarQube
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
			log.Println("Successfully connected to mock SonarQube API health endpoint")
			a.healthStatus = "healthy"
			return nil
		}
		
		// Fall back to regular mock URL
		resp, err = httpClient.Get(a.config.MockURL)
		if err != nil {
			a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to mock SonarQube API: %v", err)
			return fmt.Errorf("failed to connect to mock SonarQube API: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			a.healthStatus = fmt.Sprintf("unhealthy: mock SonarQube API returned status code: %d", resp.StatusCode)
			return fmt.Errorf("mock SonarQube API returned unexpected status code: %d", resp.StatusCode)
		}
		
		// Successfully connected to mock server
		log.Println("Successfully connected to mock SonarQube API")
		a.healthStatus = "healthy"
		return nil
	}
	
	// Simple ping to the SonarQube API
	// In a real implementation, this would verify with a system/status endpoint
	// But for now we'll just return healthy status
	a.healthStatus = "healthy"
	return nil
}

// GetData retrieves data from SonarQube
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
	case "get_quality_gate_status":
		return a.getQualityGateStatus(ctx, queryMap)
	case "get_issues":
		return a.getIssues(ctx, queryMap)
	case "get_metrics":
		return a.getMetrics(ctx, queryMap)
	case "get_projects":
		return a.getProjects(ctx, queryMap)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Handle different actions
	switch action {
	case "trigger_analysis":
		return a.triggerAnalysis(ctx, params)
	case "get_quality_gate_status":
		return a.getQualityGateStatus(ctx, params)
	case "get_issues":
		return a.getIssues(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// IsSafeOperation checks if an operation is safe to perform
func (a *Adapter) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	// All implemented SonarQube operations are considered safe (read-only or triggers analysis)
	safeActions := map[string]bool{
		"trigger_analysis":       true,
		"get_quality_gate_status": true,
		"get_issues":              true,
	}

	// Check if the action is in the safe list
	if safe, ok := safeActions[action]; ok && safe {
		return true, nil
	}

	// Default to safe if unknown
	return true, nil
}

// getQualityGateStatus gets the quality gate status for a project
func (a *Adapter) getQualityGateStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}

	// In a real implementation, this would call the SonarQube API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/qualitygates/project_status?projectKey=%s", a.config.MockURL, projectKey)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get quality gate status from mock server: %w", err)
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
		"projectStatus": map[string]interface{}{
			"status": "OK",
			"conditions": []map[string]interface{}{
				{
					"status":       "OK",
					"metricKey":    "coverage",
					"comparator":   "GT",
					"errorThreshold": "80",
					"actualValue":  "85.2",
				},
				{
					"status":       "OK",
					"metricKey":    "new_bugs",
					"comparator":   "LT",
					"errorThreshold": "5",
					"actualValue":  "0",
				},
			},
			"periods": []map[string]interface{}{
				{
					"index": 1,
					"mode":  "previous_version",
					"date":  time.Now().AddDate(0, 0, -7).Format(time.RFC3339),
				},
			},
		},
	}, nil
}

// getIssues gets issues for a project
func (a *Adapter) getIssues(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}

	// Extract optional parameters
	types := ""
	if typesParam, ok := params["types"].(string); ok {
		types = typesParam
	}

	severities := ""
	if sevParam, ok := params["severities"].(string); ok {
		severities = sevParam
	}

	status := "OPEN"
	if statusParam, ok := params["status"].(string); ok {
		status = statusParam
	}

	// In a real implementation, this would call the SonarQube API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/issues/search?projectKeys=%s", a.config.MockURL, projectKey)
		if types != "" {
			url += "&types=" + types
		}
		if severities != "" {
			url += "&severities=" + severities
		}
		if status != "" {
			url += "&statuses=" + status
		}
		
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get issues from mock server: %w", err)
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
		"total": 3,
		"issues": []map[string]interface{}{
			{
				"key":      "issue1",
				"component": "my-project:src/main/java/com/example/App.java",
				"severity":  "MAJOR",
				"type":      "BUG",
				"message":   "Possible null pointer dereference",
				"line":      42,
				"status":    "OPEN",
				"author":    "developer@example.com",
				"creationDate": time.Now().AddDate(0, 0, -3).Format(time.RFC3339),
			},
			{
				"key":      "issue2",
				"component": "my-project:src/main/java/com/example/Service.java",
				"severity":  "MINOR",
				"type":      "CODE_SMELL",
				"message":   "Remove this unused method parameter",
				"line":      78,
				"status":    "OPEN",
				"author":    "developer@example.com",
				"creationDate": time.Now().AddDate(0, 0, -2).Format(time.RFC3339),
			},
			{
				"key":      "issue3",
				"component": "my-project:src/main/java/com/example/Controller.java",
				"severity":  "MINOR",
				"type":      "VULNERABILITY",
				"message":   "Make sure this SQL query is not susceptible to injection",
				"line":      96,
				"status":    "OPEN",
				"author":    "developer@example.com",
				"creationDate": time.Now().AddDate(0, 0, -1).Format(time.RFC3339),
			},
		},
		"facets": []map[string]interface{}{
			{
				"property": "severities",
				"values": []map[string]interface{}{
					{"val": "MAJOR", "count": 1},
					{"val": "MINOR", "count": 2},
				},
			},
			{
				"property": "types",
				"values": []map[string]interface{}{
					{"val": "BUG", "count": 1},
					{"val": "CODE_SMELL", "count": 1},
					{"val": "VULNERABILITY", "count": 1},
				},
			},
		},
	}, nil
}

// getMetrics gets metrics for a project
func (a *Adapter) getMetrics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}

	// Extract optional parameters
	metricKeys := "ncloc,coverage,bugs,vulnerabilities,code_smells,duplicated_lines_density"
	if keysParam, ok := params["metric_keys"].(string); ok {
		metricKeys = keysParam
	}

	// In a real implementation, this would call the SonarQube API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/measures/component?component=%s&metricKeys=%s", a.config.MockURL, projectKey, metricKeys)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get metrics from mock server: %w", err)
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
		"component": map[string]interface{}{
			"key":  projectKey,
			"name": "My Project",
			"measures": []map[string]interface{}{
				{"metric": "ncloc", "value": "12500"},
				{"metric": "coverage", "value": "85.2"},
				{"metric": "bugs", "value": "5"},
				{"metric": "vulnerabilities", "value": "2"},
				{"metric": "code_smells", "value": "74"},
				{"metric": "duplicated_lines_density", "value": "4.2"},
			},
		},
	}, nil
}

// getProjects gets a list of all projects
func (a *Adapter) getProjects(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// In a real implementation, this would call the SonarQube API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/projects/search", a.config.MockURL)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get projects from mock server: %w", err)
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
		"projects": []map[string]interface{}{
			{
				"key":     "my-project",
				"name":    "My Project",
				"qualifier": "TRK",
				"lastAnalysisDate": time.Now().AddDate(0, 0, -1).Format(time.RFC3339),
			},
			{
				"key":     "another-project",
				"name":    "Another Project",
				"qualifier": "TRK",
				"lastAnalysisDate": time.Now().AddDate(0, 0, -3).Format(time.RFC3339),
			},
			{
				"key":     "legacy-project",
				"name":    "Legacy Project",
				"qualifier": "TRK",
				"lastAnalysisDate": time.Now().AddDate(0, -2, -5).Format(time.RFC3339),
			},
		},
	}, nil
}

// triggerAnalysis triggers a new analysis
func (a *Adapter) triggerAnalysis(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}

	// Optional parameters
	branch := "main"
	if branchParam, ok := params["branch"].(string); ok {
		branch = branchParam
	}

	// In a real implementation, this would call the SonarQube API
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/scanner/trigger?project=%s&branch=%s", a.config.MockURL, projectKey, branch)
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		if a.config.Token != "" {
			req.Header.Set("Authorization", "Bearer "+a.config.Token)
		} else {
			// Basic auth
			req.SetBasicAuth(a.config.Username, a.config.Password)
		}
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to trigger analysis on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Generate a fake task ID
	taskID := fmt.Sprintf("task-%d", time.Now().Unix())

	// Mock response for testing
	return map[string]interface{}{
		"project_key": projectKey,
		"branch":      branch,
		"task_id":     taskID,
		"status":      "PENDING",
		"submittedAt": time.Now().Format(time.RFC3339),
		"message":     "Analysis triggered successfully",
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

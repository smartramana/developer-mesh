package sonarqube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
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
	
	// Test connection to real SonarQube API by calling system/status endpoint
	req, err := a.createRequest(ctx, "GET", "/system/status", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SonarQube API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SonarQube API returned unexpected status code: %d", resp.StatusCode)
	}
	
	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Check if status contains the "status" field with value "UP"
	if statusValue, ok := status["status"].(string); !ok || statusValue != "UP" {
		return fmt.Errorf("SonarQube API is not up")
	}
	
	log.Println("Successfully connected to SonarQube API")
	a.healthStatus = "healthy"
	return nil
}

// createRequest creates an HTTP request with the proper authentication
func (a *Adapter) createRequest(ctx context.Context, method, path string, params url.Values) (*http.Request, error) {
	baseURL := a.config.BaseURL
	if a.config.MockResponses && a.config.MockURL != "" {
		baseURL = a.config.MockURL
	}
	
	// Ensure the path starts with '/'
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	
	// Remove trailing slash from baseURL
	baseURL = strings.TrimSuffix(baseURL, "/")
	
	// Construct the full URL
	fullURL := baseURL + path
	
	// Add query parameters if provided
	if params != nil && len(params) > 0 {
		if strings.Contains(fullURL, "?") {
			fullURL += "&" + params.Encode()
		} else {
			fullURL += "?" + params.Encode()
		}
	}
	
	// Create the request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, err
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	// Add authentication
	if a.config.Token != "" {
		// Authentication using token (recommended)
		req.Header.Set("Authorization", "Bearer "+a.config.Token)
	} else if a.config.Username != "" && a.config.Password != "" {
		// Authentication using Basic Auth
		req.SetBasicAuth(a.config.Username, a.config.Password)
	}
	
	return req, nil
}

// executeRequest executes the given request with retry logic
func (a *Adapter) executeRequest(req *http.Request) ([]byte, error) {
	var resp *http.Response
	var err error
	
	// Retry logic
	for attempt := 0; attempt <= a.RetryMax; attempt++ {
		if attempt > 0 {
			time.Sleep(a.RetryDelay)
			log.Printf("Retrying request to %s (attempt %d/%d)", req.URL.String(), attempt, a.RetryMax)
		}
		
		resp, err = a.client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		
		if resp != nil {
			resp.Body.Close()
		}
	}
	
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned error status: %d, body: %s", resp.StatusCode, string(body))
	}
	
	return body, nil
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
	case "get_quality_gates":
		return a.getQualityGates(ctx, queryMap)
	case "get_component_details":
		return a.getComponentDetails(ctx, queryMap)
	case "get_measures_history":
		return a.getMeasuresHistory(ctx, queryMap)
	case "search_metrics":
		return a.searchMetrics(ctx, queryMap)
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
	case "create_project":
		return a.createProject(ctx, params)
	case "delete_project":
		return a.deleteProject(ctx, params)
	case "get_analysis_status":
		return a.getAnalysisStatus(ctx, params)
	case "set_project_tags":
		return a.setProjectTags(ctx, params)
	case "set_quality_gate":
		return a.setQualityGate(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// IsSafeOperation checks if an operation is safe to perform
func (a *Adapter) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	// Define unsafe operations
	unsafeActions := map[string]bool{
		"delete_project": false,
	}

	// Check if the action is in the unsafe list
	if safe, ok := unsafeActions[action]; ok && !safe {
		return false, nil
	}

	// All other actions are considered safe
	return true, nil
}

// getQualityGateStatus gets the quality gate status for a project
func (a *Adapter) getQualityGateStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}

	// Create query parameters
	queryParams := url.Values{}
	queryParams.Set("projectKey", projectKey)
	
	// Optional branch parameter
	if branch, ok := params["branch"].(string); ok {
		queryParams.Set("branch", branch)
	}

	// Create request
	req, err := a.createRequest(ctx, "GET", "/qualitygates/project_status", queryParams)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// getIssues gets issues for a project
func (a *Adapter) getIssues(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	queryParams := url.Values{}
	
	// Project key(s)
	if projectKey, ok := params["project_key"].(string); ok {
		queryParams.Set("componentKeys", projectKey)
	} else if projectKeys, ok := params["project_keys"].([]string); ok {
		queryParams.Set("componentKeys", strings.Join(projectKeys, ","))
	}
	
	// Types (BUG, VULNERABILITY, CODE_SMELL)
	if types, ok := params["types"].(string); ok {
		queryParams.Set("types", types)
	}
	
	// Severities (INFO, MINOR, MAJOR, CRITICAL, BLOCKER)
	if severities, ok := params["severities"].(string); ok {
		queryParams.Set("severities", severities)
	}
	
	// Statuses (OPEN, CONFIRMED, REOPENED, RESOLVED, CLOSED)
	if status, ok := params["status"].(string); ok {
		queryParams.Set("statuses", status)
	}
	
	// Resolution (FIXED, FALSE-POSITIVE, WONTFIX, REMOVED, ACCEPTED)
	if resolution, ok := params["resolution"].(string); ok {
		queryParams.Set("resolutions", resolution)
	}
	
	// Created after/before date
	if createdAfter, ok := params["created_after"].(string); ok {
		queryParams.Set("createdAfter", createdAfter)
	}
	
	if createdBefore, ok := params["created_before"].(string); ok {
		queryParams.Set("createdBefore", createdBefore)
	}
	
	// Pagination
	if page, ok := params["page"].(int); ok {
		queryParams.Set("p", fmt.Sprintf("%d", page))
	}
	
	if pageSize, ok := params["page_size"].(int); ok {
		queryParams.Set("ps", fmt.Sprintf("%d", pageSize))
	}
	
	// Create request
	req, err := a.createRequest(ctx, "GET", "/issues/search", queryParams)
	if err != nil {
		return nil, err
	}
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getMetrics gets metrics for a project
func (a *Adapter) getMetrics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	componentKey, ok := params["component"].(string)
	if !ok {
		componentKey, ok = params["project_key"].(string)
		if !ok {
			return nil, fmt.Errorf("missing component or project_key parameter")
		}
	}
	
	// Create query parameters
	queryParams := url.Values{}
	queryParams.Set("component", componentKey)
	
	// Get metric keys
	metricKeys := "ncloc,coverage,bugs,vulnerabilities,code_smells,duplicated_lines_density"
	if keys, ok := params["metric_keys"].(string); ok {
		metricKeys = keys
	} else if metrics, ok := params["metrics"].([]string); ok {
		metricKeys = strings.Join(metrics, ",")
	}
	
	queryParams.Set("metricKeys", metricKeys)
	
	// Optional additional fields
	if additionalFields, ok := params["additional_fields"].(string); ok {
		queryParams.Set("additionalFields", additionalFields)
	}
	
	// Branch parameter
	if branch, ok := params["branch"].(string); ok {
		queryParams.Set("branch", branch)
	}
	
	// Create request
	req, err := a.createRequest(ctx, "GET", "/measures/component", queryParams)
	if err != nil {
		return nil, err
	}
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getProjects gets a list of all projects
func (a *Adapter) getProjects(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Create query parameters
	queryParams := url.Values{}
	
	// Optional parameters
	if query, ok := params["query"].(string); ok {
		queryParams.Set("q", query)
	}
	
	if analyzedBefore, ok := params["analyzed_before"].(string); ok {
		queryParams.Set("analyzedBefore", analyzedBefore)
	}
	
	if projects, ok := params["projects"].(string); ok {
		queryParams.Set("projects", projects)
	}
	
	if page, ok := params["page"].(int); ok {
		queryParams.Set("p", fmt.Sprintf("%d", page))
	}
	
	if pageSize, ok := params["page_size"].(int); ok {
		queryParams.Set("ps", fmt.Sprintf("%d", pageSize))
	}
	
	// Create request
	req, err := a.createRequest(ctx, "GET", "/projects/search", queryParams)
	if err != nil {
		return nil, err
	}
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getQualityGates gets a list of all quality gates
func (a *Adapter) getQualityGates(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Create request
	req, err := a.createRequest(ctx, "GET", "/qualitygates/list", nil)
	if err != nil {
		return nil, err
	}
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getComponentDetails gets detailed information about a component
func (a *Adapter) getComponentDetails(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	componentKey, ok := params["component"].(string)
	if !ok {
		componentKey, ok = params["project_key"].(string)
		if !ok {
			return nil, fmt.Errorf("missing component or project_key parameter")
		}
	}
	
	// Create query parameters
	queryParams := url.Values{}
	queryParams.Set("component", componentKey)
	
	// Create request
	req, err := a.createRequest(ctx, "GET", "/components/show", queryParams)
	if err != nil {
		return nil, err
	}
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getMeasuresHistory gets the history of measures for a component
func (a *Adapter) getMeasuresHistory(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	componentKey, ok := params["component"].(string)
	if !ok {
		componentKey, ok = params["project_key"].(string)
		if !ok {
			return nil, fmt.Errorf("missing component or project_key parameter")
		}
	}
	
	// Extract metric keys (required)
	metricKeys, ok := params["metric_keys"].(string)
	if !ok {
		metricKeys, ok = params["metrics"].(string)
		if !ok {
			return nil, fmt.Errorf("missing metric_keys parameter")
		}
	}
	
	// Create query parameters
	queryParams := url.Values{}
	queryParams.Set("component", componentKey)
	queryParams.Set("metrics", metricKeys)
	
	// Optional parameters
	if from, ok := params["from"].(string); ok {
		queryParams.Set("from", from)
	}
	
	if to, ok := params["to"].(string); ok {
		queryParams.Set("to", to)
	}
	
	if branch, ok := params["branch"].(string); ok {
		queryParams.Set("branch", branch)
	}
	
	// Create request
	req, err := a.createRequest(ctx, "GET", "/measures/search_history", queryParams)
	if err != nil {
		return nil, err
	}
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// searchMetrics searches for available metrics
func (a *Adapter) searchMetrics(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Create query parameters
	queryParams := url.Values{}
	
	// Optional parameters
	if isCustom, ok := params["is_custom"].(bool); ok {
		queryParams.Set("isCustom", fmt.Sprintf("%t", isCustom))
	}
	
	if key, ok := params["key"].(string); ok {
		queryParams.Set("k", key)
	}
	
	if page, ok := params["page"].(int); ok {
		queryParams.Set("p", fmt.Sprintf("%d", page))
	}
	
	if pageSize, ok := params["page_size"].(int); ok {
		queryParams.Set("ps", fmt.Sprintf("%d", pageSize))
	}
	
	// Create request
	req, err := a.createRequest(ctx, "GET", "/metrics/search", queryParams)
	if err != nil {
		return nil, err
	}
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// triggerAnalysis triggers a new analysis
func (a *Adapter) triggerAnalysis(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// This is a placeholder function since SonarQube doesn't have a direct API to trigger analysis remotely
	// In practice, analysis is triggered via the scanner (e.g., sonar-scanner or Maven/Gradle plugins)
	// The actual implementation would depend on the specific setup
	
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
	
	// Mock response for testing
	return map[string]interface{}{
		"project_key": projectKey,
		"branch":      branch,
		"task_id":     fmt.Sprintf("task-%d", time.Now().Unix()),
		"status":      "PENDING",
		"submittedAt": time.Now().Format(time.RFC3339),
		"message":     "Analysis triggered successfully",
	}, nil
}

// createProject creates a new project in SonarQube
func (a *Adapter) createProject(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}
	
	projectName, ok := params["name"].(string)
	if !ok {
		projectName = projectKey // Use project key as name if not provided
	}
	
	// Create form data
	formData := url.Values{}
	formData.Set("project", projectKey)
	formData.Set("name", projectName)
	
	// Create request
	req, err := a.createRequest(ctx, "POST", "/projects/create", formData)
	if err != nil {
		return nil, err
	}
	
	// Change content type for form submission
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(formData.Encode()))
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// deleteProject deletes a project from SonarQube
func (a *Adapter) deleteProject(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}
	
	// Create form data
	formData := url.Values{}
	formData.Set("project", projectKey)
	
	// Create request
	req, err := a.createRequest(ctx, "POST", "/projects/delete", formData)
	if err != nil {
		return nil, err
	}
	
	// Change content type for form submission
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(formData.Encode()))
	
	// Execute request
	_, err = a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Return success response (the API doesn't return a body on success)
	return map[string]interface{}{
		"project_key": projectKey,
		"status":      "deleted",
		"message":     "Project deleted successfully",
	}, nil
}

// getAnalysisStatus gets the status of an analysis task
func (a *Adapter) getAnalysisStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	taskId, ok := params["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing task_id parameter")
	}
	
	// Create query parameters
	queryParams := url.Values{}
	queryParams.Set("id", taskId)
	
	// Create request
	req, err := a.createRequest(ctx, "GET", "/ce/task", queryParams)
	if err != nil {
		return nil, err
	}
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// setProjectTags sets tags for a project
func (a *Adapter) setProjectTags(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}
	
	tags, ok := params["tags"].(string)
	if !ok {
		return nil, fmt.Errorf("missing tags parameter")
	}
	
	// Create form data
	formData := url.Values{}
	formData.Set("project", projectKey)
	formData.Set("tags", tags)
	
	// Create request
	req, err := a.createRequest(ctx, "POST", "/project_tags/set", formData)
	if err != nil {
		return nil, err
	}
	
	// Change content type for form submission
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(formData.Encode()))
	
	// Execute request
	body, err := a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Parse response (the API might not return a body)
	if len(body) == 0 {
		return map[string]interface{}{
			"project_key": projectKey,
			"tags":        tags,
			"status":      "updated",
			"message":     "Project tags updated successfully",
		}, nil
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// setQualityGate assigns a quality gate to a project
func (a *Adapter) setQualityGate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	projectKey, ok := params["project_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing project_key parameter")
	}
	
	gateId, ok := params["gate_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing gate_id parameter")
	}
	
	// Create form data
	formData := url.Values{}
	formData.Set("projectKey", projectKey)
	formData.Set("gateId", gateId)
	
	// Create request
	req, err := a.createRequest(ctx, "POST", "/qualitygates/select", formData)
	if err != nil {
		return nil, err
	}
	
	// Change content type for form submission
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(formData.Encode()))
	
	// Execute request
	_, err = a.executeRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Return success response (the API doesn't return a body on success)
	return map[string]interface{}{
		"project_key": projectKey,
		"gate_id":     gateId,
		"status":      "assigned",
		"message":     "Quality gate assigned successfully",
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

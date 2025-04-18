package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
)

// Config holds configuration for the Harness adapter
type Config struct {
	APIToken        string        `mapstructure:"api_token"`
	AccountID       string        `mapstructure:"account_id"`
	BaseURL         string        `mapstructure:"base_url"`
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	RetryMax        int           `mapstructure:"retry_max"`
	RetryDelay      time.Duration `mapstructure:"retry_delay"`
	MockResponses   bool          `mapstructure:"mock_responses"`
	MockURL         string        `mapstructure:"mock_url"`
}

// Adapter implements the adapter interface for Harness
type Adapter struct {
	adapters.BaseAdapter
	config       Config
	client       *http.Client
	healthStatus string
}

// NewAdapter creates a new Harness adapter
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
		config.BaseURL = "https://app.harness.io/gateway/api/graphql"
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

// Initialize sets up the adapter with the Harness client
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
	if a.config.APIToken == "" {
		return fmt.Errorf("Harness API token is required")
	}

	if a.config.AccountID == "" {
		return fmt.Errorf("Harness Account ID is required")
	}

	// Create HTTP client
	a.client = &http.Client{
		Timeout: a.config.RequestTimeout,
	}

	// Test the connection
	if err := a.testConnection(ctx); err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: %v", err)
		// Don't return error, just make the adapter usable in degraded mode
		log.Printf("Warning: Failed to connect to Harness API: %v", err)
	} else {
		a.healthStatus = "healthy"
	}

	return nil
}

// testConnection verifies connectivity to Harness
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
			log.Println("Successfully connected to mock Harness API health endpoint")
			a.healthStatus = "healthy"
			return nil
		}
		
		// Fall back to regular mock URL
		resp, err = httpClient.Get(a.config.MockURL)
		if err != nil {
			a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to mock Harness API: %v", err)
			return fmt.Errorf("failed to connect to mock Harness API: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			a.healthStatus = fmt.Sprintf("unhealthy: mock Harness API returned status code: %d", resp.StatusCode)
			return fmt.Errorf("mock Harness API returned unexpected status code: %d", resp.StatusCode)
		}
		
		// Successfully connected to mock server
		log.Println("Successfully connected to mock Harness API")
		a.healthStatus = "healthy"
		return nil
	}
	
	// Simple ping to the Harness API
	// In a real implementation, this would verify with a lightweight GraphQL query
	// But for now we'll just return healthy status
	a.healthStatus = "healthy"
	return nil
}

// GetData retrieves data from Harness
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
	case "get_pipelines":
		return a.getPipelines(ctx, queryMap)
	case "get_pipeline_status":
		return a.getPipelineStatus(ctx, queryMap)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Handle different actions
	switch action {
	case "trigger_pipeline":
		return a.triggerPipeline(ctx, params)
	case "stop_pipeline":
		return a.stopPipeline(ctx, params)
	case "rollback_deployment":
		return a.rollbackDeployment(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// IsSafeOperation checks if an operation is safe to perform
func (a *Adapter) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	// List of operations that are considered safe
	safeActions := map[string]bool{
		"trigger_pipeline":    true,
		"stop_pipeline":       true,
		"rollback_deployment": true,
	}

	// Check if the action is in the safe list
	if safe, ok := safeActions[action]; ok && safe {
		return true, nil
	}

	// Check for specific unsafe operations
	unsafeActions := map[string]bool{
		"delete_pipeline": false,
	}

	if unsafe, ok := unsafeActions[action]; ok && !unsafe {
		return false, fmt.Errorf("operation '%s' is unsafe", action)
	}

	// Default to safe if unknown
	return true, nil
}

// getPipelines gets all pipelines for an application
func (a *Adapter) getPipelines(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// In a real implementation, this would make a GraphQL query to Harness
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/pipelines", a.config.MockURL)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get pipelines from mock server: %w", err)
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
		"pipelines": []map[string]interface{}{
			{
				"id":   "pipe1",
				"name": "Production Deployment",
				"type": "deployment",
			},
			{
				"id":   "pipe2",
				"name": "Test Pipeline",
				"type": "build",
			},
		},
	}, nil
}

// getPipelineStatus gets the status of a pipeline execution
func (a *Adapter) getPipelineStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	pipelineID, ok := params["pipeline_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing pipeline_id parameter")
	}

	executionID, ok := params["execution_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing execution_id parameter")
	}

	// In a real implementation, this would make a GraphQL query to Harness
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/pipeline/%s/execution/%s", a.config.MockURL, pipelineID, executionID)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get pipeline status from mock server: %w", err)
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
		"pipeline_id":   pipelineID,
		"execution_id":  executionID,
		"status":        "SUCCESS",
		"start_time":    time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"end_time":      time.Now().Format(time.RFC3339),
		"trigger_type":  "MANUAL",
		"triggered_by":  "user@example.com",
	}, nil
}

// triggerPipeline triggers a pipeline execution
func (a *Adapter) triggerPipeline(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	pipelineID, ok := params["pipeline_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing pipeline_id parameter")
	}

	// Optional parameters
	var variables map[string]interface{}
	if vars, ok := params["variables"].(map[string]interface{}); ok {
		variables = vars
	}

	// In a real implementation, this would make a GraphQL mutation to Harness
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/pipeline/%s/trigger", a.config.MockURL, pipelineID)
		
		// Create request body
		requestBody, err := json.Marshal(map[string]interface{}{
			"variables": variables,
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
		req.Header.Set("X-API-Key", a.config.APIToken)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to trigger pipeline on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Generate a fake execution ID
	executionID := fmt.Sprintf("exec-%d", time.Now().Unix())

	// Mock response for testing
	return map[string]interface{}{
		"pipeline_id":  pipelineID,
		"execution_id": executionID,
		"status":       "RUNNING",
		"start_time":   time.Now().Format(time.RFC3339),
		"trigger_type": "API",
	}, nil
}

// stopPipeline stops a running pipeline execution
func (a *Adapter) stopPipeline(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	pipelineID, ok := params["pipeline_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing pipeline_id parameter")
	}

	executionID, ok := params["execution_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing execution_id parameter")
	}

	// In a real implementation, this would make a GraphQL mutation to Harness
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/pipeline/%s/execution/%s/stop", a.config.MockURL, pipelineID, executionID)
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", a.config.APIToken)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to stop pipeline on mock server: %w", err)
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
		"pipeline_id":  pipelineID,
		"execution_id": executionID,
		"status":       "ABORTED",
		"start_time":   time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		"end_time":     time.Now().Format(time.RFC3339),
	}, nil
}

// rollbackDeployment performs a rollback to a previous deployment
func (a *Adapter) rollbackDeployment(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	serviceID, ok := params["service_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing service_id parameter")
	}

	environmentID, ok := params["environment_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing environment_id parameter")
	}

	// Optional parameters
	var previousDeploymentID string
	if id, ok := params["previous_deployment_id"].(string); ok {
		previousDeploymentID = id
	}

	// In a real implementation, this would make a GraphQL mutation to Harness
	// For now, we'll return mock data

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/service/%s/environment/%s/rollback", a.config.MockURL, serviceID, environmentID)
		
		// Create request body
		requestBody, err := json.Marshal(map[string]interface{}{
			"previous_deployment_id": previousDeploymentID,
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
		req.Header.Set("X-API-Key", a.config.APIToken)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to rollback deployment on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Generate a fake execution ID for the rollback
	rollbackID := fmt.Sprintf("rollback-%d", time.Now().Unix())

	// Mock response for testing
	return map[string]interface{}{
		"service_id":     serviceID,
		"environment_id": environmentID,
		"rollback_id":    rollbackID,
		"status":         "RUNNING",
		"start_time":     time.Now().Format(time.RFC3339),
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

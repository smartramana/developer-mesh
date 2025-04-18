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
	APIToken          string        `mapstructure:"api_token"`
	AccountID         string        `mapstructure:"account_id"`
	OrgIdentifier     string        `mapstructure:"org_identifier"`
	ProjectIdentifier string        `mapstructure:"project_identifier"`
	BaseURL           string        `mapstructure:"base_url"`
	APIURL            string        `mapstructure:"api_url"`
	GraphQLURL        string        `mapstructure:"graphql_url"`
	CCMAPIURL         string        `mapstructure:"ccm_api_url"`
	CCMGraphQLURL     string        `mapstructure:"ccm_graphql_url"`
	RequestTimeout    time.Duration `mapstructure:"request_timeout"`
	RetryMax          int           `mapstructure:"retry_max"`
	RetryDelay        time.Duration `mapstructure:"retry_delay"`
	MockResponses     bool          `mapstructure:"mock_responses"`
	MockURL           string        `mapstructure:"mock_url"`
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
	
	// Set default base URL if not provided
	if config.BaseURL == "" {
		config.BaseURL = "https://app.harness.io"
	}
	
	// Set derived URLs if not explicitly provided
	if config.APIURL == "" {
		config.APIURL = fmt.Sprintf("%s/ng/api", config.BaseURL)
	}
	
	if config.GraphQLURL == "" {
		config.GraphQLURL = fmt.Sprintf("%s/gateway/api/graphql", config.BaseURL)
	}
	
	// Set derived CCM URLs if not explicitly provided
	if config.CCMAPIURL == "" {
		config.CCMAPIURL = fmt.Sprintf("%s/ccm/api", config.BaseURL)
	}
	
	if config.CCMGraphQLURL == "" {
		config.CCMGraphQLURL = fmt.Sprintf("%s/ccm/graphql", config.BaseURL)
	}
	
	// Set default organization identifier if not provided
	if config.OrgIdentifier == "" {
		config.OrgIdentifier = "default"
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
	
	// Test connection to the real Harness API
	// Try calling a simple API endpoint to verify connectivity
	testURL := fmt.Sprintf("%s/ping?accountIdentifier=%s", a.config.APIURL, a.config.AccountID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		a.healthStatus = fmt.Sprintf("unhealthy: failed to connect to Harness API: %v", err)
		return fmt.Errorf("failed to connect to Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		a.healthStatus = fmt.Sprintf("unhealthy: Harness API returned status code: %d", resp.StatusCode)
		return fmt.Errorf("Harness API returned unexpected status code: %d", resp.StatusCode)
	}
	
	// Successfully connected to Harness API
	log.Println("Successfully connected to Harness API")
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
	case "get_feature_flags":
		return a.getFeatureFlags(ctx, queryMap)
	case "get_ccm_costs":
		return a.getCCMCosts(ctx, queryMap)
	case "get_ccm_recommendations":
		return a.getCCMRecommendations(ctx, queryMap)
	case "get_ccm_budgets":
		return a.getCCMBudgets(ctx, queryMap)
	case "get_ccm_anomalies":
		return a.getCCMAnomalies(ctx, queryMap)
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
	case "toggle_feature_flag":
		return a.toggleFeatureFlag(ctx, params)
	case "apply_ccm_recommendation":
		return a.applyCCMRecommendation(ctx, params)
	case "ignore_ccm_recommendation":
		return a.ignoreCCMRecommendation(ctx, params)
	case "acknowledge_ccm_anomaly":
		return a.acknowledgeCCMAnomaly(ctx, params)
	case "ignore_ccm_anomaly":
		return a.ignoreCCMAnomaly(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// applyCCMRecommendation applies a cost optimization recommendation
func (a *Adapter) applyCCMRecommendation(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	recommendationID, ok := params["recommendation_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing recommendation_id parameter")
	}
	
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for applying CCM recommendations")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/ccm/recommendations/%s/apply", a.config.MockURL, recommendationID)
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.config.APIToken)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to apply recommendation on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Make real API call to Harness
	url := fmt.Sprintf("%s/recommendations/%s/apply?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
		a.config.CCMAPIURL, recommendationID, a.config.AccountID, orgIdentifier, projectIdentifier)
	
	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to apply recommendation on Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// ignoreCCMRecommendation ignores a cost optimization recommendation
func (a *Adapter) ignoreCCMRecommendation(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	recommendationID, ok := params["recommendation_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing recommendation_id parameter")
	}
	
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for ignoring CCM recommendations")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// Extract reason if provided
	reason := ""
	if r, ok := params["reason"].(string); ok {
		reason = r
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/ccm/recommendations/%s/ignore", a.config.MockURL, recommendationID)
		
		// Create request body if reason is provided
		var requestBody []byte
		var err error
		if reason != "" {
			requestBody, err = json.Marshal(map[string]interface{}{
				"reason": reason,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
		}
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.config.APIToken)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to ignore recommendation on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Create request body if reason is provided
	var requestBody []byte
	var err error
	if reason != "" {
		requestBody, err = json.Marshal(map[string]interface{}{
			"reason": reason,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}
	
	// Make real API call to Harness
	url := fmt.Sprintf("%s/recommendations/%s/ignore?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
		a.config.CCMAPIURL, recommendationID, a.config.AccountID, orgIdentifier, projectIdentifier)
	
	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to ignore recommendation on Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// acknowledgeCCMAnomaly acknowledges a cost anomaly
func (a *Adapter) acknowledgeCCMAnomaly(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	anomalyID, ok := params["anomaly_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing anomaly_id parameter")
	}
	
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for acknowledging CCM anomalies")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/ccm/anomalies/%s/acknowledge", a.config.MockURL, anomalyID)
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.config.APIToken)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to acknowledge anomaly on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Make real API call to Harness
	url := fmt.Sprintf("%s/anomalies/%s/acknowledge?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
		a.config.CCMAPIURL, anomalyID, a.config.AccountID, orgIdentifier, projectIdentifier)
	
	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to acknowledge anomaly on Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// ignoreCCMAnomaly ignores a cost anomaly
func (a *Adapter) ignoreCCMAnomaly(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	anomalyID, ok := params["anomaly_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing anomaly_id parameter")
	}
	
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for ignoring CCM anomalies")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// Extract reason if provided
	reason := ""
	if r, ok := params["reason"].(string); ok {
		reason = r
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/ccm/anomalies/%s/ignore", a.config.MockURL, anomalyID)
		
		// Create request body if reason is provided
		var requestBody []byte
		var err error
		if reason != "" {
			requestBody, err = json.Marshal(map[string]interface{}{
				"reason": reason,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
		}
		
		// Create POST request
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.config.APIToken)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to ignore anomaly on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Create request body if reason is provided
	var requestBody []byte
	var err error
	if reason != "" {
		requestBody, err = json.Marshal(map[string]interface{}{
			"reason": reason,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}
	
	// Make real API call to Harness
	url := fmt.Sprintf("%s/anomalies/%s/ignore?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
		a.config.CCMAPIURL, anomalyID, a.config.AccountID, orgIdentifier, projectIdentifier)
	
	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to ignore anomaly on Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// toggleFeatureFlag toggles a feature flag on or off
func (a *Adapter) toggleFeatureFlag(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	flagIdentifier, ok := params["flag_identifier"].(string)
	if !ok {
		return nil, fmt.Errorf("missing flag_identifier parameter")
	}
	
	enabled, ok := params["enabled"].(bool)
	if !ok {
		return nil, fmt.Errorf("missing enabled parameter")
	}
	
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for toggling feature flags")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	environmentIdentifier, ok := params["environment_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing environment_id parameter")
	}

	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/feature-flags/%s/toggle", a.config.MockURL, flagIdentifier)
		
		// Create request body
		requestBody, err := json.Marshal(map[string]interface{}{
			"enabled": enabled,
			"environment_id": environmentIdentifier,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		
		// Create POST request with body
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.config.APIToken)
		
		// Send request
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to toggle feature flag on mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Create request body
	requestBody := map[string]interface{}{
		"state": enabled,
	}
	
	// Convert request body to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	
	// Make real API call to Harness
	url := fmt.Sprintf("%s/ff/api/admin/v1/feature-flags/%s/state?accountId=%s&orgIdentifier=%s&projectIdentifier=%s&environmentIdentifier=%s", 
		a.config.BaseURL, flagIdentifier, a.config.AccountID, orgIdentifier, projectIdentifier, environmentIdentifier)
	
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to toggle feature flag on Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// IsSafeOperation checks if an operation is safe to perform
func (a *Adapter) IsSafeOperation(action string, params map[string]interface{}) (bool, error) {
	// List of operations that are considered safe
	safeActions := map[string]bool{
		"trigger_pipeline":            true,
		"stop_pipeline":               true,
		"rollback_deployment":         true,
		"toggle_feature_flag":         true,
		"apply_ccm_recommendation":    true,
		"ignore_ccm_recommendation":   true,
		"create_ccm_budget":           true,
		"update_ccm_budget":           true,
		"ignore_ccm_anomaly":          true,
		"acknowledge_ccm_anomaly":     true,
	}

	// Check if the action is in the safe list
	if safe, ok := safeActions[action]; ok && safe {
		return true, nil
	}

	// Check for specific unsafe operations
	unsafeActions := map[string]bool{
		"delete_pipeline":    false,
		"delete_ccm_budget": false,
	}

	if unsafe, ok := unsafeActions[action]; ok && !unsafe {
		return false, fmt.Errorf("operation '%s' is unsafe", action)
	}

	// Default to safe if unknown
	return true, nil
}

// getPipelines gets all pipelines for a project
func (a *Adapter) getPipelines(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
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

	// Make a real API call to Harness
	var url string
	if projectIdentifier != "" {
		// If project ID is provided, get pipelines for that project
		url = fmt.Sprintf("%s/pipeline/api/pipelines/v2?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
			a.config.BaseURL, a.config.AccountID, orgIdentifier, projectIdentifier)
	} else {
		// Otherwise get all pipelines for the account
		url = fmt.Sprintf("%s/pipeline/api/pipelines/v2?accountIdentifier=%s", 
			a.config.BaseURL, a.config.AccountID)
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipelines from Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
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
	
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}

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

	// Make a real API call to Harness
	url := fmt.Sprintf("%s/pipeline/api/pipelines/execution/%s?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s&pipelineIdentifier=%s", 
		a.config.BaseURL, executionID, a.config.AccountID, orgIdentifier, projectIdentifier, pipelineID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline status from Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// triggerPipeline triggers a pipeline execution
func (a *Adapter) triggerPipeline(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	pipelineID, ok := params["pipeline_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing pipeline_id parameter")
	}

	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// Optional variables for pipeline execution
	var variables map[string]interface{}
	if vars, ok := params["variables"].(map[string]interface{}); ok {
		variables = vars
	}

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
		
		// Create POST request with body
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.config.APIToken)
		
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

	// Create request body with execution inputs
	requestBody := map[string]interface{}{
		"pipelineIdentifier": pipelineID,
		"triggerType": "API",
	}
	
	// Add variables if provided
	if variables != nil && len(variables) > 0 {
		requestBody["variables"] = variables
	}
	
	// Convert request body to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	
	// Make real API call to Harness
	url := fmt.Sprintf("%s/pipeline/api/pipeline/execute?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
		a.config.BaseURL, a.config.AccountID, orgIdentifier, projectIdentifier)
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger pipeline on Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
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
	
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}

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
		req.Header.Set("x-api-key", a.config.APIToken)
		
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

	// Make real API call to Harness
	url := fmt.Sprintf("%s/pipeline/api/pipeline/abort?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s&executionId=%s", 
		a.config.BaseURL, a.config.AccountID, orgIdentifier, projectIdentifier, executionID)
	
	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to stop pipeline on Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
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
	
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}

	// Optional parameters
	var previousDeploymentID string
	if id, ok := params["previous_deployment_id"].(string); ok {
		previousDeploymentID = id
	}

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
		req.Header.Set("x-api-key", a.config.APIToken)
		
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

	// Create request body
	requestBody := map[string]interface{}{
		"serviceId": serviceID,
		"environmentId": environmentID,
	}
	
	// Add previous deployment ID if provided
	if previousDeploymentID != "" {
		requestBody["deploymentId"] = previousDeploymentID
	}
	
	// Convert request body to JSON
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	
	// Make real API call to Harness
	url := fmt.Sprintf("%s/ng/api/services/rollback?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
		a.config.BaseURL, a.config.AccountID, orgIdentifier, projectIdentifier)
	
	// Create POST request with body
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to rollback deployment on Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
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

// getCCMCosts gets cloud cost data from CCM
func (a *Adapter) getCCMCosts(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for getting CCM cost data")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// Get query parameters
	startTime := time.Now().Add(-30 * 24 * time.Hour) // Default to last 30 days
	if st, ok := params["start_time"].(string); ok && st != "" {
		parsedTime, err := time.Parse(time.RFC3339, st)
		if err == nil {
			startTime = parsedTime
		}
	}
	
	endTime := time.Now()
	if et, ok := params["end_time"].(string); ok && et != "" {
		parsedTime, err := time.Parse(time.RFC3339, et)
		if err == nil {
			endTime = parsedTime
		}
	}
	
	// Optional groupBy parameter
	var groupBy []string
	if gb, ok := params["group_by"].([]interface{}); ok {
		for _, g := range gb {
			if groupStr, ok := g.(string); ok {
				groupBy = append(groupBy, groupStr)
			}
		}
	}
	
	// Optional filterBy parameter
	var filterBy []string
	if fb, ok := params["filter_by"].([]interface{}); ok {
		for _, f := range fb {
			if filterStr, ok := f.(string); ok {
				filterBy = append(filterBy, filterStr)
			}
		}
	}
	
	// Optional cloud provider parameter
	cloudProvider := ""
	if cp, ok := params["cloud_provider"].(string); ok {
		cloudProvider = cp
	}
	
	// Optional perspective ID
	perspectiveID := ""
	if pid, ok := params["perspective_id"].(string); ok {
		perspectiveID = pid
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/ccm/costs", a.config.MockURL)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get CCM costs from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Build GraphQL query for CCM costs
	// This is using the GraphQL API for CCM
	graphqlQuery := map[string]interface{}{
		"query": `
			query GetCCMCosts($accountId: String!, $orgIdentifier: String!, $projectIdentifier: String!, $startTime: String!, $endTime: String!, $groupBy: [String!], $filterBy: [String!], $cloudProvider: String, $perspectiveId: String) {
				getCCMCosts(accountId: $accountId, orgIdentifier: $orgIdentifier, projectIdentifier: $projectIdentifier, startTime: $startTime, endTime: $endTime, groupBy: $groupBy, filterBy: $filterBy, cloudProvider: $cloudProvider, perspectiveId: $perspectiveId) {
					totalCost
					currency
					timeGrain
					startTime
					endTime
					costBreakdown {
						name
						cost
						percentage
					}
				}
			}
		`,
		"variables": map[string]interface{}{
			"accountId":         a.config.AccountID,
			"orgIdentifier":     orgIdentifier,
			"projectIdentifier": projectIdentifier,
			"startTime":         startTime.Format(time.RFC3339),
			"endTime":           endTime.Format(time.RFC3339),
			"groupBy":           groupBy,
			"filterBy":          filterBy,
			"cloudProvider":     cloudProvider,
			"perspectiveId":     perspectiveID,
		},
	}
	
	// Convert graphQL query to JSON
	jsonBody, err := json.Marshal(graphqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL query: %w", err)
	}
	
	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", a.config.GraphQLURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get CCM costs from Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Extract data from GraphQL response
	if data, ok := result["data"].(map[string]interface{}); ok {
		if costsData, ok := data["getCCMCosts"].(map[string]interface{}); ok {
			return costsData, nil
		}
	}
	
	// Check for errors in the GraphQL response
	if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
		errorMsg := "GraphQL errors:"
		for _, e := range errors {
			if errObj, ok := e.(map[string]interface{}); ok {
				if msg, ok := errObj["message"].(string); ok {
					errorMsg += " " + msg
				}
			}
		}
		return nil, fmt.Errorf(errorMsg)
	}
	
	return nil, fmt.Errorf("unexpected response format from Harness API")
}

// getFeatureFlags gets all feature flags for a project
func (a *Adapter) getFeatureFlags(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for getting feature flags")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// Optional filter parameters
	var environmentID string
	if envID, ok := params["environment_id"].(string); ok {
		environmentID = envID
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/feature-flags", a.config.MockURL)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get feature flags from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// Make a real API call to Harness
	url := fmt.Sprintf("%s/ff/api/client/v1/feature-flags?accountId=%s&orgIdentifier=%s&projectIdentifier=%s", 
		a.config.BaseURL, a.config.AccountID, orgIdentifier, projectIdentifier)
	
	// Add environment filter if provided
	if environmentID != "" {
		url = fmt.Sprintf("%s&environmentIdentifier=%s", url, environmentID)
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get feature flags from Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getCCMRecommendations gets cost optimization recommendations from CCM
func (a *Adapter) getCCMRecommendations(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for getting CCM recommendations")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// Optional status parameter
	status := "OPEN" // Default to open recommendations
	if s, ok := params["status"].(string); ok && s != "" {
		status = s
	}
	
	// Optional cloud provider parameter
	cloudProvider := ""
	if cp, ok := params["cloud_provider"].(string); ok {
		cloudProvider = cp
	}
	
	// Optional recommendation type
	recommendationType := ""
	if rt, ok := params["recommendation_type"].(string); ok {
		recommendationType = rt
	}
	
	// Optional resource ID
	resourceID := ""
	if rid, ok := params["resource_id"].(string); ok {
		resourceID = rid
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/ccm/recommendations", a.config.MockURL)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get CCM recommendations from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// For CCM recommendations, we'll use the REST API endpoint
	url := fmt.Sprintf("%s/ccm/api/recommendations?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
		a.config.BaseURL, a.config.AccountID, orgIdentifier, projectIdentifier)
	
	// Add optional filters
	if status != "" {
		url = fmt.Sprintf("%s&status=%s", url, status)
	}
	
	if cloudProvider != "" {
		url = fmt.Sprintf("%s&cloudProvider=%s", url, cloudProvider)
	}
	
	if recommendationType != "" {
		url = fmt.Sprintf("%s&type=%s", url, recommendationType)
	}
	
	if resourceID != "" {
		url = fmt.Sprintf("%s&resourceId=%s", url, resourceID)
	}
	
	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get CCM recommendations from Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getCCMBudgets gets budget information from CCM
func (a *Adapter) getCCMBudgets(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for getting CCM budgets")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// Optional budget ID
	budgetID := ""
	if bid, ok := params["budget_id"].(string); ok {
		budgetID = bid
	}
	
	// Optional status parameter
	status := "" // Default to all budgets
	if s, ok := params["status"].(string); ok {
		status = s
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/ccm/budgets", a.config.MockURL)
		if budgetID != "" {
			url = fmt.Sprintf("%s/%s", url, budgetID)
		}
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get CCM budgets from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// For CCM budgets, we'll use the REST API endpoint
	var url string
	if budgetID != "" {
		url = fmt.Sprintf("%s/ccm/api/budgets/%s?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
			a.config.BaseURL, budgetID, a.config.AccountID, orgIdentifier, projectIdentifier)
	} else {
		url = fmt.Sprintf("%s/ccm/api/budgets?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s", 
			a.config.BaseURL, a.config.AccountID, orgIdentifier, projectIdentifier)
		
		// Add optional status filter
		if status != "" {
			url = fmt.Sprintf("%s&status=%s", url, status)
		}
	}
	
	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get CCM budgets from Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
}

// getCCMAnomalies gets cost anomalies from CCM
func (a *Adapter) getCCMAnomalies(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract optional parameters
	projectIdentifier := a.config.ProjectIdentifier
	if projID, ok := params["project_id"].(string); ok && projID != "" {
		projectIdentifier = projID
	}
	
	if projectIdentifier == "" {
		return nil, fmt.Errorf("project_id is required for getting CCM anomalies")
	}
	
	orgIdentifier := a.config.OrgIdentifier
	if orgID, ok := params["org_id"].(string); ok && orgID != "" {
		orgIdentifier = orgID
	}
	
	// Get query parameters
	startTime := time.Now().Add(-30 * 24 * time.Hour) // Default to last 30 days
	if st, ok := params["start_time"].(string); ok && st != "" {
		parsedTime, err := time.Parse(time.RFC3339, st)
		if err == nil {
			startTime = parsedTime
		}
	}
	
	endTime := time.Now()
	if et, ok := params["end_time"].(string); ok && et != "" {
		parsedTime, err := time.Parse(time.RFC3339, et)
		if err == nil {
			endTime = parsedTime
		}
	}
	
	// Optional status parameter
	status := "" // Default to all anomalies
	if s, ok := params["status"].(string); ok {
		status = s
	}
	
	// Optional cloud provider parameter
	cloudProvider := ""
	if cp, ok := params["cloud_provider"].(string); ok {
		cloudProvider = cp
	}
	
	// If using mock server and mock responses are enabled
	if a.config.MockResponses && a.config.MockURL != "" {
		url := fmt.Sprintf("%s/ccm/anomalies", a.config.MockURL)
		resp, err := a.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get CCM anomalies from mock server: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return result, nil
	}

	// For CCM anomalies, we'll use the REST API endpoint
	url := fmt.Sprintf("%s/ccm/api/anomalies?accountIdentifier=%s&orgIdentifier=%s&projectIdentifier=%s&startTime=%s&endTime=%s", 
		a.config.BaseURL, a.config.AccountID, orgIdentifier, projectIdentifier, 
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
	
	// Add optional filters
	if status != "" {
		url = fmt.Sprintf("%s&status=%s", url, status)
	}
	
	if cloudProvider != "" {
		url = fmt.Sprintf("%s&cloudProvider=%s", url, cloudProvider)
	}
	
	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIToken)
	
	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get CCM anomalies from Harness API: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Harness API returned status code: %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return result, nil
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

package harness

import (
	"context"
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

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	return a.healthStatus
}

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing specific to clean up
	return nil
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

// For all adapter methods below, implement stub versions that return "not implemented" error
// This is to fulfill the interface without implementing the full functionality initially

// applyCCMRecommendation applies a cost optimization recommendation
func (a *Adapter) applyCCMRecommendation(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// ignoreCCMRecommendation ignores a cost optimization recommendation
func (a *Adapter) ignoreCCMRecommendation(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// acknowledgeCCMAnomaly acknowledges a cost anomaly
func (a *Adapter) acknowledgeCCMAnomaly(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// ignoreCCMAnomaly ignores a cost anomaly
func (a *Adapter) ignoreCCMAnomaly(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// toggleFeatureFlag toggles a feature flag on or off
func (a *Adapter) toggleFeatureFlag(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// getPipelines gets all pipelines for a project
func (a *Adapter) getPipelines(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// getPipelineStatus gets the status of a pipeline execution
func (a *Adapter) getPipelineStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// triggerPipeline triggers a pipeline execution
func (a *Adapter) triggerPipeline(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// stopPipeline stops a running pipeline execution
func (a *Adapter) stopPipeline(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// rollbackDeployment performs a rollback to a previous deployment
func (a *Adapter) rollbackDeployment(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// getCCMCosts gets cloud cost data from CCM
func (a *Adapter) getCCMCosts(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// getFeatureFlags gets all feature flags for a project
func (a *Adapter) getFeatureFlags(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// getCCMRecommendations gets cost optimization recommendations from CCM
func (a *Adapter) getCCMRecommendations(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// getCCMBudgets gets budget information from CCM
func (a *Adapter) getCCMBudgets(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// getCCMAnomalies gets cost anomalies from CCM
func (a *Adapter) getCCMAnomalies(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

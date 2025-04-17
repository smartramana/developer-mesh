package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters"
)

// Config holds configuration for the Harness adapter
type Config struct {
	APIToken       string        `mapstructure:"api_token"`
	AccountID      string        `mapstructure:"account_id"`
	WebhookSecret  string        `mapstructure:"webhook_secret"`
	BaseURL        string        `mapstructure:"base_url"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	MaxRetries     int           `mapstructure:"max_retries"`
	RetryDelay     time.Duration `mapstructure:"retry_delay"`
}

// Adapter implements the Harness integration
type Adapter struct {
	client       *http.Client
	config       Config
	subscribers  map[string][]func(interface{})
	subscriberMu sync.RWMutex
	baseAdapter  adapters.BaseAdapter
}

// NewAdapter creates a new Harness adapter
func NewAdapter(cfg Config) (*Adapter, error) {
	// Configure HTTP client with timeout
	httpClient := &http.Client{
		Timeout: cfg.RequestTimeout,
	}

	adapter := &Adapter{
		client:      httpClient,
		config:      cfg,
		subscribers: make(map[string][]func(interface{})),
		baseAdapter: adapters.BaseAdapter{
			RetryMax:   cfg.MaxRetries,
			RetryDelay: cfg.RetryDelay,
		},
	}

	return adapter, nil
}

// Initialize initializes the adapter
func (a *Adapter) Initialize(ctx context.Context, config interface{}) error {
	// Test connection by calling Harness API
	return a.baseAdapter.CallWithRetry(func() error {
		return a.testConnection(ctx)
	})
}

// testConnection verifies that we can connect to the Harness API
func (a *Adapter) testConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/ping", a.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("X-Api-Key", a.config.APIToken)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Account-Id", a.config.AccountID)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("harness API error: status code %d", resp.StatusCode)
	}

	return nil
}

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	// Handle different Harness actions
	switch action {
	case "get_pipeline_status":
		return a.getPipelineStatus(ctx, params)
	case "trigger_pipeline":
		return a.triggerPipeline(ctx, params)
	case "get_deployment_status":
		return a.getDeploymentStatus(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// getPipelineStatus gets the status of a pipeline
func (a *Adapter) getPipelineStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	pipelineID, ok := params["pipeline_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing pipeline_id parameter")
	}
	
	// Implementation would call Harness API
	// For now, we'll return a simple status
	return map[string]interface{}{
		"pipeline_id": pipelineID,
		"status": "running",
		"started_at": time.Now().Add(-10 * time.Minute).Unix(),
	}, nil
}

// triggerPipeline triggers a pipeline execution
func (a *Adapter) triggerPipeline(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	pipelineID, ok := params["pipeline_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing pipeline_id parameter")
	}
	
	// Extract optional parameters
	variables := make(map[string]string)
	if vars, ok := params["variables"].(map[string]interface{}); ok {
		for k, v := range vars {
			if strVal, ok := v.(string); ok {
				variables[k] = strVal
			}
		}
	}
	
	// Implementation would call Harness API
	// For now, we'll return a simple execution ID
	return map[string]interface{}{
		"pipeline_id": pipelineID,
		"execution_id": fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		"status": "started",
		"started_at": time.Now().Unix(),
	}, nil
}

// getDeploymentStatus gets the status of a deployment
func (a *Adapter) getDeploymentStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract required parameters
	deploymentID, ok := params["deployment_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing deployment_id parameter")
	}
	
	// Implementation would call Harness API
	// For now, we'll return a simple status
	return map[string]interface{}{
		"deployment_id": deploymentID,
		"status": "successful",
		"completed_at": time.Now().Unix(),
		"environment": "production",
	}, nil
}

// IsSafeOperation determines if an operation is safe to perform
func (a *Adapter) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	// Define safety rules for Harness operations
	switch operation {
	case "get_pipeline_status", "get_deployment_status":
		// Read-only operations are always safe
		return true, nil
	case "trigger_pipeline":
		// Check if this is a production pipeline
		if env, ok := params["environment"].(string); ok && env == "production" {
			// Require additional confirmation for production deployments
			if confirmed, ok := params["confirmed"].(bool); ok && confirmed {
				return true, nil
			}
			return false, fmt.Errorf("production deployment requires confirmation")
		}
		// Non-production deployments are considered safe
		return true, nil
	default:
		// Unknown operations are considered unsafe
		return false, fmt.Errorf("unknown operation: %s", operation)
	}
}

// GetData retrieves data from Harness
func (a *Adapter) GetData(ctx context.Context, query interface{}) (interface{}, error) {
	// Implementation would go here - simplified for brevity
	return nil, fmt.Errorf("not implemented")
}

// HandleWebhook processes Harness webhook events
func (a *Adapter) HandleWebhook(ctx context.Context, eventType string, payload []byte) error {
	// Parse the event
	event, err := a.parseWebhookEvent(eventType, payload)
	if err != nil {
		return err
	}

	// Log the received event for debugging
	fmt.Printf("Received Harness webhook event of type %s: %+v\n", eventType, event)

	// Determine if this is a generic webhook based on presence of certain fields
	isGenericEvent := a.isGenericEvent(event)
	
	// Use a specific event type for generic events
	if isGenericEvent {
		// Override the event type for generic webhooks
		eventType = "generic_webhook"
		
		// Extract custom event type if available
		if eventMap, ok := event.(map[string]interface{}); ok {
			if customType, exists := eventMap["event_type"]; exists && customType != nil {
				if customTypeStr, ok := customType.(string); ok && customTypeStr != "" {
					eventType = customTypeStr
				}
			}
		}
	}

	// Notify subscribers
	a.notifySubscribers(eventType, event)

	return nil
}

// isGenericEvent determines if an event is from the Generic Event Relay webhook
func (a *Adapter) isGenericEvent(event interface{}) bool {
	// Generic events typically have certain identifying characteristics
	// such as specific fields in the payload
	eventMap, ok := event.(map[string]interface{})
	if !ok {
		return false
	}

	// Check for fields that indicate this is a generic event relay webhook
	// from the Event Relay feature
	_, hasPayload := eventMap["payload"]
	_, hasHeaders := eventMap["headers"]
	_, hasEventSource := eventMap["event_source"]
	_, hasEventType := eventMap["event_type"]
	_, hasTimestamp := eventMap["timestamp"]
	
	// Primary check for Event Relay format
	if hasPayload && hasHeaders {
		return true
	}
	
	// Also check for typical Harness event relay fields
	if hasEventSource {
		return true
	}
	
	// Look for event_type which is often present in generic webhooks
	if hasEventType && hasTimestamp {
		return true
	}
	
	// Check for typical generic webhook artifact format
	if artifacts, hasArtifacts := eventMap["artifacts"]; hasArtifacts && artifacts != nil {
		if artifactsArr, ok := artifacts.([]interface{}); ok && len(artifactsArr) > 0 {
			// If there's an artifacts array with content, this is likely a generic artifact webhook
			return true
		}
	}
	
	// Additional checks based on specific formats from Event Relay
	if _, hasWebhook := eventMap["webhook"]; hasWebhook {
		return true
	}
	
	// Check for custom_data field which is often used in generic webhooks
	if _, hasCustomData := eventMap["custom_data"]; hasCustomData {
		return true
	}
	
	return false
}

// parseWebhookEvent parses a Harness webhook event
func (a *Adapter) parseWebhookEvent(eventType string, payload []byte) (interface{}, error) {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse Harness webhook payload: %w", err)
	}

	// Extract nested payload if this is a generic webhook event from Event Relay
	// This helps handle the Event Relay formatted payloads
	if payloadData, exists := event["payload"]; exists && payloadData != nil {
		if payloadStr, ok := payloadData.(string); ok && payloadStr != "" {
			// If payload is a string, it might be a JSON string - try to parse it
			var nestedPayload interface{}
			if err := json.Unmarshal([]byte(payloadStr), &nestedPayload); err == nil {
				// Successfully parsed the nested payload
				event["parsed_payload"] = nestedPayload
			}
		} else if payloadMap, ok := payloadData.(map[string]interface{}); ok {
			// Payload is already a map, we can use it directly
			event["parsed_payload"] = payloadMap
		}
	}

	// Add metadata about the event type for easier processing
	if eventType != "" {
		event["event_type"] = eventType
	}

	// Check for artifacts section and ensure it's properly formatted
	if artifacts, exists := event["artifacts"]; exists && artifacts != nil {
		// Ensure artifacts are properly captured for downstream processing
		if artifactsArr, ok := artifacts.([]interface{}); ok && len(artifactsArr) > 0 {
			// Format is correct, we can use it directly
			fmt.Printf("Detected %d artifacts in Harness webhook\n", len(artifactsArr))
		}
	}

	return event, nil
}

// Subscribe adds a callback for a specific event type
func (a *Adapter) Subscribe(eventType string, callback func(interface{})) error {
	a.subscriberMu.Lock()
	defer a.subscriberMu.Unlock()

	a.subscribers[eventType] = append(a.subscribers[eventType], callback)
	return nil
}

// notifySubscribers notifies subscribers of an event
func (a *Adapter) notifySubscribers(eventType string, event interface{}) {
	a.subscriberMu.RLock()
	defer a.subscriberMu.RUnlock()

	// Notify subscribers for the specific event type
	for _, callback := range a.subscribers[eventType] {
		go callback(event)
	}
	
	// Also notify subscribers for "all" events
	for _, callback := range a.subscribers["all"] {
		go callback(event)
	}
}

// Health returns the health status of the adapter
func (a *Adapter) Health() string {
	// Check Harness API status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/ping", a.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Sprintf("unhealthy: %v", err)
	}

	req.Header.Add("X-Api-Key", a.config.APIToken)
	req.Header.Add("X-Account-Id", a.config.AccountID)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Sprintf("unhealthy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("unhealthy: status code %d", resp.StatusCode)
	}

	return "healthy"
}

// No webhook URL methods needed since Harness.io generates the URLs

// Close gracefully shuts down the adapter
func (a *Adapter) Close() error {
	// Nothing specific to clean up for this adapter
	return nil
}

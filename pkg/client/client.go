package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/mcp"
)

// Client is the MCP client
type Client struct {
	baseURL      string
	apiKey       string
	webhookSecret string
	httpClient   *http.Client
	tenantID     string // NEW: Tenant ID for multi-tenant scenarios
}

// ClientOption is a function that configures a Client
type ClientOption func(*Client)

// WithAPIKey sets the API key for authentication
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// WithWebhookSecret sets the webhook secret for signing events
func WithWebhookSecret(secret string) ClientOption {
	return func(c *Client) {
		c.webhookSecret = secret
	}
}

// WithHTTPClient sets the HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTenantID sets the tenant ID for multi-tenant authentication
func WithTenantID(tenantID string) ClientOption {
	return func(c *Client) {
		c.tenantID = tenantID
	}
}

// NewClient creates a new MCP client
func NewClient(baseURL string, options ...ClientOption) *Client {
	client := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, option := range options {
		option(client)
	}

	return client
}

// CreateContext creates a new context
func (c *Client) CreateContext(ctx context.Context, contextData *mcp.Context) (*mcp.Context, error) {
	url := fmt.Sprintf("%s/api/v1/contexts", c.baseURL)
	
	jsonData, err := json.Marshal(contextData)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("failed to create context: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("failed to create context: status %d", resp.StatusCode)
	}
	
	var result mcp.Context
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}

// GetContext retrieves a context by ID
func (c *Client) GetContext(ctx context.Context, contextID string) (*mcp.Context, error) {
	url := fmt.Sprintf("%s/api/v1/contexts/%s", c.baseURL, contextID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("failed to get context: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("failed to get context: status %d", resp.StatusCode)
	}
	
	var result mcp.Context
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}

// UpdateContext updates an existing context
func (c *Client) UpdateContext(ctx context.Context, contextID string, contextData *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	url := fmt.Sprintf("%s/api/v1/contexts/%s", c.baseURL, contextID)
	
	// Create request body
	requestBody := map[string]interface{}{
		"context": contextData,
	}
	
	if options != nil {
		requestBody["options"] = options
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("failed to update context: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("failed to update context: status %d", resp.StatusCode)
	}
	
	var result mcp.Context
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}

// DeleteContext deletes a context
func (c *Client) DeleteContext(ctx context.Context, contextID string) error {
	url := fmt.Sprintf("%s/api/v1/contexts/%s", c.baseURL, contextID)
	
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return fmt.Errorf("failed to delete context: %s", errorMsg)
			}
		}
		return fmt.Errorf("failed to delete context: status %d", resp.StatusCode)
	}
	
	return nil
}

// ListContexts lists contexts for an agent
func (c *Client) ListContexts(ctx context.Context, agentID string, sessionID string, options map[string]string) ([]*mcp.Context, error) {
	// Build URL with query parameters
	baseURL := fmt.Sprintf("%s/api/v1/contexts?agent_id=%s", c.baseURL, agentID)
	
	if sessionID != "" {
		baseURL = fmt.Sprintf("%s&session_id=%s", baseURL, sessionID)
	}
	
	// Add additional options as query parameters
	for key, value := range options {
		baseURL = fmt.Sprintf("%s&%s=%s", baseURL, key, value)
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return nil, err
	}
	
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("failed to list contexts: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("failed to list contexts: status %d", resp.StatusCode)
	}
	
	var result struct {
		Contexts []*mcp.Context `json:"contexts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Contexts, nil
}

// SearchContext searches for text within a context
func (c *Client) SearchContext(ctx context.Context, contextID string, query string) ([]mcp.ContextItem, error) {
	url := fmt.Sprintf("%s/api/v1/contexts/%s/search", c.baseURL, contextID)
	
	requestBody := map[string]string{
		"query": query,
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("failed to search context: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("failed to search context: status %d", resp.StatusCode)
	}
	
	var result struct {
		Results []mcp.ContextItem `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Results, nil
}

// SummarizeContext gets a summary of a context
func (c *Client) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/contexts/%s/summary", c.baseURL, contextID)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return "", fmt.Errorf("failed to get context summary: %s", errorMsg)
			}
		}
		return "", fmt.Errorf("failed to get context summary: status %d", resp.StatusCode)
	}
	
	var result struct {
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	
	return result.Summary, nil
}

// SendEvent sends an event to the MCP server
func (c *Client) SendEvent(ctx context.Context, event *mcp.Event) error {
	url := fmt.Sprintf("%s/webhook/agent", c.baseURL)
	
	// Set timestamp if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	
	jsonData, err := json.Marshal(event)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Sign the request if webhook secret is provided
	if c.webhookSecret != "" {
		mac := hmac.New(sha256.New, []byte(c.webhookSecret))
		mac.Write(jsonData)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-MCP-Signature", signature)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return fmt.Errorf("failed to send event: %s", errorMsg)
			}
		}
		return fmt.Errorf("failed to send event: status %d", resp.StatusCode)
	}
	
	return nil
}

// ExecuteToolAction executes an action on a tool adapter
// Note: Safety restrictions apply - certain operations like repository deletion are blocked.
// Artifactory operations are limited to read-only access.
// Use ListTools() to see what operations are available for each tool.
func (c *Client) ExecuteToolAction(ctx context.Context, contextID string, adapterName string, action string, params map[string]interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/devops/adapters/%s/action", c.baseURL, adapterName)
	
	requestBody := map[string]interface{}{
		"context_id": contextID,
		"action":     action,
		"params":     params,
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("failed to execute tool action: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("failed to execute tool action: status %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result, nil
}

// QueryToolData queries data from a tool adapter
func (c *Client) QueryToolData(ctx context.Context, adapterName string, query map[string]interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/devops/adapters/%s/query", c.baseURL, adapterName)
	
	requestBody := map[string]interface{}{
		"query": query,
	}
	
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("failed to query tool data: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("failed to query tool data: status %d", resp.StatusCode)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result, nil
}

// ListTools lists all available tools/adapters
func (c *Client) ListTools(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/devops/adapters", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil {
			if errorMsg, ok := errResp["error"]; ok {
				return nil, fmt.Errorf("failed to list tools: %s", errorMsg)
			}
		}
		return nil, fmt.Errorf("failed to list tools: status %d", resp.StatusCode)
	}
	
	var result struct {
		Adapters []string `json:"adapters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Adapters, nil
}

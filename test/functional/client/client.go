package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	// Import models package
	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// MCPClient provides methods to interact with the MCP server API
type MCPClient struct {
	BaseURL    string
	APIKey     string
	TenantID   string
	HTTPClient *http.Client
	Logger     Logger
}

// WithTenantID sets the tenant ID for the client
func WithTenantID(tenantID string) func(*MCPClient) {
	return func(c *MCPClient) {
		c.TenantID = tenantID
	}
}

// WithLogger sets a custom logger for the client
func WithLogger(logger Logger) func(*MCPClient) {
	return func(c *MCPClient) {
		c.Logger = logger
	}
}

// NewMCPClient creates a new MCP client
func NewMCPClient(baseURL, apiKey string, opts ...func(*MCPClient)) *MCPClient {
	// Create the client with nil logger initially
	client := &MCPClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		Logger: nil, // Will be set by WithLogger option if provided
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// DoRequest performs an HTTP request to the MCP server
func (c *MCPClient) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return c.DoRequestWithHeader(ctx, method, path, body, nil)
}

// DoRequestWithHeader performs an HTTP request with custom headers
func (c *MCPClient) DoRequestWithHeader(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	// Create URL
	url := fmt.Sprintf("%s%s", c.BaseURL, path)

	// Create request body if provided
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request body: %w", err)
		}
		if method == http.MethodPut || method == http.MethodPost {
			// Use Logger if available, otherwise fall back to os.Stderr
			if c.Logger != nil {
				c.Logger.Infof("Outgoing %s %s body: %s", method, url, string(jsonData))
			} else {
				fmt.Fprintf(os.Stderr, "DEBUG: Outgoing %s %s body: %s\n", method, url, string(jsonData))
			}
		}
		reqBody = bytes.NewBuffer(jsonData)
	} else if method == http.MethodPut || method == http.MethodPost {
		// Use Logger if available, otherwise fall back to os.Stderr
		if c.Logger != nil {
			c.Logger.Infof("Outgoing %s %s body: {}", method, url)
		} else {
			fmt.Fprintf(os.Stderr, "DEBUG: Outgoing %s %s body: {}\n", method, url)
		}
		reqBody = bytes.NewBuffer([]byte("{}"))
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		// Set API key header
		req.Header.Set("X-API-Key", c.APIKey)
	}
	if c.TenantID != "" {
		req.Header.Set("X-Tenant-ID", c.TenantID)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	return resp, nil
}

// Get performs a GET request to the specified path
func (c *MCPClient) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.DoRequest(ctx, http.MethodGet, path, nil)
}

// Post performs a POST request to the specified path with the given payload
func (c *MCPClient) Post(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	return c.DoRequest(ctx, http.MethodPost, path, payload)
}

// Put performs a PUT request to the specified path with the given payload
func (c *MCPClient) Put(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	return c.DoRequest(ctx, http.MethodPut, path, payload)
}

// Delete performs a DELETE request to the specified path
func (c *MCPClient) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.DoRequest(ctx, http.MethodDelete, path, nil)
}

// ParseResponse parses the JSON response body into the provided target struct
func ParseResponse(resp *http.Response, target interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	// Reset the response body so it can be read again if needed
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	// Parse the response
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("error parsing response body: %w", err)
	}

	return nil
}

// ParseWrappedResponse parses a wrapped response that contains data, request_id, and timestamp fields
func ParseWrappedResponse(resp *http.Response, target interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	// Reset the response body so it can be read again if needed
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	// Parse the wrapped response
	var wrapped struct {
		Data      json.RawMessage `json:"data"`
		RequestID string          `json:"request_id"`
		Timestamp string          `json:"timestamp"`
	}

	if err := json.Unmarshal(body, &wrapped); err != nil {
		return fmt.Errorf("error parsing wrapped response: %w", err)
	}

	// Parse the data field into the target
	if err := json.Unmarshal(wrapped.Data, target); err != nil {
		return fmt.Errorf("error parsing data field: %w", err)
	}

	return nil
}

// GetHealth checks the server health status
func (c *MCPClient) GetHealth(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.Get(ctx, "/api/v1/health")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ListTools retrieves the list of available tools
func (c *MCPClient) ListTools(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := c.Get(ctx, "/api/v1/tools")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetContext retrieves a specific context by ID
func (c *MCPClient) GetContext(ctx context.Context, contextID string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/contexts/%s", contextID)
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	// GetContext returns a wrapped response
	if err := ParseWrappedResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetContextTyped retrieves a specific context by ID as a strongly typed model
func (c *MCPClient) GetContextTyped(ctx context.Context, contextID string) (*models.Context, error) {
	path := fmt.Sprintf("/api/v1/contexts/%s", contextID)
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result models.Context
	// GetContext returns a wrapped response
	if err := ParseWrappedResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateContext creates a new context
func (c *MCPClient) CreateContext(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	resp, err := c.Post(ctx, "/api/v1/contexts", payload)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	// CreateContext returns a wrapped response
	if err := ParseWrappedResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// CreateContextTyped creates a new context using a strongly typed model
func (c *MCPClient) CreateContextTyped(ctx context.Context, contextData *models.Context) (*models.Context, error) {
	resp, err := c.Post(ctx, "/api/v1/contexts", contextData)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result models.Context
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateModel creates a new model
func (c *MCPClient) CreateModel(ctx context.Context, model *models.Model) (*models.Model, error) {
	resp, err := c.Post(ctx, "/api/v1/models", model)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Debug: log raw response
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG: CreateModel raw response: %s\n", string(body))
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	// Handle wrapped response format
	var wrappedResponse struct {
		ID    string       `json:"id"`
		Model models.Model `json:"model"`
	}

	// First try parsing as direct model since that's what the API actually returns
	var result models.Model
	if err := json.Unmarshal(body, &result); err == nil && result.ID != "" {
		fmt.Printf("DEBUG: Direct parse succeeded: %+v\n", result)
		return &result, nil
	}

	// If direct parsing fails or returns empty ID, try wrapped format
	if err := json.Unmarshal(body, &wrappedResponse); err == nil && wrappedResponse.Model.ID != "" {
		fmt.Printf("DEBUG: Wrapped parse succeeded, returning model: %+v\n", wrappedResponse.Model)
		return &wrappedResponse.Model, nil
	}

	return nil, fmt.Errorf("failed to parse response: body=%s", string(body))
}

// GetModel retrieves a specific model by ID
func (c *MCPClient) GetModel(ctx context.Context, modelID string) (*models.Model, error) {
	path := fmt.Sprintf("/api/v1/models/%s", modelID)
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result models.Model
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CreateAgent creates a new agent
func (c *MCPClient) CreateAgent(ctx context.Context, agent *models.Agent) (*models.Agent, error) {
	resp, err := c.Post(ctx, "/api/v1/agents", agent)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result models.Agent
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetAgent retrieves a specific agent by ID
func (c *MCPClient) GetAgent(ctx context.Context, agentID string) (*models.Agent, error) {
	path := fmt.Sprintf("/api/v1/agents/%s", agentID)
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result models.Agent
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ExecuteToolAction executes a tool action
func (c *MCPClient) ExecuteToolAction(ctx context.Context, tool, action string, payload map[string]interface{}) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/tools/%s/actions/%s", tool, action)
	resp, err := c.Post(ctx, path, payload)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

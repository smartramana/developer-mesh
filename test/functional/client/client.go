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
)

// MCPClient provides methods to interact with the MCP server API
type MCPClient struct {
	BaseURL    string
	APIKey     string
	TenantID   string
	HTTPClient *http.Client
} // TenantID is used for multi-tenant API scenarios

// NewMCPClient creates a new MCP client
// WithTenantID sets the tenant ID for the client
func WithTenantID(tenantID string) func(*MCPClient) {
	return func(c *MCPClient) {
		c.TenantID = tenantID
	}
}

func NewMCPClient(baseURL, apiKey string, opts ...func(*MCPClient)) *MCPClient {
	client := &MCPClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// DoRequest performs an HTTP request to the MCP server
func (c *MCPClient) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
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
			fmt.Fprintf(os.Stderr, "DEBUG: Outgoing %s %s body: %s\n", method, url, string(jsonData))
		}
		reqBody = bytes.NewBuffer(jsonData)
	} else if method == http.MethodPut || method == http.MethodPost {
		fmt.Fprintf(os.Stderr, "DEBUG: Outgoing %s %s body: {}\n", method, url)
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
		// Send API key directly without Bearer prefix
		req.Header.Set("Authorization", c.APIKey)
	}
	if c.TenantID != "" {
		req.Header.Set("X-Tenant-ID", c.TenantID)
	}

	// Send request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	return resp, nil
}

// [Rest of the file unchanged]
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
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	// Check if response body is empty
	if len(body) == 0 {
		return nil
	}

	// Parse JSON response
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("error parsing response: %w (body: %s)", err, string(body))
	}

	return nil
}

// GetHealth checks the server health status
func (c *MCPClient) GetHealth(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.Get(ctx, "/health")
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

// ListTools retrieves the list of available tools
func (c *MCPClient) ListTools(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := c.Get(ctx, "/api/v1/tools")
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Tools, nil
}

// GetContext retrieves a specific context by ID
func (c *MCPClient) GetContext(ctx context.Context, contextID string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/contexts/%s", contextID)
	resp, err := c.Get(ctx, path)
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
	if err := ParseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
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

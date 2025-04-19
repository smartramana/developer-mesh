//go:build integration

package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// HTTPClient provides a wrapper around the standard http.Client with convenience methods
type HTTPClient struct {
	client  *http.Client
	baseURL string
	apiKey  string
}

// NewHTTPClient creates a new HTTP client for integration tests
func NewHTTPClient(baseURL, apiKey string) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// Get performs a GET request to the specified path
func (c *HTTPClient) Get(path string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add API key as Bearer token if provided
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.client.Do(req)
}

// Post performs a POST request to the specified path with the given payload
func (c *HTTPClient) Post(path string, payload interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	
	var reqBody *bytes.Buffer
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequest("POST", url, reqBody)
	if err != nil {
		return nil, err
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.client.Do(req)
}

// ParseJSONResponse parses a JSON response into the provided target
func ParseJSONResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to parse JSON response: %v, body: %s", err, string(body))
	}
	
	return nil
}

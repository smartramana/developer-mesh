package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/adapters/github/auth"
	"github.com/S-Corkum/devops-mcp/pkg/adapters/resilience"
	"github.com/S-Corkum/devops-mcp/pkg/common/errors"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// RESTClient provides a client for the GitHub REST API
type RESTClient struct {
	baseURL           *url.URL
	client            *http.Client
	authProvider      auth.AuthProvider
	logger            observability.Logger
	rateLimitCallback func(info resilience.GitHubRateLimitInfo)
	etagCache         map[string]string
	responseCache     map[string]any
	cacheMutex        sync.RWMutex
}

// NewRESTClient creates a new GitHub REST client
func NewRESTClient(
	baseURL *url.URL,
	client *http.Client,
	authProvider auth.AuthProvider,
	rateLimitCallback func(info resilience.GitHubRateLimitInfo),
	logger observability.Logger,
) *RESTClient {
	return &RESTClient{
		baseURL:           baseURL,
		client:            client,
		authProvider:      authProvider,
		logger:            logger,
		rateLimitCallback: rateLimitCallback,
		etagCache:         make(map[string]string),
		responseCache:     make(map[string]any),
	}
}

// Request makes a request to the GitHub API
func (c *RESTClient) Request(ctx context.Context, method, path string, body any, result any) error {
	// Build the URL
	u, err := c.buildURL(path)
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	// Create the request
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Check cache for ETag
	cacheKey := method + ":" + u.String()
	c.cacheMutex.RLock()
	etag, hasEtag := c.etagCache[cacheKey]
	c.cacheMutex.RUnlock()
	if hasEtag {
		req.Header.Set("If-None-Match", etag)
	}

	// Add authentication
	err = c.authProvider.AuthenticateRequest(req)
	if err != nil {
		return fmt.Errorf("failed to authenticate request: %w", err)
	}

	// Execute the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Handle rate limiting
	c.handleRateLimiting(resp)

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified && hasEtag {
		c.cacheMutex.RLock()
		cachedResult, hasCachedResult := c.responseCache[cacheKey]
		c.cacheMutex.RUnlock()
		if hasCachedResult && result != nil {
			// Copy the cached result to the result
			cachedBytes, err := json.Marshal(cachedResult)
			if err != nil {
				return fmt.Errorf("failed to marshal cached result: %w", err)
			}
			return json.Unmarshal(cachedBytes, result)
		}
	}

	// Read the response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-success status codes
	if resp.StatusCode >= 400 {
		var errorResponse struct {
			Message          string `json:"message"`
			DocumentationURL string `json:"documentation_url"`
		}
		if err := json.Unmarshal(responseBody, &errorResponse); err != nil {
			// Fallback to a generic error message if JSON parsing fails
			return errors.FromHTTPError(resp.StatusCode, string(responseBody), "")
		}
		return errors.FromHTTPError(resp.StatusCode, errorResponse.Message, errorResponse.DocumentationURL)
	}

	// Update ETag cache
	newEtag := resp.Header.Get("ETag")
	if newEtag != "" {
		c.cacheMutex.Lock()
		c.etagCache[cacheKey] = newEtag
		if result != nil {
			var resultCopy any
			resultBytes, err := json.Marshal(result)
			if err == nil {
				if err := json.Unmarshal(resultBytes, &resultCopy); err == nil {
					c.responseCache[cacheKey] = resultCopy
				}
			}
		}
		c.cacheMutex.Unlock()
	}

	// Parse the response if a result container was provided
	if result != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Get makes a GET request to the GitHub API
func (c *RESTClient) Get(ctx context.Context, path string, result any) error {
	return c.Request(ctx, http.MethodGet, path, nil, result)
}

// Post makes a POST request to the GitHub API
func (c *RESTClient) Post(ctx context.Context, path string, body any, result any) error {
	return c.Request(ctx, http.MethodPost, path, body, result)
}

// Put makes a PUT request to the GitHub API
func (c *RESTClient) Put(ctx context.Context, path string, body any, result any) error {
	return c.Request(ctx, http.MethodPut, path, body, result)
}

// Patch makes a PATCH request to the GitHub API
func (c *RESTClient) Patch(ctx context.Context, path string, body any, result any) error {
	return c.Request(ctx, http.MethodPatch, path, body, result)
}

// Delete makes a DELETE request to the GitHub API
func (c *RESTClient) Delete(ctx context.Context, path string) error {
	return c.Request(ctx, http.MethodDelete, path, nil, nil)
}

// buildURL builds a URL for a GitHub API request
func (c *RESTClient) buildURL(path string) (*url.URL, error) {
	// If the path is already a full URL, return it
	if strings.HasPrefix(path, "http") {
		return url.Parse(path)
	}

	// Make sure path has leading slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	u, err := url.Parse(c.baseURL.String() + path)
	if err != nil {
		return nil, err
	}

	return u, nil
}

// handleRateLimiting handles rate limiting headers from GitHub API responses
func (c *RESTClient) handleRateLimiting(resp *http.Response) {
	// Extract rate limit information from headers
	limit, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))
	remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	resetStr := resp.Header.Get("X-RateLimit-Reset")
	used, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Used"))

	// Parse reset time
	var resetTime time.Time
	if resetStr != "" {
		resetTimestamp, err := strconv.ParseInt(resetStr, 10, 64)
		if err == nil {
			resetTime = time.Unix(resetTimestamp, 0)
		}
	}

	// If we have a callback function for rate limiting, call it
	if c.rateLimitCallback != nil {
		c.rateLimitCallback(resilience.GitHubRateLimitInfo{
			Limit:     limit,
			Remaining: remaining,
			Reset:     resetTime,
			Used:      used,
		})
	}

	// Log rate limit information if it's getting low
	if float64(remaining)/float64(limit) < 0.1 {
		c.logger.Warnf("GitHub API rate limit is low: %d/%d remaining, reset at %s",
			remaining, limit, resetTime.Format(time.RFC3339))
	}
}

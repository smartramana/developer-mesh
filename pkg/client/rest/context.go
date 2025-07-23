package rest

import (
	"context"
	"fmt"
	"net/url"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ContextClient provides methods for interacting with the Context API
type ContextClient struct {
	client *RESTClient
	logger observability.Logger
}

// NewContextClient creates a new ContextClient with the provided client
func NewContextClient(client *RESTClient, logger observability.Logger) *ContextClient {
	return &ContextClient{
		client: client,
		logger: logger,
	}
}

// CreateContext creates a new context
func (c *ContextClient) CreateContext(ctx context.Context, contextObj *models.Context) (*models.Context, error) {
	c.logger.Debug("Creating context via REST API", map[string]interface{}{
		"context_id": contextObj.ID,
	})

	var result models.Context
	if err := c.client.Post(ctx, "/api/v1/contexts", contextObj, &result); err != nil {
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	return &result, nil
}

// GetContext retrieves a context by ID
func (c *ContextClient) GetContext(ctx context.Context, contextID string) (*models.Context, error) {
	c.logger.Debug("Getting context by ID via REST API", map[string]interface{}{
		"context_id": contextID,
	})

	var result models.Context
	if err := c.client.Get(ctx, fmt.Sprintf("/api/v1/contexts/%s", contextID), &result); err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	return &result, nil
}

// UpdateContext updates an existing context
func (c *ContextClient) UpdateContext(ctx context.Context, contextID string, contextObj *models.Context, options *models.ContextUpdateOptions) (*models.Context, error) {
	c.logger.Debug("Updating context via REST API", map[string]interface{}{
		"context_id": contextID,
	})

	// Create the request body with both the context and options
	requestBody := struct {
		Context *models.Context              `json:"context"`
		Options *models.ContextUpdateOptions `json:"options,omitempty"`
	}{
		Context: contextObj,
		Options: options,
	}

	var result models.Context
	if err := c.client.Put(ctx, fmt.Sprintf("/api/v1/contexts/%s", contextID), requestBody, &result); err != nil {
		return nil, fmt.Errorf("failed to update context: %w", err)
	}

	return &result, nil
}

// DeleteContext deletes a context by ID
func (c *ContextClient) DeleteContext(ctx context.Context, contextID string) error {
	c.logger.Debug("Deleting context via REST API", map[string]interface{}{
		"context_id": contextID,
	})

	if err := c.client.Delete(ctx, fmt.Sprintf("/api/v1/contexts/%s", contextID), nil); err != nil {
		return fmt.Errorf("failed to delete context: %w", err)
	}

	return nil
}

// ListContexts lists contexts with optional filtering
func (c *ContextClient) ListContexts(ctx context.Context, agentID, sessionID string, options map[string]interface{}) ([]*models.Context, error) {
	c.logger.Debug("Listing contexts via REST API", map[string]interface{}{
		"agent_id":   agentID,
		"session_id": sessionID,
	})

	// Build query parameters
	query := url.Values{}
	if agentID != "" {
		query.Set("agent_id", agentID)
	}
	if sessionID != "" {
		query.Set("session_id", sessionID)
	}

	// Add additional options as query parameters if provided
	for key, value := range options {
		if strValue, ok := value.(string); ok {
			query.Set(key, strValue)
		}
	}

	endpoint := "/api/v1/contexts"
	if len(query) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, query.Encode())
	}

	var result struct {
		Contexts []*models.Context `json:"contexts"`
	}
	if err := c.client.Get(ctx, endpoint, &result); err != nil {
		return nil, fmt.Errorf("failed to list contexts: %w", err)
	}

	return result.Contexts, nil
}

// SearchInContext searches for text within a context
func (c *ContextClient) SearchInContext(ctx context.Context, contextID, query string) ([]models.ContextItem, error) {
	c.logger.Debug("Searching in context via REST API", map[string]interface{}{
		"context_id": contextID,
		"query":      query,
	})

	requestBody := struct {
		Query string `json:"query"`
	}{
		Query: query,
	}

	var result struct {
		Results []models.ContextItem `json:"results"`
	}
	if err := c.client.Post(ctx, fmt.Sprintf("/api/v1/contexts/%s/search", contextID), requestBody, &result); err != nil {
		return nil, fmt.Errorf("failed to search in context: %w", err)
	}

	return result.Results, nil
}

// SummarizeContext generates a summary of a context
func (c *ContextClient) SummarizeContext(ctx context.Context, contextID string) (string, error) {
	c.logger.Debug("Summarizing context via REST API", map[string]interface{}{
		"context_id": contextID,
	})

	var result struct {
		Summary string `json:"summary"`
	}
	if err := c.client.Get(ctx, fmt.Sprintf("/api/v1/contexts/%s/summary", contextID), &result); err != nil {
		return "", fmt.Errorf("failed to summarize context: %w", err)
	}

	return result.Summary, nil
}

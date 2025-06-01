package proxies

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/client/rest"
	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// ContextAPIProxy implements the ContextRepository interface by delegating to a REST API client
type ContextAPIProxy struct {
	client *rest.ContextClient
	logger observability.Logger
}

// NewContextAPIProxy creates a new ContextAPIProxy
func NewContextAPIProxy(client *rest.ContextClient, logger observability.Logger) repository.ContextRepository {
	if logger == nil {
		logger = observability.NewLogger("context-api-proxy")
	}

	return &ContextAPIProxy{
		client: client,
		logger: logger,
	}
}

// Create creates a new context
func (p *ContextAPIProxy) Create(ctx context.Context, contextObj *repository.Context) error {
	p.logger.Debug("Creating context via API proxy", map[string]interface{}{
		"context_id": contextObj.ID,
	})

	// Convert from repository.Context to models.Context for the REST client
	// Handle the metadata storage pattern for properties that don't exist in models.Context
	metadata := make(map[string]interface{})
	metadata["status"] = contextObj.Status

	// Copy properties to metadata
	if contextObj.Properties != nil {
		for k, v := range contextObj.Properties {
			metadata[k] = v
		}
	}

	mcpContext := &models.Context{
		ID:        contextObj.ID,
		Name:      contextObj.Name,
		AgentID:   contextObj.AgentID,
		SessionID: contextObj.SessionID,
		Metadata:  metadata,
		CreatedAt: time.Unix(contextObj.CreatedAt, 0),
		UpdatedAt: time.Unix(contextObj.UpdatedAt, 0),
	}

	result, err := p.client.CreateContext(ctx, mcpContext)
	if err != nil {
		return fmt.Errorf("failed to create context via REST API: %w", err)
	}

	// Update the original context object with the returned data
	contextObj.ID = result.ID
	contextObj.CreatedAt = result.CreatedAt.Unix() // Convert time.Time to int64
	contextObj.UpdatedAt = result.UpdatedAt.Unix() // Convert time.Time to int64

	return nil
}

// Get retrieves a context by ID
func (p *ContextAPIProxy) Get(ctx context.Context, id string) (*repository.Context, error) {
	p.logger.Debug("Getting context via API proxy", map[string]interface{}{
		"context_id": id,
	})

	result, err := p.client.GetContext(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get context via REST API: %w", err)
	}

	// Convert from models.Context to repository.Context
	// Extract status and other properties from metadata
	status := ""
	properties := make(map[string]interface{})

	if result.Metadata != nil {
		// Extract status if present
		if statusVal, ok := result.Metadata["status"].(string); ok {
			status = statusVal
		}

		// Copy remaining metadata to properties
		for k, v := range result.Metadata {
			if k != "status" { // Avoid duplicate status
				properties[k] = v
			}
		}
	}

	return &repository.Context{
		ID:         result.ID,
		Name:       result.Name,
		AgentID:    result.AgentID,
		SessionID:  result.SessionID,
		Status:     status,
		Properties: properties,
		CreatedAt:  result.CreatedAt.Unix(), // Convert time.Time to int64
		UpdatedAt:  result.UpdatedAt.Unix(), // Convert time.Time to int64
	}, nil
}

// Update updates an existing context
func (p *ContextAPIProxy) Update(ctx context.Context, contextObj *repository.Context) error {
	p.logger.Debug("Updating context via API proxy", map[string]interface{}{
		"context_id": contextObj.ID,
	})

	// Convert from repository.Context to models.Context for the REST client
	// Handle the metadata storage pattern for properties that don't exist in models.Context
	metadata := make(map[string]interface{})
	metadata["status"] = contextObj.Status

	// Copy properties to metadata
	if contextObj.Properties != nil {
		for k, v := range contextObj.Properties {
			metadata[k] = v
		}
	}

	mcpContext := &models.Context{
		ID:        contextObj.ID,
		Name:      contextObj.Name,
		AgentID:   contextObj.AgentID,
		SessionID: contextObj.SessionID,
		Metadata:  metadata,
		CreatedAt: time.Unix(contextObj.CreatedAt, 0),
		UpdatedAt: time.Unix(contextObj.UpdatedAt, 0),
	}

	// Set update options if needed
	options := &models.ContextUpdateOptions{
		// Use options that exist in the current mcp package
		// This may need to be adjusted based on the actual fields available
	}

	_, err := p.client.UpdateContext(ctx, contextObj.ID, mcpContext, options)
	if err != nil {
		return fmt.Errorf("failed to update context via REST API: %w", err)
	}

	return nil
}

// Delete deletes a context by ID
func (p *ContextAPIProxy) Delete(ctx context.Context, id string) error {
	p.logger.Debug("Deleting context via API proxy", map[string]interface{}{
		"context_id": id,
	})

	err := p.client.DeleteContext(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete context via REST API: %w", err)
	}

	return nil
}

// List lists contexts with optional filtering
func (p *ContextAPIProxy) List(ctx context.Context, filter map[string]interface{}) ([]*repository.Context, error) {
	p.logger.Debug("Listing contexts via API proxy", map[string]interface{}{
		"filter": filter,
	})

	// Extract filter parameters
	var agentID, sessionID string
	if filter != nil {
		if aid, ok := filter["agent_id"].(string); ok {
			agentID = aid
		}
		if sid, ok := filter["session_id"].(string); ok {
			sessionID = sid
		}
	}

	results, err := p.client.ListContexts(ctx, agentID, sessionID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts via REST API: %w", err)
	}

	// Convert from models.Context to repository.Context
	contexts := make([]*repository.Context, len(results))
	for i, result := range results {
		// Extract status and other properties from metadata
		status := ""
		properties := make(map[string]interface{})

		if result.Metadata != nil {
			// Extract status if present
			if statusVal, ok := result.Metadata["status"].(string); ok {
				status = statusVal
			}

			// Copy remaining metadata to properties
			for k, v := range result.Metadata {
				if k != "status" { // Avoid duplicate status
					properties[k] = v
				}
			}
		}

		contexts[i] = &repository.Context{
			ID:         result.ID,
			Name:       result.Name,
			AgentID:    result.AgentID,
			SessionID:  result.SessionID,
			Status:     status,
			Properties: properties,
			CreatedAt:  result.CreatedAt.Unix(), // Convert time.Time to int64
			UpdatedAt:  result.UpdatedAt.Unix(), // Convert time.Time to int64
		}
	}

	return contexts, nil
}

// Search searches for text within a context
func (p *ContextAPIProxy) Search(ctx context.Context, contextID, query string) ([]repository.ContextItem, error) {
	p.logger.Debug("Searching in context via API proxy", map[string]interface{}{
		"context_id": contextID,
		"query":      query,
	})

	results, err := p.client.SearchInContext(ctx, contextID, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search in context via REST API: %w", err)
	}

	// Convert from models.ContextItem to repository.ContextItem
	items := make([]repository.ContextItem, len(results))
	for i, result := range results {
		// Create a default type and score
		itemType := "content"
		var score float64 = 0.0

		// Extract type and score from metadata if available
		if result.Metadata != nil {
			if typeVal, ok := result.Metadata["type"].(string); ok {
				itemType = typeVal
			}

			if scoreVal, ok := result.Metadata["score"].(float64); ok {
				score = scoreVal
			}
		}

		items[i] = repository.ContextItem{
			ID:        result.ID,
			ContextID: contextID, // Use the provided contextID
			Content:   result.Content,
			Type:      itemType,
			Score:     score,
			Metadata:  result.Metadata,
		}
	}

	return items, nil
}

// Summarize generates a summary of a context
func (p *ContextAPIProxy) Summarize(ctx context.Context, contextID string) (string, error) {
	p.logger.Debug("Summarizing context via API proxy", map[string]interface{}{
		"context_id": contextID,
	})

	summary, err := p.client.SummarizeContext(ctx, contextID)
	if err != nil {
		return "", fmt.Errorf("failed to summarize context via REST API: %w", err)
	}

	return summary, nil
}

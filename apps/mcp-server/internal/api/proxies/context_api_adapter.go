package proxies

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/client/rest"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/S-Corkum/devops-mcp/pkg/repository"
)

// ContextAPIAdapter implements the repository.ContextRepository interface
// by delegating to a REST API client and handling the necessary conversions
type ContextAPIAdapter struct {
	client *rest.ContextClient
	logger observability.Logger
}

// NewContextAPIAdapter creates a new ContextAPIAdapter
func NewContextAPIAdapter(client *rest.ContextClient, logger observability.Logger) repository.ContextRepository {
	if logger == nil {
		logger = observability.NewLogger("context-api-adapter")
	}

	return &ContextAPIAdapter{
		client: client,
		logger: logger,
	}
}

// repoToMCPContext converts from repository.Context to mcp.Context
func (a *ContextAPIAdapter) repoToMCPContext(repoContext *repository.Context) *mcp.Context {
	// Create metadata map from repository properties
	metadata := make(map[string]interface{})
	if repoContext.Properties != nil {
		// Add status to metadata
		metadata["status"] = repoContext.Status
		
		// Copy all other properties
		for k, v := range repoContext.Properties {
			metadata[k] = v
		}
	}
	
	// Convert timestamps
	createdAt := time.Unix(repoContext.CreatedAt, 0)
	updatedAt := time.Unix(repoContext.UpdatedAt, 0)
	
	// Create MCP context
	mcpContext := &mcp.Context{
		ID:        repoContext.ID,
		Name:      repoContext.Name,
		AgentID:   repoContext.AgentID,
		SessionID: repoContext.SessionID,
		Metadata:  metadata,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Content:   []mcp.ContextItem{}, // Initialize empty content array
	}
	
	return mcpContext
}

// mcpToRepoContext converts from mcp.Context to repository.Context
func (a *ContextAPIAdapter) mcpToRepoContext(mcpContext *mcp.Context) *repository.Context {
	// Create new repository context
	repoContext := &repository.Context{
		ID:         mcpContext.ID,
		Name:       mcpContext.Name,
		AgentID:    mcpContext.AgentID,
		SessionID:  mcpContext.SessionID,
		Status:     "", // Default
		Properties: make(map[string]interface{}),
		CreatedAt:  mcpContext.CreatedAt.Unix(),
		UpdatedAt:  mcpContext.UpdatedAt.Unix(),
	}
	
	// Extract additional fields from metadata
	if mcpContext.Metadata != nil {
		// Get status if present
		if status, ok := mcpContext.Metadata["status"].(string); ok {
			repoContext.Status = status
		}
		
		// Copy remaining metadata to properties
		for k, v := range mcpContext.Metadata {
			if k != "status" { // Avoid duplication
				repoContext.Properties[k] = v
			}
		}
	}
	
	return repoContext
}

// Create creates a new context
func (a *ContextAPIAdapter) Create(ctx context.Context, contextObj *repository.Context) error {
	a.logger.Debug("Creating context via adapter", map[string]interface{}{
		"context_id": contextObj.ID,
	})
	
	// Convert repository context to MCP context
	mcpContext := a.repoToMCPContext(contextObj)
	
	// Call REST API
	result, err := a.client.CreateContext(ctx, mcpContext)
	if err != nil {
		return fmt.Errorf("failed to create context via REST API: %w", err)
	}
	
	// Update the original context object with returned data
	contextObj.ID = result.ID
	contextObj.CreatedAt = result.CreatedAt.Unix()
	contextObj.UpdatedAt = result.UpdatedAt.Unix()
	
	return nil
}

// Get retrieves a context by ID
func (a *ContextAPIAdapter) Get(ctx context.Context, id string) (*repository.Context, error) {
	a.logger.Debug("Getting context via adapter", map[string]interface{}{
		"context_id": id,
	})
	
	// Call REST API
	result, err := a.client.GetContext(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get context via REST API: %w", err)
	}
	
	// Convert MCP context to repository context
	return a.mcpToRepoContext(result), nil
}

// Update updates an existing context
func (a *ContextAPIAdapter) Update(ctx context.Context, contextObj *repository.Context) error {
	a.logger.Debug("Updating context via adapter", map[string]interface{}{
		"context_id": contextObj.ID,
	})
	
	// Convert repository context to MCP context
	mcpContext := a.repoToMCPContext(contextObj)
	
	// Set update options
	options := &mcp.ContextUpdateOptions{
		ReplaceContent: false,
	}
	
	// Call REST API
	_, err := a.client.UpdateContext(ctx, contextObj.ID, mcpContext, options)
	if err != nil {
		return fmt.Errorf("failed to update context via REST API: %w", err)
	}
	
	return nil
}

// Delete deletes a context by ID
func (a *ContextAPIAdapter) Delete(ctx context.Context, id string) error {
	a.logger.Debug("Deleting context via adapter", map[string]interface{}{
		"context_id": id,
	})
	
	// Call REST API
	err := a.client.DeleteContext(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete context via REST API: %w", err)
	}
	
	return nil
}

// List lists contexts with optional filtering
func (a *ContextAPIAdapter) List(ctx context.Context, filter map[string]interface{}) ([]*repository.Context, error) {
	a.logger.Debug("Listing contexts via adapter", map[string]interface{}{
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
	
	// Call REST API
	results, err := a.client.ListContexts(ctx, agentID, sessionID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts via REST API: %w", err)
	}
	
	// Convert MCP contexts to repository contexts
	contexts := make([]*repository.Context, len(results))
	for i, result := range results {
		contexts[i] = a.mcpToRepoContext(result)
	}
	
	return contexts, nil
}

// mcpItemToRepoItem converts from mcp.ContextItem to repository.ContextItem
func (a *ContextAPIAdapter) mcpItemToRepoItem(mcpItem mcp.ContextItem, contextID string) repository.ContextItem {
	// Create new repository context item
	repoItem := repository.ContextItem{
		ID:        mcpItem.ID,
		ContextID: contextID,
		Content:   mcpItem.Content, 
		Type:      "content", // Default
		Score:     0,
		Metadata:  mcpItem.Metadata,
	}
	
	// Extract additional fields from metadata
	if mcpItem.Metadata != nil {
		// Get type if present
		if itemType, ok := mcpItem.Metadata["type"].(string); ok {
			repoItem.Type = itemType
		}
		
		// Get score if present
		if score, ok := mcpItem.Metadata["score"].(float64); ok {
			repoItem.Score = score
		}
	}
	
	return repoItem
}

// Search searches for text within a context
func (a *ContextAPIAdapter) Search(ctx context.Context, contextID, query string) ([]repository.ContextItem, error) {
	a.logger.Debug("Searching in context via adapter", map[string]interface{}{
		"context_id": contextID,
		"query":      query,
	})
	
	// Call REST API
	results, err := a.client.SearchInContext(ctx, contextID, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search in context via REST API: %w", err)
	}
	
	// Convert MCP context items to repository context items
	items := make([]repository.ContextItem, len(results))
	for i, result := range results {
		items[i] = a.mcpItemToRepoItem(result, contextID)
	}
	
	return items, nil
}

// Summarize generates a summary of a context
func (a *ContextAPIAdapter) Summarize(ctx context.Context, contextID string) (string, error) {
	a.logger.Debug("Summarizing context via adapter", map[string]interface{}{
		"context_id": contextID,
	})
	
	// Call REST API
	summary, err := a.client.SummarizeContext(ctx, contextID)
	if err != nil {
		return "", fmt.Errorf("failed to summarize context via REST API: %w", err)
	}
	
	return summary, nil
}

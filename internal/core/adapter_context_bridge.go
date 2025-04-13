package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// AdapterContextBridge connects adapters with the context management system
type AdapterContextBridge struct {
	contextManager interfaces.ContextManager
	adapters       map[string]interfaces.Adapter
}

// NewAdapterContextBridge creates a new bridge
func NewAdapterContextBridge(contextManager interfaces.ContextManager, adapters map[string]interfaces.Adapter) *AdapterContextBridge {
	return &AdapterContextBridge{
		contextManager: contextManager,
		adapters:       adapters,
	}
}

// ExecuteToolAction executes an action on a tool and records it in context
func (b *AdapterContextBridge) ExecuteToolAction(ctx context.Context, contextID string, tool string, action string, params map[string]interface{}) (interface{}, error) {
	// Get the adapter for the specified tool
	adapter, ok := b.adapters[tool]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}

	// Get the current context
	contextData, err := b.contextManager.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	// Record the tool action in the context
	actionRecord := mcp.ContextItem{
		Role:      "tool_request",
		Content:   fmt.Sprintf("Tool: %s\nAction: %s\nParams: %v", tool, action, params),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"tool":   tool,
			"action": action,
			"params": params,
		},
	}

	// Update the context with the tool request
	contextData.Content = append(contextData.Content, actionRecord)
	contextData, err = b.contextManager.UpdateContext(ctx, contextID, contextData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update context with tool request: %w", err)
	}

	// Execute the action using the adapter
	result, err := adapter.ExecuteAction(ctx, contextID, action, params)
	if err != nil {
		// Record the error in the context
		errorRecord := mcp.ContextItem{
			Role:      "tool_error",
			Content:   fmt.Sprintf("Error executing %s action on %s: %v", action, tool, err),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"tool":   tool,
				"action": action,
				"error":  err.Error(),
			},
		}
		contextData.Content = append(contextData.Content, errorRecord)
		_, updateErr := b.contextManager.UpdateContext(ctx, contextID, contextData, nil)
		if updateErr != nil {
			// Log the error but don't fail the whole operation
			fmt.Printf("Failed to update context with error: %v\n", updateErr)
		}
		return nil, err
	}

	// Marshal the result for storage
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	// Record the successful result in the context
	resultRecord := mcp.ContextItem{
		Role:      "tool_response",
		Content:   string(resultJSON),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"tool":   tool,
			"action": action,
			"status": "success",
		},
	}

	// Update the context with the tool response
	contextData.Content = append(contextData.Content, resultRecord)
	_, err = b.contextManager.UpdateContext(ctx, contextID, contextData, nil)
	if err != nil {
		// Log but don't fail the operation
		fmt.Printf("Failed to update context with result: %v\n", err)
	}

	return result, nil
}

// GetToolData retrieves data from a tool and records it in context
func (b *AdapterContextBridge) GetToolData(ctx context.Context, contextID string, tool string, query interface{}) (interface{}, error) {
	// Get the adapter for the specified tool
	adapter, ok := b.adapters[tool]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}

	// Get the current context
	contextData, err := b.contextManager.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}

	// Record the data query in the context
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	queryRecord := mcp.ContextItem{
		Role:      "tool_query",
		Content:   string(queryJSON),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"tool":  tool,
			"query": query,
		},
	}

	// Update the context with the query
	contextData.Content = append(contextData.Content, queryRecord)
	contextData, err = b.contextManager.UpdateContext(ctx, contextID, contextData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update context with query: %w", err)
	}

	// Get the data using the adapter
	result, err := adapter.GetData(ctx, query)
	if err != nil {
		// Record the error in the context
		errorRecord := mcp.ContextItem{
			Role:      "tool_error",
			Content:   fmt.Sprintf("Error getting data from %s: %v", tool, err),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"tool":  tool,
				"error": err.Error(),
			},
		}
		contextData.Content = append(contextData.Content, errorRecord)
		_, updateErr := b.contextManager.UpdateContext(ctx, contextID, contextData, nil)
		if updateErr != nil {
			// Log the error but don't fail the whole operation
			fmt.Printf("Failed to update context with error: %v\n", updateErr)
		}
		return nil, err
	}

	// Marshal the result for storage
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	// Record the successful result in the context
	resultRecord := mcp.ContextItem{
		Role:      "tool_data",
		Content:   string(resultJSON),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"tool":   tool,
			"status": "success",
		},
	}

	// Update the context with the data response
	contextData.Content = append(contextData.Content, resultRecord)
	_, err = b.contextManager.UpdateContext(ctx, contextID, contextData, nil)
	if err != nil {
		// Log but don't fail the operation
		fmt.Printf("Failed to update context with result: %v\n", err)
	}

	return result, nil
}

// HandleToolWebhook processes a webhook and records it in appropriate contexts
func (b *AdapterContextBridge) HandleToolWebhook(ctx context.Context, tool string, eventType string, payload []byte) error {
	// Get the adapter for the specified tool
	adapter, ok := b.adapters[tool]
	if !ok {
		return fmt.Errorf("unknown tool: %s", tool)
	}

	// Parse the payload to extract context relevant information
	var payloadJSON map[string]interface{}
	if err := json.Unmarshal(payload, &payloadJSON); err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Look for context identifiers in the payload
	// This is hypothetical and would need to be customized based on webhook payloads
	var contextIDs []string
	
	// Example: If the payload has a "metadata" field with "context_ids"
	if metadata, ok := payloadJSON["metadata"].(map[string]interface{}); ok {
		if ids, ok := metadata["context_ids"].([]interface{}); ok {
			for _, id := range ids {
				if contextID, ok := id.(string); ok {
					contextIDs = append(contextIDs, contextID)
				}
			}
		}
	}

	// Forward the webhook to the adapter
	if handler, ok := adapter.(interfaces.WebhookHandler); ok {
		if err := handler.HandleWebhook(ctx, eventType, payload); err != nil {
			return fmt.Errorf("failed to handle webhook: %w", err)
		}
	} else {
		return fmt.Errorf("adapter does not support webhook handling")
	}

	// Record the webhook in relevant contexts
	for _, contextID := range contextIDs {
		contextData, err := b.contextManager.GetContext(ctx, contextID)
		if err != nil {
			// Log but continue
			fmt.Printf("Failed to get context %s: %v\n", contextID, err)
			continue
		}

		// Record the webhook in the context
		webhookRecord := mcp.ContextItem{
			Role:      "webhook",
			Content:   string(payload),
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"tool":       tool,
				"event_type": eventType,
			},
		}

		// Update the context with the webhook
		contextData.Content = append(contextData.Content, webhookRecord)
		_, err = b.contextManager.UpdateContext(ctx, contextID, contextData, nil)
		if err != nil {
			// Log but continue
			fmt.Printf("Failed to update context %s: %v\n", contextID, err)
		}
	}

	return nil
}



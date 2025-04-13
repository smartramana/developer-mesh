package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// ContextAwareAdapter adds context management capabilities to adapters
type ContextAwareAdapter struct {
	contextManager interfaces.ContextManager
	adapterName    string
}

// NewContextAwareAdapter creates a new context-aware adapter wrapper
func NewContextAwareAdapter(contextManager interfaces.ContextManager, adapterName string) *ContextAwareAdapter {
	return &ContextAwareAdapter{
		contextManager: contextManager,
		adapterName:    adapterName,
	}
}

// RecordOperationInContext records an operation performed by an adapter in a context
func (ca *ContextAwareAdapter) RecordOperationInContext(ctx context.Context, contextID string, operation string, request interface{}, response interface{}) error {
	// Get current context
	contextData, err := ca.contextManager.GetContext(ctx, contextID)
	if err != nil {
		return fmt.Errorf("failed to get context %s: %w", contextID, err)
	}

	// Convert request and response to JSON strings for storage
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Create a new context item for this operation
	operationItem := mcp.ContextItem{
		Role:      "tool",
		Content:   fmt.Sprintf("Operation: %s\nAdapter: %s\nRequest: %s\nResponse: %s", operation, ca.adapterName, string(requestJSON), string(responseJSON)),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"adapter":   ca.adapterName,
			"operation": operation,
			"request":   request,
			"response":  response,
		},
	}

	// Calculate approximate token count
	operationItem.Tokens = len(operationItem.Content) / 4 // Rough approximation

	// Add operation to context
	contextData.Content = append(contextData.Content, operationItem)
	contextData.CurrentTokens += operationItem.Tokens

	// Update the context
	_, err = ca.contextManager.UpdateContext(ctx, contextID, contextData, &mcp.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	})

	return err
}

// RecordWebhookInContext records a webhook event in a context
func (ca *ContextAwareAdapter) RecordWebhookInContext(ctx context.Context, agentID string, eventType string, payload interface{}) (string, error) {
	// Look for an existing context for this agent
	contexts, err := ca.contextManager.ListContexts(ctx, agentID, "", map[string]interface{}{
		"limit": 1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list contexts: %w", err)
	}

	var contextData *mcp.Context
	var contextID string

	if len(contexts) > 0 {
		contextData = contexts[0]
		contextID = contextData.ID
	} else {
		// Create a new context if none exists
		contextData = &mcp.Context{
			AgentID:       agentID,
			ModelID:       "webhook",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			MaxTokens:     100000, // Default high limit for webhooks
			CurrentTokens: 0,
			Content:       []mcp.ContextItem{},
			Metadata: map[string]interface{}{
				"source": "webhook",
			},
		}

		newContext, err := ca.contextManager.CreateContext(ctx, contextData)
		if err != nil {
			return "", fmt.Errorf("failed to create context: %w", err)
		}
		contextData = newContext
		contextID = newContext.ID
	}

	// Convert payload to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return contextID, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create a new context item for this webhook
	webhookItem := mcp.ContextItem{
		Role:      "event",
		Content:   fmt.Sprintf("Event: %s\nAdapter: %s\nPayload: %s", eventType, ca.adapterName, string(payloadJSON)),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"adapter":   ca.adapterName,
			"eventType": eventType,
			"payload":   payload,
		},
	}

	// Calculate approximate token count
	webhookItem.Tokens = len(webhookItem.Content) / 4 // Rough approximation

	// Add webhook to context
	contextData.Content = append(contextData.Content, webhookItem)
	contextData.CurrentTokens += webhookItem.Tokens

	// Update the context
	_, err = ca.contextManager.UpdateContext(ctx, contextID, contextData, &mcp.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	})

	return contextID, err
}

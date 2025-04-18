package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/core"
	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/adapters/resilience"
	"github.com/S-Corkum/mcp-server/internal/interfaces"
	"github.com/S-Corkum/mcp-server/internal/observability"
	"github.com/S-Corkum/mcp-server/pkg/mcp"
)

// ContextBridge bridges adapters to the context management system
type ContextBridge struct {
	contextManager interfaces.ContextManager
	logger         *observability.Logger
	eventBus       *events.EventBus
	retryConfig    resilience.RetryConfig
}

// NewContextBridge creates a new context bridge
func NewContextBridge(
	contextManager interfaces.ContextManager,
	logger *observability.Logger,
	eventBus *events.EventBus,
) *ContextBridge {
	bridge := &ContextBridge{
		contextManager: contextManager,
		logger:         logger,
		eventBus:       eventBus,
		retryConfig:    resilience.DefaultRetryConfig(),
	}
	
	// Subscribe to adapter events to record them in context
	if eventBus != nil {
		eventBus.SubscribeAll(bridge)
	}
	
	return bridge
}

// WithRetryConfig sets the retry configuration for context operations
func (b *ContextBridge) WithRetryConfig(config resilience.RetryConfig) *ContextBridge {
	b.retryConfig = config
	return b
}

// Handle handles adapter events and records them in context if appropriate
func (b *ContextBridge) Handle(ctx context.Context, event *events.AdapterEvent) error {
	// Check if the event includes a context ID
	contextID, ok := event.Metadata["contextId"].(string)
	if !ok || contextID == "" {
		// No context ID, nothing to do
		return nil
	}
	
	// Record the event in the context
	return b.RecordEventInContext(ctx, contextID, event)
}

// RecordOperationInContext records an adapter operation in a context
func (b *ContextBridge) RecordOperationInContext(ctx context.Context, contextID string, adapterType string, operation string, request interface{}, response interface{}, err error) error {
	// Get current context with retry
	contextData, getErr := b.getContextWithRetry(ctx, contextID)
	if getErr != nil {
		// Log error but don't fail the operation completely
		b.logger.Warn("Failed to get context", map[string]interface{}{
			"contextId":   contextID,
			"adapterType": adapterType,
			"operation":   operation,
			"error":       getErr.Error(),
		})
		
		// Create a temporary context item to track the operation even if we couldn't get the full context
		tempItem := b.createOperationContextItem(adapterType, operation, request, response, err)
		
		// Try to append just this item as a standalone update
		return b.appendContextItem(ctx, contextID, tempItem)
	}
	
	// Convert request and response to JSON strings for storage
	requestJSON, jsonErr := b.safeJSONMarshal(request)
	if jsonErr != nil {
		b.logger.Warn("Failed to marshal request", map[string]interface{}{
			"contextId":   contextID,
			"adapterType": adapterType,
			"operation":   operation,
			"error":       jsonErr.Error(),
		})
		requestJSON = []byte("Error marshaling request")
	}
	
	responseJSON, jsonErr := b.safeJSONMarshal(response)
	if jsonErr != nil {
		b.logger.Warn("Failed to marshal response", map[string]interface{}{
			"contextId":   contextID,
			"adapterType": adapterType,
			"operation":   operation,
			"error":       jsonErr.Error(),
		})
		responseJSON = []byte("Error marshaling response")
	}
	
	// Create a new context item for this operation
	var operationStatus string
	if err != nil {
		operationStatus = "failure"
	} else {
		operationStatus = "success"
	}
	
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}
	
	operationItem := mcp.ContextItem{
		Role:      "tool",
		Content:   fmt.Sprintf("Operation: %s\nAdapter: %s\nStatus: %s\nRequest: %s\nResponse: %s\nError: %s", 
			operation, adapterType, operationStatus, string(requestJSON), string(responseJSON), errorMsg),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"adapter":   adapterType,
			"operation": operation,
			"status":    operationStatus,
			"request":   request,
			"response":  response,
			"error":     errorMsg,
		},
	}
	
	// Calculate approximate token count
	operationItem.Tokens = len(operationItem.Content) / 4 // Rough approximation
	
	// Add operation to context
	contextData.Content = append(contextData.Content, operationItem)
	contextData.CurrentTokens += operationItem.Tokens
	
	// Update the context with retry mechanism
	_, updateErr := b.updateContextWithRetry(ctx, contextID, contextData, &mcp.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	})
	
	return updateErr
}

// RecordEventInContext records an adapter event in a context
func (b *ContextBridge) RecordEventInContext(ctx context.Context, contextID string, event *events.AdapterEvent) error {
	// Get current context with retry
	contextData, getErr := b.getContextWithRetry(ctx, contextID)
	if getErr != nil {
		// Log error but don't fail the operation completely
		b.logger.Warn("Failed to get context for event", map[string]interface{}{
			"contextId":   contextID,
			"adapterType": event.AdapterType,
			"eventType":   string(event.EventType),
			"error":       getErr.Error(),
		})
		
		// Create a temporary context item to track the event even if we couldn't get the full context
		tempItem := b.createEventContextItem(event)
		
		// Try to append just this item as a standalone update
		return b.appendContextItem(ctx, contextID, tempItem)
	}
	
	// Convert payload to JSON
	payloadJSON, jsonErr := b.safeJSONMarshal(event.Payload)
	if jsonErr != nil {
		b.logger.Warn("Failed to marshal event payload", map[string]interface{}{
			"contextId":   contextID,
			"adapterType": event.AdapterType,
			"eventType":   string(event.EventType),
			"error":       jsonErr.Error(),
		})
		payloadJSON = []byte("Error marshaling payload")
	}
	
	// Create a new context item for this event
	eventItem := mcp.ContextItem{
		Role:      "event",
		Content:   fmt.Sprintf("Event: %s\nAdapter: %s\nTimestamp: %s\nPayload: %s", 
			string(event.EventType), event.AdapterType, event.Timestamp.Format(time.RFC3339), string(payloadJSON)),
		Timestamp: event.Timestamp,
		Metadata: map[string]interface{}{
			"adapter":    event.AdapterType,
			"eventType":  string(event.EventType),
			"eventId":    event.ID,
			"payload":    event.Payload,
			"eventMeta":  event.Metadata,
		},
	}
	
	// Calculate approximate token count
	eventItem.Tokens = len(eventItem.Content) / 4 // Rough approximation
	
	// Add event to context
	contextData.Content = append(contextData.Content, eventItem)
	contextData.CurrentTokens += eventItem.Tokens
	
	// Update the context with retry mechanism
	_, updateErr := b.updateContextWithRetry(ctx, contextID, contextData, &mcp.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	})
	
	return updateErr
}

// RecordWebhookInContext records a webhook event in a context
func (b *ContextBridge) RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error) {
	// Look for an existing context for this agent
	contexts, err := b.contextManager.ListContexts(ctx, agentID, "", map[string]interface{}{
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
		
		newContext, err := b.contextManager.CreateContext(ctx, contextData)
		if err != nil {
			return "", fmt.Errorf("failed to create context: %w", err)
		}
		contextData = newContext
		contextID = newContext.ID
	}
	
	// Convert payload to JSON
	payloadJSON, err := b.safeJSONMarshal(payload)
	if err != nil {
		b.logger.Warn("Failed to marshal webhook payload", map[string]interface{}{
			"agentId":     agentID,
			"adapterType": adapterType,
			"eventType":   eventType,
			"error":       err.Error(),
		})
		payloadJSON = []byte("Error marshaling payload")
	}
	
	// Create a new context item for this webhook
	webhookItem := mcp.ContextItem{
		Role:      "webhook",
		Content:   fmt.Sprintf("Webhook: %s\nAdapter: %s\nTimestamp: %s\nPayload: %s", 
			eventType, adapterType, time.Now().Format(time.RFC3339), string(payloadJSON)),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"adapter":   adapterType,
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
	_, err = b.contextManager.UpdateContext(ctx, contextID, contextData, &mcp.ContextUpdateOptions{
		Truncate:         true,
		TruncateStrategy: "oldest_first",
	})
	
	if err != nil {
		return contextID, fmt.Errorf("failed to update context with webhook: %w", err)
	}
	
	// Emit event for the webhook
	if b.eventBus != nil {
		event := events.NewAdapterEvent(adapterType, events.EventTypeWebhookReceived, payload)
		event.WithMetadata("contextId", contextID)
		event.WithMetadata("eventType", eventType)
		
		b.eventBus.Emit(ctx, event)
	}
	
	return contextID, nil
}

// Helper methods

// getContextWithRetry gets a context with retry
func (b *ContextBridge) getContextWithRetry(ctx context.Context, contextID string) (*mcp.Context, error) {
	var contextData *mcp.Context
	var err error
	
	operation := func() error {
		contextData, err = b.contextManager.GetContext(ctx, contextID)
		return err
	}
	
	err = resilience.Retry(ctx, b.retryConfig, operation)
	return contextData, err
}

// updateContextWithRetry updates a context with retry
func (b *ContextBridge) updateContextWithRetry(ctx context.Context, contextID string, contextData *mcp.Context, options *mcp.ContextUpdateOptions) (*mcp.Context, error) {
	var updatedContext *mcp.Context
	var err error
	
	operation := func() error {
		updatedContext, err = b.contextManager.UpdateContext(ctx, contextID, contextData, options)
		return err
	}
	
	err = resilience.Retry(ctx, b.retryConfig, operation)
	return updatedContext, err
}

// appendContextItem appends a single item to a context
func (b *ContextBridge) appendContextItem(ctx context.Context, contextID string, item mcp.ContextItem) error {
	var err error
	
	operation := func() error {
		// Get just enough context data to append this item
		contextData, err := b.contextManager.GetContext(ctx, contextID)
		if err != nil {
			return err
		}
		
		// Add the item
		contextData.Content = append(contextData.Content, item)
		contextData.CurrentTokens += item.Tokens
		
		// Update the context
		_, err = b.contextManager.UpdateContext(ctx, contextID, contextData, &mcp.ContextUpdateOptions{
			Truncate:         true,
			TruncateStrategy: "oldest_first",
		})
		
		return err
	}
	
	return resilience.Retry(ctx, b.retryConfig, operation)
}

// createOperationContextItem creates a context item for an operation
func (b *ContextBridge) createOperationContextItem(adapterType string, operation string, request interface{}, response interface{}, err error) mcp.ContextItem {
	// Convert request and response to JSON strings for storage
	requestJSON, jsonErr := b.safeJSONMarshal(request)
	if jsonErr != nil {
		requestJSON = []byte("Error marshaling request")
	}
	
	responseJSON, jsonErr := b.safeJSONMarshal(response)
	if jsonErr != nil {
		responseJSON = []byte("Error marshaling response")
	}
	
	// Create a new context item for this operation
	var operationStatus string
	if err != nil {
		operationStatus = "failure"
	} else {
		operationStatus = "success"
	}
	
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}
	
	item := mcp.ContextItem{
		Role:      "tool",
		Content:   fmt.Sprintf("Operation: %s\nAdapter: %s\nStatus: %s\nRequest: %s\nResponse: %s\nError: %s", 
			operation, adapterType, operationStatus, string(requestJSON), string(responseJSON), errorMsg),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"adapter":   adapterType,
			"operation": operation,
			"status":    operationStatus,
			"request":   request,
			"response":  response,
			"error":     errorMsg,
		},
	}
	
	// Calculate approximate token count
	item.Tokens = len(item.Content) / 4 // Rough approximation
	
	return item
}

// createEventContextItem creates a context item for an event
func (b *ContextBridge) createEventContextItem(event *events.AdapterEvent) mcp.ContextItem {
	// Convert payload to JSON
	payloadJSON, jsonErr := b.safeJSONMarshal(event.Payload)
	if jsonErr != nil {
		payloadJSON = []byte("Error marshaling payload")
	}
	
	// Create a new context item for this event
	item := mcp.ContextItem{
		Role:      "event",
		Content:   fmt.Sprintf("Event: %s\nAdapter: %s\nTimestamp: %s\nPayload: %s", 
			string(event.EventType), event.AdapterType, event.Timestamp.Format(time.RFC3339), string(payloadJSON)),
		Timestamp: event.Timestamp,
		Metadata: map[string]interface{}{
			"adapter":    event.AdapterType,
			"eventType":  string(event.EventType),
			"eventId":    event.ID,
			"payload":    event.Payload,
			"eventMeta":  event.Metadata,
		},
	}
	
	// Calculate approximate token count
	item.Tokens = len(item.Content) / 4 // Rough approximation
	
	return item
}

// safeJSONMarshal safely marshals an object to JSON
func (b *ContextBridge) safeJSONMarshal(obj interface{}) ([]byte, error) {
	if obj == nil {
		return []byte("null"), nil
	}
	
	// Marshal with HTMLEscape disabled for better readability
	buffer, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	
	return buffer, nil
}

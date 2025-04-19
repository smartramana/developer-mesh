package bridge

import (
	"context"
	"fmt"
	"time"

	"github.com/S-Corkum/mcp-server/internal/adapters/events"
	"github.com/S-Corkum/mcp-server/internal/observability"
)

// ContextBridge implements a stub for the former context-bridge functionality
// It no longer connects to a context management system but provides compatible methods
// that log operations instead of storing them
type ContextBridge struct {
	logger   *observability.Logger
	eventBus *events.EventBus
}

// NewContextBridge creates a new context bridge
func NewContextBridge(
	_ interface{}, // Formerly contextManager
	logger *observability.Logger,
	eventBus *events.EventBus,
) *ContextBridge {
	bridge := &ContextBridge{
		logger:   logger,
		eventBus: eventBus,
	}
	
	// Subscribe to adapter events
	if eventBus != nil {
		eventBus.SubscribeAll(bridge)
	}
	
	return bridge
}

// Handle handles adapter events
func (b *ContextBridge) Handle(ctx context.Context, event *events.AdapterEvent) error {
	// Just log the event now
	b.logger.Info("Received adapter event", map[string]interface{}{
		"adapterType": event.AdapterType,
		"eventType":   string(event.EventType),
		"eventId":     event.ID,
	})
	
	return nil
}

// RecordOperationInContext logs an adapter operation
func (b *ContextBridge) RecordOperationInContext(ctx context.Context, contextID string, adapterType string, operation string, params map[string]interface{}, result interface{}, err error) error {
	// Just log the operation now
	status := "success"
	if err != nil {
		status = "error"
	}
	
	b.logger.Info("Adapter operation", map[string]interface{}{
		"contextId":   contextID,
		"adapterType": adapterType,
		"operation":   operation,
		"status":      status,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
	
	return nil
}

// RecordWebhookInContext logs a webhook event
func (b *ContextBridge) RecordWebhookInContext(ctx context.Context, agentID string, adapterType string, eventType string, payload interface{}) (string, error) {
	// Generate a mock context ID
	contextID := fmt.Sprintf("mock-%s-%d", agentID, time.Now().Unix())
	
	// Log the webhook
	b.logger.Info("Webhook received", map[string]interface{}{
		"agentId":     agentID,
		"contextId":   contextID,
		"adapterType": adapterType,
		"eventType":   eventType,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
	
	// Emit event for the webhook
	if b.eventBus != nil {
		event := events.NewAdapterEvent(adapterType, events.EventTypeWebhookReceived, payload)
		event.WithMetadata("contextId", contextID)
		event.WithMetadata("eventType", eventType)
		
		b.eventBus.Emit(ctx, event)
	}
	
	return contextID, nil
}

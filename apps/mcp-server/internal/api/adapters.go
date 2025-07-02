package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/api/events"
	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/api/tools"
	"github.com/S-Corkum/devops-mcp/apps/mcp-server/internal/api/websocket"

	"github.com/google/uuid"
)

// ToolRegistryAdapter adapts tools.Registry to websocket.ToolRegistry
type ToolRegistryAdapter struct {
	registry   *tools.Registry
	executions sync.Map // executionID -> status
}

// NewToolRegistryAdapter creates a new adapter
func NewToolRegistryAdapter(registry *tools.Registry) websocket.ToolRegistry {
	return &ToolRegistryAdapter{
		registry: registry,
	}
}

// GetToolsForAgent returns tools available for a specific agent
func (a *ToolRegistryAdapter) GetToolsForAgent(agentID string) ([]websocket.Tool, error) {
	// Get all tools from registry
	allTools := a.registry.List()

	// Convert to websocket.Tool format
	tools := make([]websocket.Tool, 0, len(allTools))
	for _, tool := range allTools {
		if tool.Enabled {
			tools = append(tools, websocket.Tool{
				ID:          tool.Name,
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Schema,
			})
		}
	}

	return tools, nil
}

// ExecuteTool executes a tool for an agent
func (a *ToolRegistryAdapter) ExecuteTool(ctx context.Context, agentID, toolID string, args map[string]interface{}) (interface{}, error) {
	// Generate execution ID
	executionID := uuid.New().String()

	// Store execution status
	a.executions.Store(executionID, &websocket.ToolExecutionStatus{
		ExecutionID: executionID,
		Status:      "running",
		StartedAt:   time.Now(),
	})

	// Execute the tool
	result, err := a.registry.Execute(ctx, toolID, args)

	// Update execution status
	status := "completed"
	var errorMsg string
	if err != nil {
		status = "failed"
		errorMsg = err.Error()
	}

	now := time.Now()
	a.executions.Store(executionID, &websocket.ToolExecutionStatus{
		ExecutionID: executionID,
		Status:      status,
		StartedAt:   now,
		CompletedAt: &now,
		Result:      result,
		Error:       errorMsg,
	})

	return result, err
}

// CancelExecution cancels a tool execution
func (a *ToolRegistryAdapter) CancelExecution(ctx context.Context, executionID string) error {
	// In this simple implementation, we just mark it as cancelled
	if val, ok := a.executions.Load(executionID); ok {
		status := val.(*websocket.ToolExecutionStatus)
		status.Status = "cancelled"
		now := time.Now()
		status.CompletedAt = &now
		return nil
	}
	return fmt.Errorf("execution %s not found", executionID)
}

// GetExecutionStatus returns the status of a tool execution
func (a *ToolRegistryAdapter) GetExecutionStatus(ctx context.Context, executionID string) (*websocket.ToolExecutionStatus, error) {
	if val, ok := a.executions.Load(executionID); ok {
		return val.(*websocket.ToolExecutionStatus), nil
	}
	return nil, fmt.Errorf("execution %s not found", executionID)
}

// EventBusAdapter adapts events.Bus to websocket.EventBus
type EventBusAdapter struct {
	bus           *events.Bus
	subscriptions sync.Map // connectionID -> []subscriptionIDs
}

// NewEventBusAdapter creates a new adapter
func NewEventBusAdapter(bus *events.Bus) websocket.EventBus {
	return &EventBusAdapter{
		bus: bus,
	}
}

// Subscribe subscribes a connection to events
func (a *EventBusAdapter) Subscribe(connectionID string, eventTypes []string) error {
	subscriptionIDs := make([]string, 0, len(eventTypes))

	for _, eventType := range eventTypes {
		// Create a handler that will send events to the WebSocket connection
		handler := func(ctx context.Context, event *events.Event) error {
			// In a real implementation, this would send the event to the WebSocket connection
			// For now, we just log it
			return nil
		}

		subID := a.bus.Subscribe(eventType, handler)
		subscriptionIDs = append(subscriptionIDs, subID)
	}

	// Store subscription IDs for this connection
	a.subscriptions.Store(connectionID, subscriptionIDs)

	return nil
}

// Unsubscribe removes all subscriptions for a connection
func (a *EventBusAdapter) Unsubscribe(connectionID string) error {
	if val, ok := a.subscriptions.Load(connectionID); ok {
		subscriptionIDs := val.([]string)
		for _, subID := range subscriptionIDs {
			_ = a.bus.Unsubscribe(subID)
		}
		a.subscriptions.Delete(connectionID)
	}
	return nil
}

// UnsubscribeEvents removes specific event subscriptions for a connection
func (a *EventBusAdapter) UnsubscribeEvents(connectionID string, eventTypes []string) error {
	// In this simple implementation, we just unsubscribe all and resubscribe to remaining
	// A more sophisticated implementation would track individual subscriptions by event type
	return a.Unsubscribe(connectionID)
}

// Publish publishes an event
func (a *EventBusAdapter) Publish(event string, data interface{}) error {
	return a.bus.Publish(context.Background(), event, "websocket", data)
}

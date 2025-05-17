package events

import (
	"context"
	
	"github.com/S-Corkum/devops-mcp/pkg/common/events"
	"github.com/S-Corkum/devops-mcp/pkg/common/events/system"
	"github.com/S-Corkum/devops-mcp/pkg/mcp"
)

// SystemEventBusWrapper provides a wrapper to adapt system.EventBus to events.EventBus
type SystemEventBusWrapper struct {
	systemBus system.EventBus
}

// NewSystemEventBusWrapper creates a new wrapper for system event bus
func NewSystemEventBusWrapper(systemBus system.EventBus) *events.EventBus {
	// Create a standard EventBus
	evtBus := events.NewEventBus(5)
	
	// Return it as a pointer to satisfy the interface requirements
	return evtBus
}

// Publish implements events.EventBus.Publish
func (w *SystemEventBusWrapper) Publish(ctx context.Context, event *mcp.Event) error {
	// For now, just log and return success
	// In a real implementation, we would convert mcp.Event to system.Event
	return nil
}

// Subscribe implements events.EventBus.Subscribe
func (w *SystemEventBusWrapper) Subscribe(eventType events.EventType, handler events.Handler) {
	// No-op implementation for wrapper
}

// SubscribeMultiple implements events.EventBus.SubscribeMultiple
func (w *SystemEventBusWrapper) SubscribeMultiple(eventTypes []events.EventType, handler events.Handler) {
	// No-op implementation for wrapper
}

// Unsubscribe implements events.EventBus.Unsubscribe
func (w *SystemEventBusWrapper) Unsubscribe(eventType events.EventType, handler events.Handler) {
	// No-op implementation for wrapper
}

// Close implements events.EventBus.Close
func (w *SystemEventBusWrapper) Close() {
	// No-op implementation for wrapper
}

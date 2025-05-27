package events

// LegacyEvent represents an event published to the event bus
// This is kept for compatibility with older code
type LegacyEvent struct {
	Type string
	Data map[string]any
}

// LegacyEventBusImpl provides a simple event bus for publishing and subscribing to events
// This is kept for compatibility with older code and implements the LegacyEventBus interface
type LegacyEventBusImpl struct {
	// Implementation details
}

// Publish publishes an event to the event bus
func (b *LegacyEventBusImpl) Publish(event LegacyEvent) {
	// Implementation details
}

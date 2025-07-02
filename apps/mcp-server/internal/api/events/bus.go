package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/observability"
	"github.com/google/uuid"
)

// Event represents a system event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Handler is a function that processes events
type Handler func(ctx context.Context, event *Event) error

// Subscription represents an event subscription
type Subscription struct {
	ID       string
	Pattern  string
	Handler  Handler
	Filter   func(*Event) bool
	Priority int
}

// Bus is an event bus for publishing and subscribing to events
type Bus struct {
	subscriptions map[string][]*Subscription
	handlers      map[string][]Handler
	logger        observability.Logger
	metrics       observability.MetricsClient
	mu            sync.RWMutex

	// Event history for replay
	history      []*Event
	historySize  int
	historyMutex sync.RWMutex
}

// NewBus creates a new event bus
func NewBus(logger observability.Logger, metrics observability.MetricsClient) *Bus {
	return &Bus{
		subscriptions: make(map[string][]*Subscription),
		handlers:      make(map[string][]Handler),
		logger:        logger,
		metrics:       metrics,
		history:       make([]*Event, 0),
		historySize:   1000, // Keep last 1000 events
	}
}

// Publish publishes an event to all subscribers
func (b *Bus) Publish(ctx context.Context, eventType string, source string, data interface{}) error {
	event := &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    source,
		Timestamp: time.Now(),
		Data:      data,
		Metadata:  make(map[string]interface{}),
	}

	// Add to history
	b.addToHistory(event)

	// Record metrics
	if b.metrics != nil {
		b.metrics.IncrementCounter("events.published", 1)
		b.metrics.IncrementCounterWithLabels("events.by_type", 1, map[string]string{
			"type":   eventType,
			"source": source,
		})
	}

	// Get handlers for this event type
	b.mu.RLock()
	handlers := b.handlers[eventType]
	subscriptions := b.subscriptions[eventType]
	b.mu.RUnlock()

	// Execute direct handlers
	for _, handler := range handlers {
		go b.executeHandler(ctx, event, handler)
	}

	// Execute subscription handlers
	for _, sub := range subscriptions {
		if sub.Filter == nil || sub.Filter(event) {
			go b.executeHandler(ctx, event, sub.Handler)
		}
	}

	b.logger.Info("Event published", map[string]interface{}{
		"event_id":   event.ID,
		"event_type": eventType,
		"source":     source,
	})

	return nil
}

// Subscribe adds a handler for a specific event type
func (b *Bus) Subscribe(eventType string, handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscriptionID := uuid.New().String()
	subscription := &Subscription{
		ID:      subscriptionID,
		Pattern: eventType,
		Handler: handler,
	}

	b.subscriptions[eventType] = append(b.subscriptions[eventType], subscription)

	b.logger.Info("Event subscription created", map[string]interface{}{
		"subscription_id": subscriptionID,
		"event_type":      eventType,
	})

	return subscriptionID
}

// SubscribeWithFilter adds a handler with a filter function
func (b *Bus) SubscribeWithFilter(eventType string, handler Handler, filter func(*Event) bool) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscriptionID := uuid.New().String()
	subscription := &Subscription{
		ID:      subscriptionID,
		Pattern: eventType,
		Handler: handler,
		Filter:  filter,
	}

	b.subscriptions[eventType] = append(b.subscriptions[eventType], subscription)

	b.logger.Info("Filtered event subscription created", map[string]interface{}{
		"subscription_id": subscriptionID,
		"event_type":      eventType,
	})

	return subscriptionID
}

// Unsubscribe removes a subscription
func (b *Bus) Unsubscribe(subscriptionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	found := false
	for eventType, subs := range b.subscriptions {
		filtered := make([]*Subscription, 0, len(subs))
		for _, sub := range subs {
			if sub.ID != subscriptionID {
				filtered = append(filtered, sub)
			} else {
				found = true
			}
		}
		b.subscriptions[eventType] = filtered
	}

	if !found {
		return fmt.Errorf("subscription %s not found", subscriptionID)
	}

	b.logger.Info("Event subscription removed", map[string]interface{}{
		"subscription_id": subscriptionID,
	})

	return nil
}

// On is a simpler way to subscribe to events
func (b *Bus) On(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// GetHistory returns recent events
func (b *Bus) GetHistory(limit int) []*Event {
	b.historyMutex.RLock()
	defer b.historyMutex.RUnlock()

	if limit <= 0 || limit > len(b.history) {
		limit = len(b.history)
	}

	// Return the most recent events
	start := len(b.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*Event, limit)
	copy(result, b.history[start:])
	return result
}

// Replay replays events from history
func (b *Bus) Replay(ctx context.Context, filter func(*Event) bool, handler Handler) error {
	b.historyMutex.RLock()
	events := make([]*Event, 0)
	for _, event := range b.history {
		if filter == nil || filter(event) {
			events = append(events, event)
		}
	}
	b.historyMutex.RUnlock()

	for _, event := range events {
		if err := handler(ctx, event); err != nil {
			return fmt.Errorf("failed to replay event %s: %w", event.ID, err)
		}
	}

	return nil
}

// executeHandler runs a handler with error handling
func (b *Bus) executeHandler(ctx context.Context, event *Event, handler Handler) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("Event handler panicked", map[string]interface{}{
				"event_id":   event.ID,
				"event_type": event.Type,
				"panic":      r,
			})
			if b.metrics != nil {
				b.metrics.IncrementCounter("events.handler_errors", 1)
			}
		}
	}()

	start := time.Now()
	err := handler(ctx, event)
	duration := time.Since(start)

	if b.metrics != nil {
		b.metrics.RecordDuration("events.handler_duration", duration)
	}

	if err != nil {
		b.logger.Error("Event handler failed", map[string]interface{}{
			"event_id":   event.ID,
			"event_type": event.Type,
			"error":      err.Error(),
		})
		if b.metrics != nil {
			b.metrics.IncrementCounter("events.handler_errors", 1)
		}
	}
}

// addToHistory adds an event to the history buffer
func (b *Bus) addToHistory(event *Event) {
	b.historyMutex.Lock()
	defer b.historyMutex.Unlock()

	b.history = append(b.history, event)

	// Trim history if it exceeds the size limit
	if len(b.history) > b.historySize {
		b.history = b.history[len(b.history)-b.historySize:]
	}
}

// Clear removes all subscriptions and handlers
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscriptions = make(map[string][]*Subscription)
	b.handlers = make(map[string][]Handler)

	b.logger.Info("Event bus cleared", nil)
}

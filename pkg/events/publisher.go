package events

import (
	"context"
)

// Publisher defines the interface for publishing events
type Publisher interface {
	// Publish publishes an event
	Publish(ctx context.Context, event interface{}) error
	
	// PublishBatch publishes multiple events
	PublishBatch(ctx context.Context, events []interface{}) error
	
	// Close closes the publisher
	Close() error
}

// NoOpPublisher is a no-op implementation for testing
type NoOpPublisher struct{}

func (p *NoOpPublisher) Publish(ctx context.Context, event interface{}) error {
	return nil
}

func (p *NoOpPublisher) PublishBatch(ctx context.Context, events []interface{}) error {
	return nil
}

func (p *NoOpPublisher) Close() error {
	return nil
}
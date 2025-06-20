package events

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EventStore provides event persistence
type EventStore interface {
	Append(ctx context.Context, event *DomainEvent) error
	GetEvents(ctx context.Context, aggregateID uuid.UUID, fromVersion int) ([]*DomainEvent, error)
	GetEventsByType(ctx context.Context, eventType string, from time.Time) ([]*DomainEvent, error)
	CreateSnapshot(ctx context.Context, aggregateID uuid.UUID, snapshot interface{}) error
	GetSnapshot(ctx context.Context, aggregateID uuid.UUID) (interface{}, error)
}

// EventPublisher publishes events to subscribers
type EventPublisher interface {
	Publish(ctx context.Context, event *DomainEvent) error
	Subscribe(eventType string, handler EventHandler) error
	Unsubscribe(eventType string, handler EventHandler) error
}

// EventHandler handles domain events
type EventHandler func(ctx context.Context, event *DomainEvent) error

// DomainEvent represents a domain event
type DomainEvent struct {
	ID            uuid.UUID   `json:"id"`
	Type          string      `json:"type"`
	AggregateID   uuid.UUID   `json:"aggregate_id"`
	AggregateType string      `json:"aggregate_type"`
	Version       int         `json:"version"`
	Timestamp     time.Time   `json:"timestamp"`
	Data          interface{} `json:"data"`
	Metadata      Metadata    `json:"metadata"`
}

// Metadata contains event metadata
type Metadata struct {
	TenantID      uuid.UUID `json:"tenant_id"`
	UserID        string    `json:"user_id"`
	CorrelationID string    `json:"correlation_id"`
	CausationID   string    `json:"causation_id"`
}

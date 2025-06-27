package collaboration

import (
	"time"

	"github.com/google/uuid"
)

// DocumentOperation represents an operation on a document
type DocumentOperation struct {
	ID           uuid.UUID      `json:"id"`
	DocumentID   uuid.UUID      `json:"document_id"`
	Sequence     int64          `json:"sequence"`
	Type         string         `json:"type"` // insert, delete, replace, move
	Path         string         `json:"path"` // JSON path
	Value        interface{}    `json:"value,omitempty"`
	OldValue     interface{}    `json:"old_value,omitempty"`
	AgentID      string         `json:"agent_id"`
	VectorClock  map[string]int `json:"vector_clock"`
	AppliedAt    time.Time      `json:"applied_at"`
	Dependencies []uuid.UUID    `json:"dependencies,omitempty"`
}

// OperationType represents the type of operation
type OperationType string

const (
	OpInsert  OperationType = "insert"
	OpDelete  OperationType = "delete"
	OpReplace OperationType = "replace"
	OpMove    OperationType = "move"
)

// VectorClockManager manages vector clocks for distributed systems
type VectorClockManager struct{}

// CRDTEngine handles CRDT operations
type CRDTEngine struct {
	// Implementation details
}

// ClockComparison represents the result of comparing two vector clocks
type ClockComparison int

const (
	ClockEqual      ClockComparison = 0
	ClockBefore     ClockComparison = -1
	ClockAfter      ClockComparison = 1
	ClockConcurrent ClockComparison = 2
)

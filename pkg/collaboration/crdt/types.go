package crdt

import (
	"time"
	
	"github.com/google/uuid"
)

// NodeID represents a unique identifier for a node in the distributed system
type NodeID string

// VectorClock tracks causality in a distributed system
type VectorClock map[NodeID]uint64

// NewVectorClock creates a new vector clock
func NewVectorClock() VectorClock {
	return make(VectorClock)
}

// Increment increments the clock for a given node
func (vc VectorClock) Increment(nodeID NodeID) {
	vc[nodeID]++
}

// Update updates this vector clock with another, taking the maximum of each component
func (vc VectorClock) Update(other VectorClock) {
	for nodeID, clock := range other {
		if clock > vc[nodeID] {
			vc[nodeID] = clock
		}
	}
}

// HappensBefore returns true if this vector clock happens before the other
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	atLeastOneLess := false
	for nodeID, clock := range vc {
		if clock > other[nodeID] {
			return false
		}
		if clock < other[nodeID] {
			atLeastOneLess = true
		}
	}
	// Check for nodes in other that aren't in vc
	for nodeID, clock := range other {
		if _, exists := vc[nodeID]; !exists && clock > 0 {
			atLeastOneLess = true
		}
	}
	return atLeastOneLess
}

// Concurrent returns true if neither vector clock happens before the other
func (vc VectorClock) Concurrent(other VectorClock) bool {
	return !vc.HappensBefore(other) && !other.HappensBefore(vc)
}

// Clone creates a deep copy of the vector clock
func (vc VectorClock) Clone() VectorClock {
	clone := make(VectorClock)
	for k, v := range vc {
		clone[k] = v
	}
	return clone
}

// Timestamp represents a logical timestamp with a vector clock
type Timestamp struct {
	Clock VectorClock
	Node  NodeID
	Time  time.Time
}

// CRDT is the base interface for all CRDT types
type CRDT interface {
	// Merge combines this CRDT with another, resolving conflicts
	Merge(other CRDT) error
	
	// Clone creates a deep copy of the CRDT
	Clone() CRDT
	
	// GetType returns the type of CRDT
	GetType() string
}

// Operation represents a CRDT operation that can be applied
type Operation interface {
	// Apply applies the operation to a CRDT
	Apply(crdt CRDT) error
	
	// GetTimestamp returns the operation timestamp
	GetTimestamp() Timestamp
	
	// GetType returns the operation type
	GetType() string
}

// Delta represents a change that can be propagated
type Delta interface {
	// GetOperations returns the operations in this delta
	GetOperations() []Operation
	
	// Merge combines this delta with another
	Merge(other Delta) Delta
}

// DeltaCRDT supports delta-based synchronization
type DeltaCRDT interface {
	CRDT
	
	// GenerateDelta creates a delta since the given vector clock
	GenerateDelta(since VectorClock) Delta
	
	// ApplyDelta applies a delta to this CRDT
	ApplyDelta(delta Delta) error
}

// Metadata for CRDT operations
type Metadata struct {
	ID        uuid.UUID              `json:"id"`
	NodeID    NodeID                 `json:"node_id"`
	Clock     VectorClock            `json:"clock"`
	Timestamp time.Time              `json:"timestamp"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}
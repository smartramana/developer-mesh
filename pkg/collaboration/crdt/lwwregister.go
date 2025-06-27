package crdt

import (
	"fmt"
	"sync"
	"time"
)

// LWWRegister is a Last-Write-Wins Register CRDT
type LWWRegister struct {
	mu        sync.RWMutex
	value     interface{}
	timestamp time.Time
	nodeID    NodeID
}

// NewLWWRegister creates a new LWW-Register
func NewLWWRegister() *LWWRegister {
	return &LWWRegister{}
}

// Set sets the value with a timestamp
func (r *LWWRegister) Set(value interface{}, timestamp time.Time, nodeID NodeID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update if this write is newer, or same time but higher node ID (for consistency)
	if timestamp.After(r.timestamp) || (timestamp.Equal(r.timestamp) && nodeID > r.nodeID) {
		r.value = value
		r.timestamp = timestamp
		r.nodeID = nodeID
	}
}

// Get returns the current value
func (r *LWWRegister) Get() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.value
}

// GetWithMetadata returns the value with timestamp and node information
func (r *LWWRegister) GetWithMetadata() (interface{}, time.Time, NodeID) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.value, r.timestamp, r.nodeID
}

// Merge combines this register with another
func (r *LWWRegister) Merge(other CRDT) error {
	otherReg, ok := other.(*LWWRegister)
	if !ok {
		return fmt.Errorf("cannot merge LWWRegister with %T", other)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	otherReg.mu.RLock()
	defer otherReg.mu.RUnlock()

	// Apply LWW logic
	if otherReg.timestamp.After(r.timestamp) ||
		(otherReg.timestamp.Equal(r.timestamp) && otherReg.nodeID > r.nodeID) {
		r.value = otherReg.value
		r.timestamp = otherReg.timestamp
		r.nodeID = otherReg.nodeID
	}

	return nil
}

// Clone creates a deep copy of the register
func (r *LWWRegister) Clone() CRDT {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return &LWWRegister{
		value:     r.value,
		timestamp: r.timestamp,
		nodeID:    r.nodeID,
	}
}

// GetType returns the CRDT type
func (r *LWWRegister) GetType() string {
	return "LWWRegister"
}

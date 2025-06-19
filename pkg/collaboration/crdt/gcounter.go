package crdt

import (
	"fmt"
	"sync"
)

// GCounter is a grow-only counter CRDT
type GCounter struct {
	mu       sync.RWMutex
	counters map[NodeID]uint64
}

// NewGCounter creates a new G-Counter
func NewGCounter() *GCounter {
	return &GCounter{
		counters: make(map[NodeID]uint64),
	}
}

// Increment increments the counter for the given node
func (g *GCounter) Increment(nodeID NodeID, delta uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.counters[nodeID] += delta
}

// Value returns the total value of the counter
func (g *GCounter) Value() uint64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	var total uint64
	for _, count := range g.counters {
		total += count
	}
	return total
}

// Merge combines this counter with another
func (g *GCounter) Merge(other CRDT) error {
	otherCounter, ok := other.(*GCounter)
	if !ok {
		return fmt.Errorf("cannot merge GCounter with %T", other)
	}
	
	g.mu.Lock()
	defer g.mu.Unlock()
	
	otherCounter.mu.RLock()
	defer otherCounter.mu.RUnlock()
	
	// Take the maximum count for each node
	for nodeID, count := range otherCounter.counters {
		if count > g.counters[nodeID] {
			g.counters[nodeID] = count
		}
	}
	
	return nil
}

// Clone creates a deep copy of the counter
func (g *GCounter) Clone() CRDT {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	clone := NewGCounter()
	for nodeID, count := range g.counters {
		clone.counters[nodeID] = count
	}
	return clone
}

// GetType returns the CRDT type
func (g *GCounter) GetType() string {
	return "GCounter"
}

// GetState returns the internal state for debugging
func (g *GCounter) GetState() map[NodeID]uint64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	state := make(map[NodeID]uint64)
	for k, v := range g.counters {
		state[k] = v
	}
	return state
}
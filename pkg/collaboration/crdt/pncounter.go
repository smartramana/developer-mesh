package crdt

import (
	"fmt"
	"sync"
)

// PNCounter is a counter that supports both increment and decrement
type PNCounter struct {
	mu        sync.RWMutex
	positive  *GCounter
	negative  *GCounter
}

// NewPNCounter creates a new PN-Counter
func NewPNCounter() *PNCounter {
	return &PNCounter{
		positive: NewGCounter(),
		negative: NewGCounter(),
	}
}

// Increment increments the counter
func (p *PNCounter) Increment(nodeID NodeID, delta uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.positive.Increment(nodeID, delta)
}

// Decrement decrements the counter
func (p *PNCounter) Decrement(nodeID NodeID, delta uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.negative.Increment(nodeID, delta)
}

// Value returns the current value (positive - negative)
func (p *PNCounter) Value() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return int64(p.positive.Value()) - int64(p.negative.Value())
}

// Merge combines this counter with another
func (p *PNCounter) Merge(other CRDT) error {
	otherCounter, ok := other.(*PNCounter)
	if !ok {
		return fmt.Errorf("cannot merge PNCounter with %T", other)
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if err := p.positive.Merge(otherCounter.positive); err != nil {
		return fmt.Errorf("failed to merge positive counters: %w", err)
	}
	
	if err := p.negative.Merge(otherCounter.negative); err != nil {
		return fmt.Errorf("failed to merge negative counters: %w", err)
	}
	
	return nil
}

// Clone creates a deep copy of the counter
func (p *PNCounter) Clone() CRDT {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return &PNCounter{
		positive: p.positive.Clone().(*GCounter),
		negative: p.negative.Clone().(*GCounter),
	}
}

// GetType returns the CRDT type
func (p *PNCounter) GetType() string {
	return "PNCounter"
}
package collaboration

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/collaboration/crdt"
	"github.com/google/uuid"
)

// StateCRDT implements a CRDT for workspace state synchronization
type StateCRDT struct {
	mu      sync.RWMutex
	stateID uuid.UUID
	nodeID  crdt.NodeID
	clock   crdt.VectorClock

	// State fields using different CRDT types
	counters  map[string]*crdt.PNCounter   // For numeric values
	registers map[string]*crdt.LWWRegister // For single values
	sets      map[string]*crdt.ORSet       // For collections

	// Operation log for synchronization
	operations []StateOperation
}

// StateOperation represents an operation on the state
type StateOperation struct {
	ID        uuid.UUID        `json:"id"`
	Type      string           `json:"type"` // set, increment, decrement, add_to_set, remove_from_set
	Path      string           `json:"path"`
	Value     interface{}      `json:"value,omitempty"`
	Delta     int64            `json:"delta,omitempty"`
	NodeID    crdt.NodeID      `json:"node_id"`
	Clock     crdt.VectorClock `json:"clock"`
	Timestamp time.Time        `json:"timestamp"`
}

// NewStateCRDT creates a new state CRDT
func NewStateCRDT(stateID uuid.UUID, nodeID crdt.NodeID) *StateCRDT {
	return &StateCRDT{
		stateID:    stateID,
		nodeID:     nodeID,
		clock:      crdt.NewVectorClock(),
		counters:   make(map[string]*crdt.PNCounter),
		registers:  make(map[string]*crdt.LWWRegister),
		sets:       make(map[string]*crdt.ORSet),
		operations: []StateOperation{},
	}
}

// Set sets a value at the given path
func (s *StateCRDT) Set(path string, value interface{}) (*StateOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment clock
	s.clock.Increment(s.nodeID)

	// Create operation
	op := StateOperation{
		ID:        uuid.New(),
		Type:      "set",
		Path:      path,
		Value:     value,
		NodeID:    s.nodeID,
		Clock:     s.clock.Clone(),
		Timestamp: time.Now(),
	}

	// Apply to LWW register
	if s.registers[path] == nil {
		s.registers[path] = crdt.NewLWWRegister()
	}
	s.registers[path].Set(value, op.Timestamp, op.NodeID)

	s.operations = append(s.operations, op)
	return &op, nil
}

// Get retrieves a value at the given path
func (s *StateCRDT) Get(path string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check registers first
	if reg, exists := s.registers[path]; exists {
		value := reg.Get()
		return value, value != nil
	}

	// Check counters
	if counter, exists := s.counters[path]; exists {
		return counter.Value(), true
	}

	// Check sets
	if set, exists := s.sets[path]; exists {
		return set.Elements(), true
	}

	return nil, false
}

// IncrementCounter increments a counter at the given path
func (s *StateCRDT) IncrementCounter(path string, delta uint64) (*StateOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment clock
	s.clock.Increment(s.nodeID)

	// Create operation
	op := StateOperation{
		ID:        uuid.New(),
		Type:      "increment",
		Path:      path,
		Delta:     int64(delta),
		NodeID:    s.nodeID,
		Clock:     s.clock.Clone(),
		Timestamp: time.Now(),
	}

	// Apply to counter
	if s.counters[path] == nil {
		s.counters[path] = crdt.NewPNCounter()
	}
	s.counters[path].Increment(op.NodeID, delta)

	s.operations = append(s.operations, op)
	return &op, nil
}

// DecrementCounter decrements a counter at the given path
func (s *StateCRDT) DecrementCounter(path string, delta uint64) (*StateOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment clock
	s.clock.Increment(s.nodeID)

	// Create operation
	op := StateOperation{
		ID:        uuid.New(),
		Type:      "decrement",
		Path:      path,
		Delta:     int64(delta),
		NodeID:    s.nodeID,
		Clock:     s.clock.Clone(),
		Timestamp: time.Now(),
	}

	// Apply to counter
	if s.counters[path] == nil {
		s.counters[path] = crdt.NewPNCounter()
	}
	s.counters[path].Decrement(op.NodeID, delta)

	s.operations = append(s.operations, op)
	return &op, nil
}

// AddToSet adds an element to a set at the given path
func (s *StateCRDT) AddToSet(path string, element string) (*StateOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment clock
	s.clock.Increment(s.nodeID)

	// Create operation
	op := StateOperation{
		ID:        uuid.New(),
		Type:      "add_to_set",
		Path:      path,
		Value:     element,
		NodeID:    s.nodeID,
		Clock:     s.clock.Clone(),
		Timestamp: time.Now(),
	}

	// Apply to set
	if s.sets[path] == nil {
		s.sets[path] = crdt.NewORSet()
	}
	s.sets[path].Add(element)

	s.operations = append(s.operations, op)
	return &op, nil
}

// RemoveFromSet removes an element from a set at the given path
func (s *StateCRDT) RemoveFromSet(path string, element string) (*StateOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increment clock
	s.clock.Increment(s.nodeID)

	// Create operation
	op := StateOperation{
		ID:        uuid.New(),
		Type:      "remove_from_set",
		Path:      path,
		Value:     element,
		NodeID:    s.nodeID,
		Clock:     s.clock.Clone(),
		Timestamp: time.Now(),
	}

	// Apply to set
	if s.sets[path] != nil {
		s.sets[path].Remove(element)
	}

	s.operations = append(s.operations, op)
	return &op, nil
}

// ApplyOperation applies a remote operation
func (s *StateCRDT) ApplyOperation(op *StateOperation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update vector clock
	s.clock.Update(op.Clock)

	switch op.Type {
	case "set":
		if s.registers[op.Path] == nil {
			s.registers[op.Path] = crdt.NewLWWRegister()
		}
		s.registers[op.Path].Set(op.Value, op.Timestamp, op.NodeID)

	case "increment":
		if s.counters[op.Path] == nil {
			s.counters[op.Path] = crdt.NewPNCounter()
		}
		s.counters[op.Path].Increment(op.NodeID, uint64(op.Delta))

	case "decrement":
		if s.counters[op.Path] == nil {
			s.counters[op.Path] = crdt.NewPNCounter()
		}
		s.counters[op.Path].Decrement(op.NodeID, uint64(op.Delta))

	case "add_to_set":
		if s.sets[op.Path] == nil {
			s.sets[op.Path] = crdt.NewORSet()
		}
		if element, ok := op.Value.(string); ok {
			s.sets[op.Path].Add(element)
		}

	case "remove_from_set":
		if s.sets[op.Path] != nil {
			if element, ok := op.Value.(string); ok {
				s.sets[op.Path].Remove(element)
			}
		}

	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}

	s.operations = append(s.operations, *op)
	return nil
}

// GetState returns the complete state as a map
func (s *StateCRDT) GetState() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state := make(map[string]interface{})

	// Add registers
	for path, reg := range s.registers {
		if value := reg.Get(); value != nil {
			state[path] = value
		}
	}

	// Add counters
	for path, counter := range s.counters {
		state[path] = counter.Value()
	}

	// Add sets
	for path, set := range s.sets {
		state[path] = set.Elements()
	}

	return state
}

// Merge merges another state CRDT into this one
func (s *StateCRDT) Merge(other crdt.CRDT) error {
	otherState, ok := other.(*StateCRDT)
	if !ok {
		return fmt.Errorf("cannot merge StateCRDT with %T", other)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	otherState.mu.RLock()
	defer otherState.mu.RUnlock()

	// Merge registers
	for path, otherReg := range otherState.registers {
		if s.registers[path] == nil {
			s.registers[path] = crdt.NewLWWRegister()
		}
		if err := s.registers[path].Merge(otherReg); err != nil {
			return fmt.Errorf("failed to merge register at path %s: %w", path, err)
		}
	}

	// Merge counters
	for path, otherCounter := range otherState.counters {
		if s.counters[path] == nil {
			s.counters[path] = crdt.NewPNCounter()
		}
		if err := s.counters[path].Merge(otherCounter); err != nil {
			return fmt.Errorf("failed to merge counter at path %s: %w", path, err)
		}
	}

	// Merge sets
	for path, otherSet := range otherState.sets {
		if s.sets[path] == nil {
			s.sets[path] = crdt.NewORSet()
		}
		if err := s.sets[path].Merge(otherSet); err != nil {
			return fmt.Errorf("failed to merge set at path %s: %w", path, err)
		}
	}

	// Update clock
	s.clock.Update(otherState.clock)

	return nil
}

// Clone creates a deep copy
func (s *StateCRDT) Clone() crdt.CRDT {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := NewStateCRDT(s.stateID, s.nodeID)
	clone.clock = s.clock.Clone()

	// Clone registers
	for path, reg := range s.registers {
		clone.registers[path] = reg.Clone().(*crdt.LWWRegister)
	}

	// Clone counters
	for path, counter := range s.counters {
		clone.counters[path] = counter.Clone().(*crdt.PNCounter)
	}

	// Clone sets
	for path, set := range s.sets {
		clone.sets[path] = set.Clone().(*crdt.ORSet)
	}

	// Clone operations
	clone.operations = make([]StateOperation, len(s.operations))
	copy(clone.operations, s.operations)

	return clone
}

// GetType returns the CRDT type
func (s *StateCRDT) GetType() string {
	return "StateCRDT"
}

// ToJSON serializes the state to JSON
func (s *StateCRDT) ToJSON() ([]byte, error) {
	state := s.GetState()
	return json.Marshal(state)
}

// GetOperationsSince returns operations since the given vector clock
func (s *StateCRDT) GetOperationsSince(since crdt.VectorClock) []StateOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ops []StateOperation
	for _, op := range s.operations {
		if !since.HappensBefore(op.Clock) && !since.Concurrent(op.Clock) {
			continue
		}
		ops = append(ops, op)
	}

	return ops
}

package crdt

import (
	"fmt"
	"sync"
	
	"github.com/google/uuid"
)

// ORSet is an Observed-Remove Set CRDT
type ORSet struct {
	mu       sync.RWMutex
	elements map[string]map[uuid.UUID]bool // element -> unique tags
}

// NewORSet creates a new OR-Set
func NewORSet() *ORSet {
	return &ORSet{
		elements: make(map[string]map[uuid.UUID]bool),
	}
}

// Add adds an element to the set
func (s *ORSet) Add(element string) uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	tag := uuid.New()
	
	if s.elements[element] == nil {
		s.elements[element] = make(map[uuid.UUID]bool)
	}
	s.elements[element][tag] = true
	
	return tag
}

// Remove removes an element from the set
func (s *ORSet) Remove(element string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Remove all tags for this element
	delete(s.elements, element)
}

// Contains checks if an element is in the set
func (s *ORSet) Contains(element string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	tags, exists := s.elements[element]
	return exists && len(tags) > 0
}

// Elements returns all elements in the set
func (s *ORSet) Elements() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []string
	for element, tags := range s.elements {
		if len(tags) > 0 {
			result = append(result, element)
		}
	}
	return result
}

// Merge combines this set with another
func (s *ORSet) Merge(other CRDT) error {
	otherSet, ok := other.(*ORSet)
	if !ok {
		return fmt.Errorf("cannot merge ORSet with %T", other)
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	otherSet.mu.RLock()
	defer otherSet.mu.RUnlock()
	
	// Union of all tags for each element
	for element, otherTags := range otherSet.elements {
		if s.elements[element] == nil {
			s.elements[element] = make(map[uuid.UUID]bool)
		}
		for tag := range otherTags {
			s.elements[element][tag] = true
		}
	}
	
	return nil
}

// Clone creates a deep copy of the set
func (s *ORSet) Clone() CRDT {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	clone := NewORSet()
	for element, tags := range s.elements {
		clone.elements[element] = make(map[uuid.UUID]bool)
		for tag := range tags {
			clone.elements[element][tag] = true
		}
	}
	return clone
}

// GetType returns the CRDT type
func (s *ORSet) GetType() string {
	return "ORSet"
}

// Size returns the number of elements in the set
func (s *ORSet) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	count := 0
	for _, tags := range s.elements {
		if len(tags) > 0 {
			count++
		}
	}
	return count
}
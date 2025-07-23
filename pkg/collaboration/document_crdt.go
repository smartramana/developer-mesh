package collaboration

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/collaboration/crdt"
	"github.com/google/uuid"
)

// CRDTOperation represents an operation on a collaborative document
type CRDTOperation struct {
	ID        uuid.UUID              `json:"id"`
	Type      string                 `json:"type"` // insert, delete, format
	Position  int                    `json:"position"`
	Content   string                 `json:"content,omitempty"`
	Length    int                    `json:"length,omitempty"`
	NodeID    crdt.NodeID            `json:"node_id"`
	Clock     crdt.VectorClock       `json:"clock"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentCRDT implements a CRDT for collaborative document editing
type DocumentCRDT struct {
	mu         sync.RWMutex
	documentID uuid.UUID
	operations map[uuid.UUID]*CRDTOperation
	clock      crdt.VectorClock
	nodeID     crdt.NodeID

	// Tombstones for deleted content
	tombstones map[uuid.UUID]bool

	// Character positions with unique IDs for ordering
	characters []CharacterNode
}

// CharacterNode represents a character in the document with ordering information
type CharacterNode struct {
	ID        uuid.UUID
	Character rune
	Position  float64 // Fractional indexing for insertion between characters
	Deleted   bool
	NodeID    crdt.NodeID
	Clock     crdt.VectorClock
}

// NewDocumentCRDT creates a new document CRDT
func NewDocumentCRDT(documentID uuid.UUID, nodeID crdt.NodeID) *DocumentCRDT {
	return &DocumentCRDT{
		documentID: documentID,
		operations: make(map[uuid.UUID]*CRDTOperation),
		clock:      crdt.NewVectorClock(),
		nodeID:     nodeID,
		tombstones: make(map[uuid.UUID]bool),
		characters: []CharacterNode{},
	}
}

// Insert inserts text at the specified position
func (d *DocumentCRDT) Insert(position int, content string) (*CRDTOperation, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Increment vector clock
	d.clock.Increment(d.nodeID)

	// Create operation
	op := &CRDTOperation{
		ID:        uuid.New(),
		Type:      "insert",
		Position:  position,
		Content:   content,
		NodeID:    d.nodeID,
		Clock:     d.clock.Clone(),
		Timestamp: time.Now(),
	}

	// Apply operation locally
	if err := d.applyInsert(op); err != nil {
		return nil, err
	}

	d.operations[op.ID] = op
	return op, nil
}

// Delete deletes text at the specified position
func (d *DocumentCRDT) Delete(position int, length int) (*CRDTOperation, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Increment vector clock
	d.clock.Increment(d.nodeID)

	// Create operation
	op := &CRDTOperation{
		ID:        uuid.New(),
		Type:      "delete",
		Position:  position,
		Length:    length,
		NodeID:    d.nodeID,
		Clock:     d.clock.Clone(),
		Timestamp: time.Now(),
	}

	// Apply operation locally
	if err := d.applyDelete(op); err != nil {
		return nil, err
	}

	d.operations[op.ID] = op
	return op, nil
}

// ApplyOperation applies a remote operation
func (d *DocumentCRDT) ApplyOperation(op *CRDTOperation) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if we've already seen this operation
	if _, exists := d.operations[op.ID]; exists {
		return nil // Idempotent
	}

	// Update vector clock
	d.clock.Update(op.Clock)

	// Apply based on operation type
	var err error
	switch op.Type {
	case "insert":
		err = d.applyInsert(op)
	case "delete":
		err = d.applyDelete(op)
	default:
		err = fmt.Errorf("unknown operation type: %s", op.Type)
	}

	if err != nil {
		return err
	}

	d.operations[op.ID] = op
	return nil
}

// GetContent returns the current document content
func (d *DocumentCRDT) GetContent() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Sort characters by position
	chars := make([]CharacterNode, 0, len(d.characters))
	for _, char := range d.characters {
		if !char.Deleted {
			chars = append(chars, char)
		}
	}

	sort.Slice(chars, func(i, j int) bool {
		return chars[i].Position < chars[j].Position
	})

	// Build string
	result := make([]rune, len(chars))
	for i, char := range chars {
		result[i] = char.Character
	}

	return string(result)
}

// Merge merges another document CRDT into this one
func (d *DocumentCRDT) Merge(other crdt.CRDT) error {
	otherDoc, ok := other.(*DocumentCRDT)
	if !ok {
		return fmt.Errorf("cannot merge DocumentCRDT with %T", other)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	otherDoc.mu.RLock()
	defer otherDoc.mu.RUnlock()

	// Apply all operations we haven't seen
	for opID, op := range otherDoc.operations {
		if _, exists := d.operations[opID]; !exists {
			// Make a copy of the operation
			opCopy := *op
			if err := d.ApplyOperation(&opCopy); err != nil {
				return fmt.Errorf("failed to apply operation %s: %w", opID, err)
			}
		}
	}

	return nil
}

// Clone creates a deep copy of the document CRDT
func (d *DocumentCRDT) Clone() crdt.CRDT {
	d.mu.RLock()
	defer d.mu.RUnlock()

	clone := NewDocumentCRDT(d.documentID, d.nodeID)

	// Copy operations
	for id, op := range d.operations {
		opCopy := *op
		clone.operations[id] = &opCopy
	}

	// Copy clock
	clone.clock = d.clock.Clone()

	// Copy characters
	clone.characters = make([]CharacterNode, len(d.characters))
	copy(clone.characters, d.characters)

	// Copy tombstones
	for id := range d.tombstones {
		clone.tombstones[id] = true
	}

	return clone
}

// GetType returns the CRDT type
func (d *DocumentCRDT) GetType() string {
	return "DocumentCRDT"
}

// Internal methods

func (d *DocumentCRDT) applyInsert(op *CRDTOperation) error {
	// Calculate fractional position for the new characters
	startPos := d.calculatePosition(op.Position)

	// Insert each character
	for i, char := range op.Content {
		charNode := CharacterNode{
			ID:        uuid.New(),
			Character: char,
			Position:  startPos + float64(i)*0.00001, // Small increment for ordering
			Deleted:   false,
			NodeID:    op.NodeID,
			Clock:     op.Clock.Clone(),
		}
		d.characters = append(d.characters, charNode)
	}

	return nil
}

func (d *DocumentCRDT) applyDelete(op *CRDTOperation) error {
	// Mark characters as deleted (tombstone approach)
	visibleChars := d.getVisibleCharacters()

	if op.Position >= len(visibleChars) {
		return fmt.Errorf("delete position %d out of bounds", op.Position)
	}

	endPos := op.Position + op.Length
	if endPos > len(visibleChars) {
		endPos = len(visibleChars)
	}

	// Mark characters as deleted
	for i := op.Position; i < endPos; i++ {
		for j := range d.characters {
			if d.characters[j].ID == visibleChars[i].ID {
				d.characters[j].Deleted = true
				d.tombstones[d.characters[j].ID] = true
				break
			}
		}
	}

	return nil
}

func (d *DocumentCRDT) calculatePosition(index int) float64 {
	visibleChars := d.getVisibleCharacters()

	if index <= 0 {
		return 0.0
	}

	if index >= len(visibleChars) {
		if len(visibleChars) == 0 {
			return 1.0
		}
		return visibleChars[len(visibleChars)-1].Position + 1.0
	}

	// Insert between two characters
	prevPos := visibleChars[index-1].Position
	nextPos := visibleChars[index].Position
	return (prevPos + nextPos) / 2.0
}

func (d *DocumentCRDT) getVisibleCharacters() []CharacterNode {
	var visible []CharacterNode
	for _, char := range d.characters {
		if !char.Deleted {
			visible = append(visible, char)
		}
	}

	sort.Slice(visible, func(i, j int) bool {
		return visible[i].Position < visible[j].Position
	})

	return visible
}

// GetOperationsSince returns all operations since the given vector clock
func (d *DocumentCRDT) GetOperationsSince(since crdt.VectorClock) []*CRDTOperation {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var ops []*CRDTOperation
	for _, op := range d.operations {
		if !since.HappensBefore(op.Clock) && !since.Concurrent(op.Clock) {
			continue
		}
		ops = append(ops, op)
	}

	// Sort by timestamp for consistent ordering
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Timestamp.Before(ops[j].Timestamp)
	})

	return ops
}

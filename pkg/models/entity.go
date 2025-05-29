// This file contains legacy entity methods for backward compatibility.
// The primary entity type definitions are now in relationships.go
package models

import (
	"fmt"
)

// String returns a string representation of the entity ID
func (e EntityID) String() string {
	return fmt.Sprintf("%s:%s/%s:%s", e.Type, e.Owner, e.Repo, e.ID)
}

// WithContext adds context to the relationship with more flexible parameters
// It can be called with either a key-value pair or a complete map
func (r *EntityRelationship) WithContextMap(keyOrMap any, value ...any) *EntityRelationship {
	// Convert Context from string to map if needed
	ctxMap, ok := r.Metadata["context"].(map[string]any)
	if !ok {
		ctxMap = make(map[string]any)
		r.Metadata["context"] = ctxMap
	}

	// Check if we're adding a complete map
	if m, ok := keyOrMap.(map[string]any); ok && len(value) == 0 {
		for k, v := range m {
			ctxMap[k] = v
		}
		return r
	}

	// Otherwise treat it as a key-value pair
	if key, ok := keyOrMap.(string); ok && len(value) > 0 {
		ctxMap[key] = value[0]
	}
	return r
}

// WithMetadataMap adds metadata to the relationship with more flexible parameters
// It can be called with either a key-value pair or a complete map
func (r *EntityRelationship) WithMetadataMap(keyOrMap any, value ...any) *EntityRelationship {
	if r.Metadata == nil {
		r.Metadata = make(map[string]any)
	}

	// Check if we're adding a complete map
	if m, ok := keyOrMap.(map[string]any); ok && len(value) == 0 {
		for k, v := range m {
			r.Metadata[k] = v
		}
		return r
	}

	// Otherwise treat it as a key-value pair
	if key, ok := keyOrMap.(string); ok && len(value) > 0 {
		r.Metadata[key] = value[0]
	}
	return r
}

// LegacyGenerateRelationshipID generates a unique ID for a relationship
// Takes either (source, target, relType) or (relType, source, target, direction) - the latter ignores direction
// This is a legacy implementation for backward compatibility with existing code
func LegacyGenerateRelationshipID(arg1 any, arg2 any, arg3 any, arg4 ...any) string {
	var source EntityID
	var target EntityID
	var relType RelationshipType

	// Handle the original signature: (source, target, relType)
	if src, ok := arg1.(EntityID); ok {
		if tgt, ok := arg2.(EntityID); ok {
			if rt, ok := arg3.(RelationshipType); ok {
				source = src
				target = tgt
				relType = rt
			}
		}
	}

	// Handle alternative signature: (relType, source, target, direction)
	if rt, ok := arg1.(RelationshipType); ok {
		if src, ok := arg2.(EntityID); ok {
			if tgt, ok := arg3.(EntityID); ok {
				source = src
				target = tgt
				relType = rt
			}
		}
	}

	// Use simple format for legacy ID generation
	return fmt.Sprintf("%s-%s-%s", source, relType, target)
}

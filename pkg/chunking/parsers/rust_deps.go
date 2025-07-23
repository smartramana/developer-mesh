package parsers

import (
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/chunking"
)

// processDependencies analyzes relationships between chunks and adds dependency information
func (p *RustParser) processDependencies(chunks []*chunking.CodeChunk) {
	// Create maps for quick lookups
	chunksByID := make(map[string]*chunking.CodeChunk)
	typesByName := make(map[string][]string) // map of type name -> list of chunk IDs

	// First pass: build lookup maps
	for _, chunk := range chunks {
		chunksByID[chunk.ID] = chunk

		// Index types (structs, enums, traits) by name for dependency lookups
		switch chunk.Type {
		case chunking.ChunkTypeStruct, chunking.ChunkTypeBlock, chunking.ChunkTypeInterface:
			// Only add if this is a named type (struct, enum, trait)
			meta := chunk.Metadata
			if typeStr, ok := meta["type"].(string); ok {
				if typeStr == "struct" || typeStr == "enum" || typeStr == "trait" {
					typesByName[chunk.Name] = append(typesByName[chunk.Name], chunk.ID)
				}
			}
		}
	}

	// Second pass: analyze dependencies
	for _, chunk := range chunks {
		// Initialize dependencies list if needed
		if chunk.Dependencies == nil {
			chunk.Dependencies = make([]string, 0)
		}

		// Skip certain chunk types that don't usually have dependencies
		if chunk.Type == chunking.ChunkTypeComment || chunk.Type == chunking.ChunkTypeFile {
			continue
		}

		// Check for dependencies based on the chunk's content
		content := chunk.Content

		// Function dependencies
		if chunk.Type == chunking.ChunkTypeFunction {
			// Check function body for references to other functions or types
			for otherChunk := range chunksByID {
				other := chunksByID[otherChunk]

				// Skip self-references and non-callable items
				if other.ID == chunk.ID ||
					other.Type == chunking.ChunkTypeComment ||
					other.Type == chunking.ChunkTypeFile ||
					other.Type == chunking.ChunkTypeImport {
					continue
				}

				// Look for references to other function names
				if other.Type == chunking.ChunkTypeFunction {
					// Check for function calls like "other_func(..." or "other_func::<...>("
					if strings.Contains(content, other.Name+"(") ||
						strings.Contains(content, other.Name+"::") {
						// Add dependency if not already present
						if !containsString(chunk.Dependencies, other.ID) {
							chunk.Dependencies = append(chunk.Dependencies, other.ID)
						}
					}
				}
			}

			// Check for type references in function body
			for typeName, chunkIDs := range typesByName {
				// Skip simple/common type names to avoid false positives
				if len(typeName) <= 2 || typeName == "self" || typeName == "type" {
					continue
				}

				// Check for type references with word boundaries
				if containsTypeReference(content, typeName) {
					for _, typeChunkID := range chunkIDs {
						// Add dependency if not already present
						if !containsString(chunk.Dependencies, typeChunkID) {
							chunk.Dependencies = append(chunk.Dependencies, typeChunkID)
						}
					}
				}
			}
		}

		// Struct and enum dependencies on other types
		if chunk.Type == chunking.ChunkTypeStruct ||
			(chunk.Type == chunking.ChunkTypeBlock && isEnumOrTrait(chunk)) {

			for typeName, chunkIDs := range typesByName {
				// Skip self-reference
				if typeName == chunk.Name || len(typeName) <= 2 {
					continue
				}

				// Check for type references with word boundaries
				if containsTypeReference(content, typeName) {
					for _, typeChunkID := range chunkIDs {
						// Add dependency if not already present
						if !containsString(chunk.Dependencies, typeChunkID) {
							chunk.Dependencies = append(chunk.Dependencies, typeChunkID)
						}
					}
				}
			}
		}

		// Implementation blocks depend on the type they implement for
		if chunk.Type == chunking.ChunkTypeBlock {
			meta := chunk.Metadata
			if typeStr, ok := meta["type"].(string); ok && typeStr == "implementation" {
				// For trait impls, add dependencies on both the trait and the type
				if implType, ok := meta["impl_type"].(string); ok && implType == "trait" {
					if traitName, ok := meta["trait"].(string); ok {
						for _, traitChunkID := range typesByName[traitName] {
							if !containsString(chunk.Dependencies, traitChunkID) {
								chunk.Dependencies = append(chunk.Dependencies, traitChunkID)
							}
						}
					}
					// Add dependency on the implemented type
					if forType, ok := meta["for_type"].(string); ok {
						for _, typeChunkID := range typesByName[forType] {
							if !containsString(chunk.Dependencies, typeChunkID) {
								chunk.Dependencies = append(chunk.Dependencies, typeChunkID)
							}
						}
					}
				}
			}
		}
	}
}

// Helper function to check if a chunk is an enum or trait
func isEnumOrTrait(chunk *chunking.CodeChunk) bool {
	meta := chunk.Metadata
	if typeStr, ok := meta["type"].(string); ok {
		return typeStr == "enum" || typeStr == "trait"
	}
	return false
}

// Helper function to check if a string contains a reference to a type
func containsTypeReference(content, typeName string) bool {
	// Look for the type name with word boundaries to avoid false positives
	possiblePatterns := []string{
		typeName + "::", // qualified access
		typeName + "<",  // generic instantiation
		typeName + " ",  // type followed by space
		" " + typeName,  // type preceded by space
		"<" + typeName,  // type in generic bounds
		">" + typeName,  // type after generic closing
		"(" + typeName,  // type in function args
		")" + typeName,  // type after parenthesis
		"[" + typeName,  // type in array
		"]" + typeName,  // type after array
		":" + typeName,  // type in type annotation
		"&" + typeName,  // reference to type
	}

	for _, pattern := range possiblePatterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}

	return false
}

// Note: containsString function is now used from utils.go

package parsers

import (
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// processDependencies analyzes relationships between chunks and adds dependency information
func (p *KotlinParser) processDependencies(chunks []*chunking.CodeChunk) {
	// Create maps for quick lookups
	chunksByID := make(map[string]*chunking.CodeChunk)
	typesByName := make(map[string][]string) // map of type name -> list of chunk IDs

	// First pass: build lookup maps
	for _, chunk := range chunks {
		chunksByID[chunk.ID] = chunk

		// Index types (classes, interfaces, objects) by name for dependency lookups
		switch chunk.Type {
		case chunking.ChunkTypeClass, chunking.ChunkTypeInterface, chunking.ChunkTypeBlock:
			// Only add if this is a named type
			meta := chunk.Metadata
			if typeStr, ok := meta["type"].(string); ok {
				if typeStr == "class" || typeStr == "interface" || typeStr == "object" {
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

		// Get chunk content for analysis
		content := chunk.Content
		meta := chunk.Metadata

		// Handle class dependencies (superclasses, interfaces)
		if chunk.Type == chunking.ChunkTypeClass {
			if superTypes, ok := meta["super_types"].([]string); ok {
				for _, superType := range superTypes {
					if superTypeIDs, exists := typesByName[superType]; exists {
						for _, superTypeID := range superTypeIDs {
							if !containsString(chunk.Dependencies, superTypeID) {
								chunk.Dependencies = append(chunk.Dependencies, superTypeID)
							}
						}
					}
				}
			}
		}

		// Handle interface dependencies
		if chunk.Type == chunking.ChunkTypeInterface {
			if superInterfaces, ok := meta["super_interfaces"].([]string); ok {
				for _, superInterface := range superInterfaces {
					if superInterfaceIDs, exists := typesByName[superInterface]; exists {
						for _, superInterfaceID := range superInterfaceIDs {
							if !containsString(chunk.Dependencies, superInterfaceID) {
								chunk.Dependencies = append(chunk.Dependencies, superInterfaceID)
							}
						}
					}
				}
			}
		}

		// Handle function dependencies
		if chunk.Type == chunking.ChunkTypeFunction {
			// Check for return type dependencies
			if returnType, ok := meta["return_type"].(string); ok {
				// Extract just the type name without generics
				if strings.Contains(returnType, "<") {
					returnType = returnType[:strings.Index(returnType, "<")]
				}

				// Remove any nullable marker (?)
				returnType = strings.TrimSuffix(returnType, "?")

				// Check if this return type exists as a known type
				if returnTypeIDs, exists := typesByName[returnType]; exists {
					for _, returnTypeID := range returnTypeIDs {
						if !containsString(chunk.Dependencies, returnTypeID) {
							chunk.Dependencies = append(chunk.Dependencies, returnTypeID)
						}
					}
				}
			}

			// Check for parameter type dependencies
			if params, ok := meta["parameters"].([]map[string]string); ok {
				for _, param := range params {
					if paramType, ok := param["type"]; ok {
						// Extract base type without generics
						baseType := paramType
						if strings.Contains(baseType, "<") {
							baseType = baseType[:strings.Index(baseType, "<")]
						}

						// Remove any nullable marker (?)
						baseType = strings.TrimSuffix(baseType, "?")

						// Check if this parameter type exists as a known type
						if paramTypeIDs, exists := typesByName[baseType]; exists {
							for _, paramTypeID := range paramTypeIDs {
								if !containsString(chunk.Dependencies, paramTypeID) {
									chunk.Dependencies = append(chunk.Dependencies, paramTypeID)
								}
							}
						}
					}
				}
			}

			// For extension functions, add dependency on the receiver type
			if typeStr, ok := meta["type"].(string); ok && typeStr == "extension_function" {
				if receiverType, ok := meta["receiver_type"].(string); ok {
					if receiverTypeIDs, exists := typesByName[receiverType]; exists {
						for _, receiverTypeID := range receiverTypeIDs {
							if !containsString(chunk.Dependencies, receiverTypeID) {
								chunk.Dependencies = append(chunk.Dependencies, receiverTypeID)
							}
						}
					}
				}
			}

			// Check for function calls to other functions
			for otherChunkID, otherChunk := range chunksByID {
				// Skip self-references and non-callable items
				if otherChunkID == chunk.ID ||
					otherChunk.Type == chunking.ChunkTypeComment ||
					otherChunk.Type == chunking.ChunkTypeFile ||
					otherChunk.Type == chunking.ChunkTypeImport {
					continue
				}

				// Only check function calls to other functions
				if otherChunk.Type == chunking.ChunkTypeFunction {
					// Look for function calls like "otherFunc(" or "otherFunc "
					if strings.Contains(content, otherChunk.Name+"(") ||
						strings.Contains(content, otherChunk.Name+" ") {
						// Add dependency if not already present
						if !containsString(chunk.Dependencies, otherChunkID) {
							chunk.Dependencies = append(chunk.Dependencies, otherChunkID)
						}
					}
				}
			}
		}

		// Handle property dependencies
		if chunk.Type == chunking.ChunkTypeBlock {
			if typeStr, ok := meta["type"].(string); ok && typeStr == "property" {
				// Check for property type dependencies
				if propType, ok := meta["property_type"].(string); ok {
					// Extract base type without generics
					baseType := propType
					if strings.Contains(baseType, "<") {
						baseType = baseType[:strings.Index(baseType, "<")]
					}

					// Remove any nullable marker (?)
					baseType = strings.TrimSuffix(baseType, "?")

					// Check if this property type exists as a known type
					if propTypeIDs, exists := typesByName[baseType]; exists {
						for _, propTypeID := range propTypeIDs {
							if !containsString(chunk.Dependencies, propTypeID) {
								chunk.Dependencies = append(chunk.Dependencies, propTypeID)
							}
						}
					}
				}
			}
		}
	}
}

// Note: containsString function is now used from utils.go

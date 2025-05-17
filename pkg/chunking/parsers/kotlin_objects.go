package parsers

import (
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractObjects extracts object declarations from Kotlin code
func (p *KotlinParser) extractObjects(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all object declarations
	objectMatches := kotlinObjectRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range objectMatches {
		if len(match) < 2 {
			continue
		}
		
		// Determine if this is a named object
		var objectName string
		if len(match) >= 4 && match[2] != -1 && match[3] != -1 {
			objectName = code[match[2]:match[3]]
		} else {
			// This is likely a companion object without a name
			objectName = "companion"
		}
		
		// Find the object content
		startPos := match[0]
		objectContent := code[startPos:match[1]]
		endPos := match[1]
		
		// Check for the opening brace and get full object body if found
		if strings.Contains(objectContent, "{") {
			objectContent, endPos = p.findBlockContent(code, startPos)
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Extract modifiers
		objectDeclLine := objectContent
		if idx := strings.Index(objectContent, "{"); idx != -1 {
			objectDeclLine = objectContent[:idx]
		}
		
		// Check if this is a companion object
		isCompanion := strings.Contains(objectDeclLine, "companion object")
		
		// Determine visibility
		visibility := "public" // default in Kotlin
		if strings.Contains(objectDeclLine, "private object") {
			visibility = "private"
		} else if strings.Contains(objectDeclLine, "protected object") {
			visibility = "protected"
		} else if strings.Contains(objectDeclLine, "internal object") {
			visibility = "internal"
		}
		
		// Extract super-interfaces or classes if present
		superTypes := []string{}
		if strings.Contains(objectDeclLine, ":") {
			parts := strings.Split(objectDeclLine, ":")
			if len(parts) > 1 {
				// Process part after colon to extract super-interfaces and classes
				superTypesPart := parts[1]
				
				// Find position of the opening brace or end of line
				end := len(superTypesPart)
				if bracePos := strings.Index(superTypesPart, "{"); bracePos != -1 {
					end = bracePos
				}
				superTypesPart = superTypesPart[:end]
				
				// Split by commas to get individual super-types
				for _, superType := range strings.Split(superTypesPart, ",") {
					superType = strings.TrimSpace(superType)
					
					// Remove constructor part if present
					if parenPos := strings.Index(superType, "("); parenPos != -1 {
						superType = superType[:parenPos]
					}
					
					// Remove generic part if present
					if anglePos := strings.Index(superType, "<"); anglePos != -1 {
						superType = superType[:anglePos]
					}
					
					if superType != "" {
						superTypes = append(superTypes, superType)
					}
				}
			}
		}
		
		// Create object metadata
		objectMetadata := map[string]interface{}{
			"type":        "object",
			"visibility":  visibility,
			"is_companion": isCompanion,
		}
		
		if len(superTypes) > 0 {
			objectMetadata["super_types"] = superTypes
		}
		
		// Create path based on object type
		objectPath := fmt.Sprintf("object:%s", objectName)
		if isCompanion {
			objectPath = fmt.Sprintf("companion:%s", objectName)
		}
		
		// Create object chunk
		objectChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block since there's no specific Object type
			Name:      objectName,
			Path:      objectPath,
			Content:   objectContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  objectMetadata,
		}
		objectChunk.ID = generateKotlinChunkID(objectChunk)
		chunks = append(chunks, objectChunk)
		
		// If the object has a body, we could extract nested elements here
		// Similar to what we'd do in classes
	}
	
	return chunks
}

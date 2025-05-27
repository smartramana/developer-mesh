package parsers

import (
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractProperties extracts property declarations from Kotlin code
func (p *KotlinParser) extractProperties(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all property declarations (val and var)
	propertyMatches := kotlinPropertyRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range propertyMatches {
		if len(match) < 4 {
			continue
		}

		// Get the property name
		propName := code[match[2]:match[3]]

		// Find the property content
		startPos := match[0]
		endPos := match[1]

		// Get the line with property declaration
		propLine := code[startPos:endPos]

		// Check if property has a getter/setter block
		if strings.Contains(propLine, "{") {
			// Find the full property definition including getter/setter blocks
			propLine, endPos = p.findBlockContent(code, startPos)
		} else {
			// If no block, find the end of the statement (;)
			semiPos := strings.Index(code[startPos:], ";")
			if semiPos != -1 {
				endPos = startPos + semiPos + 1
				propLine = code[startPos:endPos]
			} else {
				// If no semicolon, assume the property ends at the end of the line
				nlPos := strings.Index(code[startPos:], "\n")
				if nlPos != -1 {
					endPos = startPos + nlPos
					propLine = code[startPos:endPos]
				}
			}
		}

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Determine property type (val or var)
		isVal := false
		isVar := false
		if strings.Contains(propLine, "val ") {
			isVal = true
		} else if strings.Contains(propLine, "var ") {
			isVar = true
		}

		// Determine visibility
		visibility := "public" // default in Kotlin
		if strings.Contains(propLine, "private ") {
			visibility = "private"
		} else if strings.Contains(propLine, "protected ") {
			visibility = "protected"
		} else if strings.Contains(propLine, "internal ") {
			visibility = "internal"
		}

		// Check for special modifiers
		isLateinit := strings.Contains(propLine, "lateinit ")
		isConst := strings.Contains(propLine, "const ")

		// Extract property type if present
		propType := ""
		if strings.Contains(propLine, ":") {
			parts := strings.Split(propLine, ":")
			if len(parts) > 1 {
				typePart := parts[1]

				// Trim until = or { or end of string
				end := len(typePart)
				if idx := strings.Index(typePart, "="); idx != -1 {
					end = idx
				} else if idx := strings.Index(typePart, "{"); idx != -1 {
					end = idx
				}

				propType = strings.TrimSpace(typePart[:end])
			}
		}

		// Extract property value if present
		propValue := ""
		if strings.Contains(propLine, "=") {
			parts := strings.Split(propLine, "=")
			if len(parts) > 1 {
				valuePart := parts[1]

				// Trim until { or end of statement
				end := len(valuePart)
				if idx := strings.Index(valuePart, "{"); idx != -1 {
					end = idx
				} else if idx := strings.Index(valuePart, ";"); idx != -1 {
					end = idx
				} else if idx := strings.Index(valuePart, "\n"); idx != -1 {
					end = idx
				}

				propValue = strings.TrimSpace(valuePart[:end])
			}
		}

		// Check if property has custom accessors
		hasGetter := strings.Contains(propLine, "get()")
		hasSetter := strings.Contains(propLine, "set(")

		// Create property metadata
		propMetadata := map[string]interface{}{
			"type":        "property",
			"visibility":  visibility,
			"is_val":      isVal,
			"is_var":      isVar,
			"is_lateinit": isLateinit,
			"is_const":    isConst,
			"has_getter":  hasGetter,
			"has_setter":  hasSetter,
		}

		if propType != "" {
			propMetadata["property_type"] = propType
		}

		if propValue != "" {
			propMetadata["initial_value"] = propValue
		}

		// Create property chunk
		propertyChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block type since there's no specific Property type
			Name:      propName,
			Path:      fmt.Sprintf("property:%s", propName),
			Content:   propLine,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  propMetadata,
		}
		propertyChunk.ID = generateKotlinChunkID(propertyChunk)
		chunks = append(chunks, propertyChunk)
	}

	return chunks
}

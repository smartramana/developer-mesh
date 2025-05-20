package parsers

import (
	"fmt"
	"strings"
	"regexp"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractClasses extracts class declarations from Kotlin code
func (p *KotlinParser) extractClasses(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all class declarations
	classMatches := kotlinClassRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range classMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the class name
		className := code[match[2]:match[3]]
		
		// Find the class content
		startPos := match[0]
		classContent := code[startPos:match[1]]
		endPos := match[1]
		
		// Check for the opening brace and get full class body if found
		if strings.Contains(classContent, "{") {
			classContent, endPos = p.findBlockContent(code, startPos)
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Extract modifiers
		classDeclLine := classContent
		if idx := strings.Index(classContent, "{"); idx != -1 {
			classDeclLine = classContent[:idx]
		}
		
		// Check for specific class types and modifiers
		isDataClass := strings.Contains(classDeclLine, "data class")
		isSealedClass := strings.Contains(classDeclLine, "sealed class")
		isEnumClass := strings.Contains(classDeclLine, "enum class")
		isAbstractClass := strings.Contains(classDeclLine, "abstract class")
		isOpenClass := strings.Contains(classDeclLine, "open class")
		
		// Determine visibility
		visibility := "public" // default in Kotlin
		if strings.Contains(classDeclLine, "private class") {
			visibility = "private"
		} else if strings.Contains(classDeclLine, "protected class") {
			visibility = "protected"
		} else if strings.Contains(classDeclLine, "internal class") {
			visibility = "internal"
		}
		
		// Extract generics if present
		generics := ""
		if strings.Contains(className, "<") {
			genericStart := strings.Index(className, "<")
			if genericStart != -1 {
				generics = className[genericStart:]
				className = className[:genericStart]
			}
		}
		
		// Extract superclass/interfaces if present
		// The pattern is: class Name : SuperClass(), Interface1, Interface2
		superTypes := []string{}
		if strings.Contains(classDeclLine, ":") {
			parts := strings.Split(classDeclLine, ":")
			if len(parts) > 1 {
				// Process part after colon to extract superclasses and interfaces
				superTypesPart := parts[1]
				
				// Find position of the opening brace or end of line
				end := len(superTypesPart)
				if bracePos := strings.Index(superTypesPart, "{"); bracePos != -1 {
					end = bracePos
				}
				superTypesPart = superTypesPart[:end]
				
				// Split by commas to get individual supertypes
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
		
		// Create class metadata
		classMetadata := map[string]interface{}{
			"type":          "class",
			"visibility":    visibility,
			"is_data":       isDataClass,
			"is_sealed":     isSealedClass,
			"is_enum":       isEnumClass,
			"is_abstract":   isAbstractClass,
			"is_open":       isOpenClass,
		}
		
		if generics != "" {
			classMetadata["generics"] = generics
		}
		
		if len(superTypes) > 0 {
			classMetadata["super_types"] = superTypes
		}
		
		// Determine chunk type based on class type
		chunkType := chunking.ChunkTypeClass
		if isEnumClass {
			// For enum classes, use ChunkTypeBlock as there's no specific enum type
			chunkType = chunking.ChunkTypeBlock 
		}
		
		// Create class chunk
		classChunk := &chunking.CodeChunk{
			Type:      chunkType,
			Name:      className,
			Path:      fmt.Sprintf("class:%s", className),
			Content:   classContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  classMetadata,
		}
		classChunk.ID = generateKotlinChunkID(classChunk)
		chunks = append(chunks, classChunk)
		
		// If the class has a body, extract nested elements
		if strings.Contains(classContent, "{") {
			// Extract the class body (content between the braces)
			// This is where we would recursively extract class members
			openBracePos := strings.Index(classContent, "{")
			if openBracePos != -1 && len(classContent) > openBracePos+1 {
				// Extract nested constructors, methods, properties, etc.
				// In a full implementation, we would call method extraction, property extraction, etc.
				// with parent ID set to classChunk.ID
				
				// For example:
				// nestedFunctions := p.extractFunctions(classContent[openBracePos+1:len(classContent)-1], nil, classChunk.ID)
				// chunks = append(chunks, nestedFunctions...)
			}
		}
	}
	
	return chunks
}

// extractInterfaces extracts interface declarations from Kotlin code
func (p *KotlinParser) extractInterfaces(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all interface declarations
	interfaceMatches := kotlinInterfaceRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range interfaceMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the interface name
		interfaceName := code[match[2]:match[3]]
		
		// Find the interface content
		startPos := match[0]
		interfaceContent := code[startPos:match[1]]
		endPos := match[1]
		
		// Check for the opening brace and get full interface body if found
		if strings.Contains(interfaceContent, "{") {
			interfaceContent, endPos = p.findBlockContent(code, startPos)
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Extract modifiers
		interfaceDeclLine := interfaceContent
		if idx := strings.Index(interfaceContent, "{"); idx != -1 {
			interfaceDeclLine = interfaceContent[:idx]
		}
		
		// Check for modifiers
		isSealedInterface := strings.Contains(interfaceDeclLine, "sealed interface")
		
		// Determine visibility
		visibility := "public" // default in Kotlin
		if strings.Contains(interfaceDeclLine, "private interface") {
			visibility = "private"
		} else if strings.Contains(interfaceDeclLine, "protected interface") {
			visibility = "protected"
		} else if strings.Contains(interfaceDeclLine, "internal interface") {
			visibility = "internal"
		}
		
		// Extract generics if present
		generics := ""
		if strings.Contains(interfaceName, "<") {
			genericStart := strings.Index(interfaceName, "<")
			if genericStart != -1 {
				generics = interfaceName[genericStart:]
				interfaceName = interfaceName[:genericStart]
			}
		}
		
		// Extract super-interfaces if present
		superInterfaces := []string{}
		if strings.Contains(interfaceDeclLine, ":") {
			parts := strings.Split(interfaceDeclLine, ":")
			if len(parts) > 1 {
				// Process part after colon to extract super-interfaces
				superInterfacesPart := parts[1]
				
				// Find position of the opening brace or end of line
				end := len(superInterfacesPart)
				if bracePos := strings.Index(superInterfacesPart, "{"); bracePos != -1 {
					end = bracePos
				}
				superInterfacesPart = superInterfacesPart[:end]
				
				// Split by commas to get individual super-interfaces
				for _, superInterface := range strings.Split(superInterfacesPart, ",") {
					superInterface = strings.TrimSpace(superInterface)
					
					// Remove generic part if present
					if anglePos := strings.Index(superInterface, "<"); anglePos != -1 {
						superInterface = superInterface[:anglePos]
					}
					
					if superInterface != "" {
						superInterfaces = append(superInterfaces, superInterface)
					}
				}
			}
		}
		
		// Find method signatures in the interface
		methodSignatures := []string{}
		if strings.Contains(interfaceContent, "fun ") {
			// Extract method names from interface body
			methodRegex := regexp.MustCompile(`(?m)fun\s+(\w+)\s*\([^)]*\)(?:\s*:\s*[^{=]+)?`)
			methodMatches := methodRegex.FindAllStringSubmatch(interfaceContent, -1)
			
			for _, mMatch := range methodMatches {
				if len(mMatch) > 1 {
					methodSignatures = append(methodSignatures, mMatch[1])
				}
			}
		}
		
		// Create interface metadata
		interfaceMetadata := map[string]interface{}{
			"type":          "interface",
			"visibility":    visibility,
			"is_sealed":     isSealedInterface,
		}
		
		if generics != "" {
			interfaceMetadata["generics"] = generics
		}
		
		if len(superInterfaces) > 0 {
			interfaceMetadata["super_interfaces"] = superInterfaces
		}
		
		if len(methodSignatures) > 0 {
			interfaceMetadata["method_signatures"] = methodSignatures
		}
		
		// Create interface chunk
		interfaceChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeInterface,
			Name:      interfaceName,
			Path:      fmt.Sprintf("interface:%s", interfaceName),
			Content:   interfaceContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  interfaceMetadata,
		}
		interfaceChunk.ID = generateKotlinChunkID(interfaceChunk)
		chunks = append(chunks, interfaceChunk)
	}
	
	return chunks
}

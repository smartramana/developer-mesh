package parsers

import (
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/internal/chunking"
)

// extractFunctions extracts function declarations from Rust code
func (p *RustParser) extractFunctions(code string, lines []string, parentID string) []*chunking.CodeChunk {
	return p.extractFunctionsWithOffset(code, lines, parentID, false)
}

// extractFunctionsWithOffset extracts function declarations with optional offset handling
func (p *RustParser) extractFunctionsWithOffset(code string, lines []string, parentID string, isSubmodule bool) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all function declarations
	functionMatches := rustFunctionRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range functionMatches {
		if len(match) < 6 {
			continue
		}
		
		// Get the function name and parameters
		funcName := code[match[2]:match[3]]
		params := code[match[4]:match[5]]
		
		// Find the function content (including body)
		startPos := match[0]
		funcContent, endPos := p.findBlockContent(code, startPos)
		
		// Calculate line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Check if it's an async function
		isAsync := strings.Contains(code[match[0]:match[2]], "async")
		
		// Check if the function is public
		isPublic := false
		if strings.HasPrefix(strings.TrimSpace(code[match[0]:]), "pub") {
			isPublic = true
		}
		
		// Extract return type
		returnType := ""
		functionDecl := code[match[0]:endPos]
		returnStart := strings.Index(functionDecl, ") -> ")
		if returnStart != -1 {
			// Find the end of return type (either where '{' or 'where' starts)
			whereOrBrace := strings.IndexAny(functionDecl[returnStart+5:], "{w")
			if whereOrBrace != -1 {
				returnType = strings.TrimSpace(functionDecl[returnStart+5 : returnStart+5+whereOrBrace])
			}
		}
		
		// Parse parameters
		parsedParams := parseRustFunctionParams(params)
		
		// Create function metadata
		funcMetadata := map[string]interface{}{
			"type":       "function",
			"public":     isPublic,
			"async":      isAsync,
			"parameters": parsedParams,
		}
		
		if returnType != "" {
			funcMetadata["return_type"] = returnType
		}
		
		// Create function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      funcName,
			Path:      fmt.Sprintf("fn:%s", funcName),
			Content:   funcContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  funcMetadata,
		}
		functionChunk.ID = generateRustChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}
	
	return chunks
}

// parseRustFunctionParams parses Rust function parameters
func parseRustFunctionParams(paramsStr string) []map[string]string {
	params := []map[string]string{}
	
	// Handle empty params
	paramsStr = strings.TrimSpace(paramsStr)
	if paramsStr == "" {
		return params
	}
	
	// Split parameters (handling complex nested types properly)
	depth := 0
	startPos := 0
	inStr := false
	
	for i, char := range paramsStr {
		switch char {
		case '<', '(', '[', '{':
			if !inStr {
				depth++
			}
		case '>', ')', ']', '}':
			if !inStr {
				depth--
			}
		case '"', '\'':
			// Flip whether we're in a string or not 
			// (simplistic approach, doesn't handle escape sequences)
			inStr = !inStr
		case ',':
			if !inStr && depth == 0 {
				// We found a parameter separator
				paramPart := strings.TrimSpace(paramsStr[startPos:i])
				if paramPart != "" {
					params = append(params, parseRustParam(paramPart))
				}
				startPos = i + 1
			}
		}
	}
	
	// Don't forget the last parameter
	if startPos < len(paramsStr) {
		paramPart := strings.TrimSpace(paramsStr[startPos:])
		if paramPart != "" {
			params = append(params, parseRustParam(paramPart))
		}
	}
	
	return params
}

// parseRustParam parses a single Rust function parameter
func parseRustParam(param string) map[string]string {
	result := map[string]string{}
	
	// Handle self parameter variants
	switch param {
	case "self", "&self", "&mut self", "mut self":
		result["name"] = "self"
		result["type"] = param
		return result
	}
	
	// Try to separate name and type
	parts := strings.SplitN(param, ":", 2)
	if len(parts) == 2 {
		result["name"] = strings.TrimSpace(parts[0])
		result["type"] = strings.TrimSpace(parts[1])
	} else {
		// If no type specified, just store the parameter as is
		result["name"] = param
	}
	
	return result
}

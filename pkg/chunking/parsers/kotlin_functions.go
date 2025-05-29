package parsers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractFunctions extracts function declarations from Kotlin code
func (p *KotlinParser) extractFunctions(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all function declarations
	functionMatches := kotlinFunctionRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range functionMatches {
		if len(match) < 6 {
			continue
		}

		// Get the function name and parameters
		funcName := code[match[2]:match[3]]
		params := code[match[4]:match[5]]

		// Find the function content
		startPos := match[0]
		funcContent := code[startPos:match[1]]
		endPos := match[1]

		// Check for the opening brace and get full function body if found
		if strings.Contains(funcContent, "{") {
			funcContent, endPos = p.findBlockContent(code, startPos)
		}

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Extract modifiers from function declaration
		funcDeclLine := funcContent
		if idx := strings.Index(funcContent, "{"); idx != -1 {
			funcDeclLine = funcContent[:idx]
		} else if idx := strings.Index(funcContent, "="); idx != -1 {
			funcDeclLine = funcContent[:idx]
		}

		// Check for specific function modifiers
		isInline := strings.Contains(funcDeclLine, "inline fun")
		isSuspend := strings.Contains(funcDeclLine, "suspend fun")
		isTailrec := strings.Contains(funcDeclLine, "tailrec fun")
		isExternal := strings.Contains(funcDeclLine, "external fun")
		isOperator := strings.Contains(funcDeclLine, "operator fun")
		isInfix := strings.Contains(funcDeclLine, "infix fun")

		// Determine visibility
		visibility := "public" // default in Kotlin
		if strings.Contains(funcDeclLine, "private fun") {
			visibility = "private"
		} else if strings.Contains(funcDeclLine, "protected fun") {
			visibility = "protected"
		} else if strings.Contains(funcDeclLine, "internal fun") {
			visibility = "internal"
		}

		// Extract return type if present
		returnType := ""
		if strings.Contains(funcDeclLine, ":") {
			// Parse everything after the parameter list
			afterParams := funcDeclLine[strings.Index(funcDeclLine, ")")+1:]

			// Look for the return type declaration after :
			if strings.Contains(afterParams, ":") {
				returnTypePart := afterParams[strings.Index(afterParams, ":")+1:]

				// Trim until { or = or end
				end := len(returnTypePart)
				if idx := strings.Index(returnTypePart, "{"); idx != -1 {
					end = idx
				} else if idx := strings.Index(returnTypePart, "="); idx != -1 {
					end = idx
				}

				returnType = strings.TrimSpace(returnTypePart[:end])
			}
		}

		// Extract generic type parameters if present
		generics := ""
		genericRegex := regexp.MustCompile(`fun\s+<([^>]+)>`)
		genericMatch := genericRegex.FindStringSubmatch(funcDeclLine)
		if len(genericMatch) > 1 {
			generics = genericMatch[1]
		}

		// Parse parameters
		parsedParams := parseKotlinFunctionParams(params)

		// Check if this is an expression body function (using = instead of {})
		isExpressionBody := strings.Contains(funcDeclLine, "=") && !strings.Contains(funcDeclLine, "{")

		// Create function metadata
		funcMetadata := map[string]interface{}{
			"type":               "function",
			"visibility":         visibility,
			"is_inline":          isInline,
			"is_suspend":         isSuspend,
			"is_tailrec":         isTailrec,
			"is_external":        isExternal,
			"is_operator":        isOperator,
			"is_infix":           isInfix,
			"is_expression_body": isExpressionBody,
			"parameters":         parsedParams,
		}

		if returnType != "" {
			funcMetadata["return_type"] = returnType
		}

		if generics != "" {
			funcMetadata["generics"] = generics
		}

		// Create function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      funcName,
			Path:      fmt.Sprintf("fun:%s", funcName),
			Content:   funcContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  funcMetadata,
		}
		functionChunk.ID = generateKotlinChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}

	return chunks
}

// extractExtensions extracts extension function declarations from Kotlin code
func (p *KotlinParser) extractExtensions(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all extension function declarations
	extensionMatches := kotlinExtensionRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range extensionMatches {
		if len(match) < 8 {
			continue
		}

		// Get the receiver type, function name, and parameters
		receiverType := code[match[2]:match[3]]
		funcName := code[match[4]:match[5]]
		params := code[match[6]:match[7]]

		// Find the function content
		startPos := match[0]
		funcContent := code[startPos:match[1]]
		endPos := match[1]

		// Check for the opening brace and get full function body if found
		if strings.Contains(funcContent, "{") {
			funcContent, endPos = p.findBlockContent(code, startPos)
		}

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Extract modifiers from function declaration
		funcDeclLine := funcContent
		if idx := strings.Index(funcContent, "{"); idx != -1 {
			funcDeclLine = funcContent[:idx]
		} else if idx := strings.Index(funcContent, "="); idx != -1 {
			funcDeclLine = funcContent[:idx]
		}

		// Check for specific function modifiers
		isInline := strings.Contains(funcDeclLine, "inline fun")
		isSuspend := strings.Contains(funcDeclLine, "suspend fun")
		isTailrec := strings.Contains(funcDeclLine, "tailrec fun")

		// Determine visibility
		visibility := "public" // default in Kotlin
		if strings.Contains(funcDeclLine, "private fun") {
			visibility = "private"
		} else if strings.Contains(funcDeclLine, "protected fun") {
			visibility = "protected"
		} else if strings.Contains(funcDeclLine, "internal fun") {
			visibility = "internal"
		}

		// Extract return type if present
		returnType := ""
		if strings.Contains(funcDeclLine, ":") {
			// Parse everything after the parameter list
			afterParams := funcDeclLine[strings.Index(funcDeclLine, ")")+1:]

			// Look for the return type declaration after :
			if strings.Contains(afterParams, ":") {
				returnTypePart := afterParams[strings.Index(afterParams, ":")+1:]

				// Trim until { or = or end
				end := len(returnTypePart)
				if idx := strings.Index(returnTypePart, "{"); idx != -1 {
					end = idx
				} else if idx := strings.Index(returnTypePart, "="); idx != -1 {
					end = idx
				}

				returnType = strings.TrimSpace(returnTypePart[:end])
			}
		}

		// Clean up the receiver type (remove generics if present)
		if idx := strings.Index(receiverType, "<"); idx != -1 {
			receiverType = receiverType[:idx]
		}
		receiverType = strings.TrimSpace(receiverType)

		// Parse parameters
		parsedParams := parseKotlinFunctionParams(params)

		// Check if this is an expression body function (using = instead of {})
		isExpressionBody := strings.Contains(funcDeclLine, "=") && !strings.Contains(funcDeclLine, "{")

		// Create extension function metadata
		funcMetadata := map[string]interface{}{
			"type":               "extension_function",
			"visibility":         visibility,
			"is_inline":          isInline,
			"is_suspend":         isSuspend,
			"is_tailrec":         isTailrec,
			"is_expression_body": isExpressionBody,
			"receiver_type":      receiverType,
			"parameters":         parsedParams,
		}

		if returnType != "" {
			funcMetadata["return_type"] = returnType
		}

		// Create extension function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      funcName,
			Path:      fmt.Sprintf("extension:%s.%s", receiverType, funcName),
			Content:   funcContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  funcMetadata,
		}
		functionChunk.ID = generateKotlinChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}

	return chunks
}

// Helper function to parse Kotlin function parameters
func parseKotlinFunctionParams(paramsStr string) []map[string]string {
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
			// Toggle string state
			inStr = !inStr
		case ',':
			if !inStr && depth == 0 {
				// We found a parameter separator
				paramPart := strings.TrimSpace(paramsStr[startPos:i])
				if paramPart != "" {
					params = append(params, parseKotlinParam(paramPart))
				}
				startPos = i + 1
			}
		}
	}

	// Don't forget the last parameter
	if startPos < len(paramsStr) {
		paramPart := strings.TrimSpace(paramsStr[startPos:])
		if paramPart != "" {
			params = append(params, parseKotlinParam(paramPart))
		}
	}

	return params
}

// parseKotlinParam parses a single Kotlin function parameter
func parseKotlinParam(param string) map[string]string {
	result := map[string]string{}

	// Check for default value
	hasDefaultValue := false
	defaultValue := ""
	if strings.Contains(param, "=") {
		parts := strings.SplitN(param, "=", 2)
		param = strings.TrimSpace(parts[0])
		defaultValue = strings.TrimSpace(parts[1])
		hasDefaultValue = true
	}

	// Check for variadic parameter
	isVariadic := strings.Contains(param, "vararg ")
	if isVariadic {
		param = strings.Replace(param, "vararg ", "", 1)
	}

	// Extract name and type
	if strings.Contains(param, ":") {
		parts := strings.SplitN(param, ":", 2)
		name := strings.TrimSpace(parts[0])
		paramType := strings.TrimSpace(parts[1])

		result["name"] = name
		result["type"] = paramType
	} else {
		// If no type specified, just store the parameter as name
		result["name"] = param
	}

	// Add additional metadata
	if isVariadic {
		result["is_vararg"] = "true"
	}

	if hasDefaultValue {
		result["has_default"] = "true"
		result["default_value"] = defaultValue
	}

	return result
}

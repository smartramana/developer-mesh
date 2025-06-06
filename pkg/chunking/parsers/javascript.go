package parsers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// JavaScript regular expressions for extracting code structure
var (
	// Match import statements
	jsImportRegex = regexp.MustCompile(`(?m)^(?:import\s+(?:\{[^}]*\}|\*\s+as\s+\w+|\w+)\s+from\s+['"]([^'"]+)['"]|const\s+(?:\{[^}]*\}|\w+)\s*=\s*require\(['"]([^'"]+)['"]\))`)

	// Match class declarations
	jsClassRegex = regexp.MustCompile(`(?m)^(?:export\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?\s*\{`)

	// Match function declarations
	jsFunctionRegex = regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(([^)]*)\)`)

	// Match arrow functions assigned to variables
	jsArrowFunctionRegex = regexp.MustCompile(`(?m)^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(([^)]*)\)\s*=>`)

	// Match method declarations inside classes
	jsMethodRegex = regexp.MustCompile(`(?m)^\s+(?:async\s+)?(\w+)\s*\(([^)]*)\)`)

	// Match object literal methods
	// TODO: Implement object literal method extraction for better object analysis
	// jsObjectMethodRegex = regexp.MustCompile(`(?m)^\s+(\w+):\s*(?:async\s*)?\(([^)]*)\)`)

	// Match constructor
	jsConstructorRegex = regexp.MustCompile(`(?m)^\s+constructor\s*\(([^)]*)\)`)

	// Match JSDoc comments
	jsDocRegex = regexp.MustCompile(`(?ms)/\*\*.*?\*/`)
)

// JavaScriptParser is a parser for JavaScript code
type JavaScriptParser struct{}

// NewJavaScriptParser creates a new JavaScriptParser
func NewJavaScriptParser() *JavaScriptParser {
	return &JavaScriptParser{}
}

// GetLanguage returns the language this parser handles
func (p *JavaScriptParser) GetLanguage() chunking.Language {
	return chunking.LanguageJavaScript
}

// Parse parses JavaScript code and returns chunks
func (p *JavaScriptParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	chunks := []*chunking.CodeChunk{}

	// Create a chunk for the entire file
	fileChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageJavaScript,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata:  map[string]interface{}{},
	}
	fileChunk.ID = generateJSChunkID(fileChunk)
	chunks = append(chunks, fileChunk)

	// Split code by lines
	lines := strings.Split(code, "\n")

	// Extract imports
	importChunks := p.extractImports(code, lines, fileChunk.ID)
	chunks = append(chunks, importChunks...)

	// Extract JSDoc comments
	commentChunks := p.extractJSDocComments(code, lines, fileChunk.ID)
	chunks = append(chunks, commentChunks...)

	// Extract classes with their methods
	classChunks, methodChunks := p.extractClasses(code, lines, fileChunk.ID)
	chunks = append(chunks, classChunks...)
	chunks = append(chunks, methodChunks...)

	// Extract standalone functions
	functionChunks := p.extractFunctions(code, lines, fileChunk.ID)
	chunks = append(chunks, functionChunks...)

	// Process dependencies
	p.processDependencies(chunks)

	return chunks, nil
}

// extractImports extracts import statements from JavaScript code
func (p *JavaScriptParser) extractImports(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all import statements
	importMatches := jsImportRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range importMatches {
		if len(match) < 4 {
			continue
		}

		// Get the import statement
		importStatement := code[match[0]:match[1]]

		// Get the import path
		var importPath string
		if match[2] != -1 && match[3] != -1 {
			importPath = code[match[2]:match[3]]
		} else if match[4] != -1 && match[5] != -1 {
			importPath = code[match[4]:match[5]]
		} else {
			continue
		}

		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := getLineNumberFromPos(code, match[1]) + 1

		// Create import chunk
		importChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeImport,
			Name:      filepath.Base(importPath),
			Path:      importPath,
			Content:   importStatement,
			Language:  chunking.LanguageJavaScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata: map[string]interface{}{
				"import_path": importPath,
			},
		}
		importChunk.ID = generateJSChunkID(importChunk)
		chunks = append(chunks, importChunk)
	}

	return chunks
}

// extractJSDocComments extracts JSDoc comments from JavaScript code
func (p *JavaScriptParser) extractJSDocComments(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all JSDoc comments
	commentMatches := jsDocRegex.FindAllStringIndex(code, -1)

	for i, match := range commentMatches {
		if len(match) < 2 {
			continue
		}

		// Get the comment
		commentText := code[match[0]:match[1]]

		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := getLineNumberFromPos(code, match[1]) + 1

		// Create comment chunk
		commentChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("JSDoc_%d", i+1),
			Path:      fmt.Sprintf("comment:%d", startLine),
			Content:   commentText,
			Language:  chunking.LanguageJavaScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
		}
		commentChunk.ID = generateJSChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}

	return chunks
}

// extractClasses extracts classes and their methods from JavaScript code
func (p *JavaScriptParser) extractClasses(code string, lines []string, parentID string) ([]*chunking.CodeChunk, []*chunking.CodeChunk) {
	classChunks := []*chunking.CodeChunk{}
	methodChunks := []*chunking.CodeChunk{}

	// Find all class declarations
	classMatches := jsClassRegex.FindAllStringSubmatchIndex(code, -1)

	for _, classMatch := range classMatches {
		if len(classMatch) < 4 {
			continue
		}

		// Get the class name
		className := code[classMatch[2]:classMatch[3]]

		// Get parent class if exists
		var parentClass string
		if classMatch[4] != -1 && classMatch[5] != -1 {
			parentClass = code[classMatch[4]:classMatch[5]]
		}

		// Find the class body by counting braces
		braceCount := 1
		classEndPos := classMatch[1]

		// Skip to the opening brace
		for i := classMatch[1]; i < len(code); i++ {
			if code[i] == '{' {
				classEndPos = i + 1
				break
			}
		}

		// Find the closing brace by counting opening and closing braces
		for i := classEndPos; i < len(code); i++ {
			if code[i] == '{' {
				braceCount++
			} else if code[i] == '}' {
				braceCount--
				if braceCount == 0 {
					classEndPos = i + 1
					break
				}
			}
		}

		// Get the class content
		classContent := code[classMatch[0]:classEndPos]

		// Find the line numbers
		startLine := getLineNumberFromPos(code, classMatch[0]) + 1
		endLine := getLineNumberFromPos(code, classEndPos) + 1

		// Create class metadata
		classMetadata := map[string]interface{}{
			"type": "class",
		}

		if parentClass != "" {
			classMetadata["extends"] = parentClass
		}

		// Create class chunk
		classChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeClass,
			Name:      className,
			Path:      className,
			Content:   classContent,
			Language:  chunking.LanguageJavaScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  classMetadata,
		}
		classChunk.ID = generateJSChunkID(classChunk)
		classChunks = append(classChunks, classChunk)

		// Extract methods from class content
		classMethods := p.extractMethods(classContent, classChunk.ID, className, startLine)
		methodChunks = append(methodChunks, classMethods...)
	}

	return classChunks, methodChunks
}

// extractMethods extracts methods from a class
func (p *JavaScriptParser) extractMethods(classContent string, parentID, className string, classStartLine int) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all method declarations
	methodMatches := jsMethodRegex.FindAllStringSubmatchIndex(classContent, -1)

	for _, methodMatch := range methodMatches {
		if len(methodMatch) < 6 {
			continue
		}

		// Get the method name
		methodName := classContent[methodMatch[2]:methodMatch[3]]

		// Get parameters
		params := classContent[methodMatch[4]:methodMatch[5]]

		// Find the method body by counting braces
		braceCount := 1
		methodEndPos := methodMatch[1]

		// Skip to the opening brace
		for i := methodMatch[1]; i < len(classContent); i++ {
			if classContent[i] == '{' {
				methodEndPos = i + 1
				break
			}
		}

		// Find the closing brace by counting opening and closing braces
		for i := methodEndPos; i < len(classContent); i++ {
			if classContent[i] == '{' {
				braceCount++
			} else if classContent[i] == '}' {
				braceCount--
				if braceCount == 0 {
					methodEndPos = i + 1
					break
				}
			}
		}

		// Get the method content
		methodContent := classContent[methodMatch[0]:methodEndPos]

		// Find the line numbers (relative to the class)
		methodStartLine := getLineNumberFromPos(classContent, methodMatch[0]) + 1
		methodEndLine := getLineNumberFromPos(classContent, methodEndPos) + 1

		// Adjust line numbers to be relative to the file
		absoluteStartLine := classStartLine + methodStartLine - 1
		absoluteEndLine := classStartLine + methodEndLine - 1

		// Create method metadata
		methodMetadata := map[string]interface{}{
			"params": params,
			"class":  className,
		}

		// Create method chunk
		methodChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeMethod,
			Name:      methodName,
			Path:      fmt.Sprintf("%s.%s", className, methodName),
			Content:   methodContent,
			Language:  chunking.LanguageJavaScript,
			StartLine: absoluteStartLine,
			EndLine:   absoluteEndLine,
			ParentID:  parentID,
			Metadata:  methodMetadata,
		}
		methodChunk.ID = generateJSChunkID(methodChunk)
		chunks = append(chunks, methodChunk)
	}

	// Find constructor
	constructorMatches := jsConstructorRegex.FindAllStringSubmatchIndex(classContent, -1)

	for _, constructorMatch := range constructorMatches {
		if len(constructorMatch) < 3 {
			continue
		}

		// Get parameters
		params := classContent[constructorMatch[2]:constructorMatch[3]]

		// Find the constructor body by counting braces
		braceCount := 1
		constructorEndPos := constructorMatch[1]

		// Skip to the opening brace
		for i := constructorMatch[1]; i < len(classContent); i++ {
			if classContent[i] == '{' {
				constructorEndPos = i + 1
				break
			}
		}

		// Find the closing brace by counting opening and closing braces
		for i := constructorEndPos; i < len(classContent); i++ {
			if classContent[i] == '{' {
				braceCount++
			} else if classContent[i] == '}' {
				braceCount--
				if braceCount == 0 {
					constructorEndPos = i + 1
					break
				}
			}
		}

		// Get the constructor content
		constructorContent := classContent[constructorMatch[0]:constructorEndPos]

		// Find the line numbers (relative to the class)
		constructorStartLine := getLineNumberFromPos(classContent, constructorMatch[0]) + 1
		constructorEndLine := getLineNumberFromPos(classContent, constructorEndPos) + 1

		// Adjust line numbers to be relative to the file
		absoluteStartLine := classStartLine + constructorStartLine - 1
		absoluteEndLine := classStartLine + constructorEndLine - 1

		// Create constructor metadata
		constructorMetadata := map[string]interface{}{
			"params": params,
			"class":  className,
		}

		// Create constructor chunk
		constructorChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeMethod,
			Name:      "constructor",
			Path:      fmt.Sprintf("%s.constructor", className),
			Content:   constructorContent,
			Language:  chunking.LanguageJavaScript,
			StartLine: absoluteStartLine,
			EndLine:   absoluteEndLine,
			ParentID:  parentID,
			Metadata:  constructorMetadata,
		}
		constructorChunk.ID = generateJSChunkID(constructorChunk)
		chunks = append(chunks, constructorChunk)
	}

	return chunks
}

// extractFunctions extracts standalone functions from JavaScript code
func (p *JavaScriptParser) extractFunctions(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all function declarations
	functionMatches := jsFunctionRegex.FindAllStringSubmatchIndex(code, -1)

	for _, functionMatch := range functionMatches {
		if len(functionMatch) < 6 {
			continue
		}

		// Get the function name
		functionName := code[functionMatch[2]:functionMatch[3]]

		// Get parameters
		params := code[functionMatch[4]:functionMatch[5]]

		// Find the function body by counting braces
		braceCount := 1
		functionEndPos := functionMatch[1]

		// Skip to the opening brace
		for i := functionMatch[1]; i < len(code); i++ {
			if code[i] == '{' {
				functionEndPos = i + 1
				break
			}
		}

		// Find the closing brace by counting opening and closing braces
		for i := functionEndPos; i < len(code); i++ {
			if code[i] == '{' {
				braceCount++
			} else if code[i] == '}' {
				braceCount--
				if braceCount == 0 {
					functionEndPos = i + 1
					break
				}
			}
		}

		// Get the function content
		functionContent := code[functionMatch[0]:functionEndPos]

		// Find the line numbers
		startLine := getLineNumberFromPos(code, functionMatch[0]) + 1
		endLine := getLineNumberFromPos(code, functionEndPos) + 1

		// Create function metadata
		functionMetadata := map[string]interface{}{
			"params": params,
		}

		// Create function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      functionName,
			Path:      functionName,
			Content:   functionContent,
			Language:  chunking.LanguageJavaScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  functionMetadata,
		}
		functionChunk.ID = generateJSChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}

	// Find all arrow functions
	arrowFunctionMatches := jsArrowFunctionRegex.FindAllStringSubmatchIndex(code, -1)

	for _, arrowMatch := range arrowFunctionMatches {
		if len(arrowMatch) < 6 {
			continue
		}

		// Get the function name (variable name)
		functionName := code[arrowMatch[2]:arrowMatch[3]]

		// Get parameters
		params := code[arrowMatch[4]:arrowMatch[5]]

		// Find the function body by counting braces
		braceCount := 1
		functionEndPos := arrowMatch[1]

		// Skip to the => and then to the opening brace or expression
		foundArrow := false
		for i := arrowMatch[1]; i < len(code); i++ {
			if !foundArrow && code[i] == '=' && i+1 < len(code) && code[i+1] == '>' {
				foundArrow = true
				i++
				continue
			}

			if foundArrow {
				// Check if it's a block body or expression body
				if code[i] == '{' {
					functionEndPos = i + 1

					// Find the closing brace
					for j := functionEndPos; j < len(code); j++ {
						if code[j] == '{' {
							braceCount++
						} else if code[j] == '}' {
							braceCount--
							if braceCount == 0 {
								functionEndPos = j + 1
								break
							}
						}
					}
					break
				} else if code[i] != ' ' && code[i] != '\t' && code[i] != '\n' && code[i] != '\r' {
					// Expression body - find the end of the line or semicolon
					functionEndPos = i
					for j := i; j < len(code); j++ {
						if code[j] == ';' || code[j] == '\n' {
							functionEndPos = j + 1
							break
						}
					}
					break
				}
			}
		}

		// Get the function content
		functionContent := code[arrowMatch[0]:functionEndPos]

		// Find the line numbers
		startLine := getLineNumberFromPos(code, arrowMatch[0]) + 1
		endLine := getLineNumberFromPos(code, functionEndPos) + 1

		// Create function metadata
		functionMetadata := map[string]interface{}{
			"params":         params,
			"arrow_function": true,
		}

		// Create function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      functionName,
			Path:      functionName,
			Content:   functionContent,
			Language:  chunking.LanguageJavaScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  functionMetadata,
		}
		functionChunk.ID = generateJSChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}

	return chunks
}

// processDependencies analyzes chunks to identify dependencies between them
func (p *JavaScriptParser) processDependencies(chunks []*chunking.CodeChunk) {
	// Map chunks by ID for quick lookup
	chunkMap := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}

	// Map chunks by name for dependency tracking
	chunksByName := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeFunction ||
			chunk.Type == chunking.ChunkTypeMethod ||
			chunk.Type == chunking.ChunkTypeClass {
			chunksByName[chunk.Name] = chunk
		}
	}

	// Analyze dependencies
	for _, chunk := range chunks {
		// Skip non-code chunks
		if chunk.Type != chunking.ChunkTypeFunction &&
			chunk.Type != chunking.ChunkTypeMethod &&
			chunk.Type != chunking.ChunkTypeClass {
			continue
		}

		// Analyze content for references to other chunks
		for name, dependentChunk := range chunksByName {
			// Skip self-reference
			if name == chunk.Name {
				continue
			}

			// Check if the chunk content contains references to other chunks
			// Use word boundary regex to avoid partial matches
			regex := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
			if regex.MatchString(chunk.Content) {
				// Add the dependency
				if chunk.Dependencies == nil {
					chunk.Dependencies = []string{}
				}
				chunk.Dependencies = append(chunk.Dependencies, dependentChunk.ID)
			}
		}

		// Add parent dependency if it exists
		if chunk.ParentID != "" && chunk.ParentID != chunk.ID {
			chunk.Dependencies = append(chunk.Dependencies, chunk.ParentID)
		}
	}
}

// generateJSChunkID generates a unique ID for a JavaScript chunk
func generateJSChunkID(chunk *chunking.CodeChunk) string {
	// Combine type, name, path, and line numbers for a unique identifier
	idString := string(chunk.Type) + ":" + chunk.Path + ":" + strconv.Itoa(chunk.StartLine) + "-" + strconv.Itoa(chunk.EndLine)

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(idString))
	return hex.EncodeToString(hash[:])
}

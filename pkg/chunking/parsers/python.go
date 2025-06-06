package parsers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// Python regular expressions for extracting code structure
var (
	// Match import statements - both import x and from x import y
	pythonImportRegex = regexp.MustCompile(`(?m)^(?:from\s+([\w.]+)\s+import\s+(?:[^#\n]+)|import\s+([\w.]+(?:\s*,\s*[\w.]+)*)(?:\s+as\s+\w+)?)`)

	// Match class declarations
	pythonClassRegex = regexp.MustCompile(`(?m)^class\s+(\w+)(?:\(([^)]+)\))?:`)

	// Match function declarations
	pythonFunctionRegex = regexp.MustCompile(`(?m)^def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*[^:]+)?:`)

	// Match method declarations (indented functions)
	pythonMethodRegex = regexp.MustCompile(`(?m)^(\s+)def\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*[^:]+)?:`)

	// Match docstrings (single and multi-line)
	pythonDocstringRegex = regexp.MustCompile(`(?ms)('''.*?'''|""".*?""")`)

	// Match decorators
	// TODO: Implement decorator extraction to enhance Python code analysis
	// pythonDecoratorRegex = regexp.MustCompile(`(?m)^(@\w+(?:\([^)]*\))?)`)
)

// PythonParser is a parser for Python code
type PythonParser struct{}

// NewPythonParser creates a new PythonParser
func NewPythonParser() *PythonParser {
	return &PythonParser{}
}

// GetLanguage returns the language this parser handles
func (p *PythonParser) GetLanguage() chunking.Language {
	return chunking.LanguagePython
}

// Parse parses Python code and returns chunks
func (p *PythonParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	chunks := []*chunking.CodeChunk{}

	// Create a chunk for the entire file
	fileChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguagePython,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata:  map[string]interface{}{},
	}
	fileChunk.ID = generatePythonChunkID(fileChunk)
	chunks = append(chunks, fileChunk)

	// Extract imports
	importChunks := p.extractImports(code, fileChunk.ID)
	chunks = append(chunks, importChunks...)

	// Extract docstrings
	docstringChunks := p.extractDocstrings(code, fileChunk.ID)
	chunks = append(chunks, docstringChunks...)

	// Extract classes with their methods
	classChunks, classMethodChunks := p.extractClasses(code, fileChunk.ID)
	chunks = append(chunks, classChunks...)
	chunks = append(chunks, classMethodChunks...)

	// Extract standalone functions (not methods within classes)
	functionChunks := p.extractFunctions(code, fileChunk.ID)
	chunks = append(chunks, functionChunks...)

	// Process dependencies
	p.processDependencies(chunks)

	return chunks, nil
}

// extractImports extracts import statements from Python code
func (p *PythonParser) extractImports(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all import statements
	importMatches := pythonImportRegex.FindAllStringSubmatchIndex(code, -1)

	for i, match := range importMatches {
		if len(match) < 2 {
			continue
		}

		// Get the import statement
		importStatement := code[match[0]:match[1]]

		// Find the line numbers
		startLine := countLinesUpTo(code, match[0]) + 1
		endLine := countLinesUpTo(code, match[1]) + 1

		// Extract import name/path
		var importPath string
		if match[2] != -1 && match[3] != -1 {
			// from X import Y
			importPath = code[match[2]:match[3]]
		} else if match[4] != -1 && match[5] != -1 {
			// import X
			importPath = code[match[4]:match[5]]
		} else {
			importPath = fmt.Sprintf("import_%d", i+1)
		}

		// Create import chunk
		importChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeImport,
			Name:      fmt.Sprintf("import_%d", i+1),
			Path:      fmt.Sprintf("import:%s", importPath),
			Content:   importStatement,
			Language:  chunking.LanguagePython,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata: map[string]interface{}{
				"import_path": importPath,
			},
		}
		importChunk.ID = generatePythonChunkID(importChunk)
		chunks = append(chunks, importChunk)
	}

	return chunks
}

// extractDocstrings extracts docstring comments from Python code
func (p *PythonParser) extractDocstrings(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all docstrings
	docstringMatches := pythonDocstringRegex.FindAllStringIndex(code, -1)

	for i, match := range docstringMatches {
		if len(match) < 2 {
			continue
		}

		// Get the docstring
		docstringText := code[match[0]:match[1]]

		// Find the line numbers
		startLine := countLinesUpTo(code, match[0]) + 1
		endLine := countLinesUpTo(code, match[1]) + 1

		// Create docstring chunk
		docstringChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("Docstring_%d", i+1),
			Path:      fmt.Sprintf("comment:%d", startLine),
			Content:   docstringText,
			Language:  chunking.LanguagePython,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
		}
		docstringChunk.ID = generatePythonChunkID(docstringChunk)
		chunks = append(chunks, docstringChunk)
	}

	return chunks
}

// getIndentedBlock extracts the indented block following a Python class/function declaration
func (p *PythonParser) getIndentedBlock(code string, startIdx int, baseIndent string) (string, int) {
	// Find the end of the line
	endOfLine := startIdx
	for endOfLine < len(code) && code[endOfLine] != '\n' {
		endOfLine++
	}
	if endOfLine >= len(code) {
		return code[startIdx:], len(code)
	}

	// Skip the newline
	startIdx = endOfLine + 1
	if startIdx >= len(code) {
		return code[startIdx-1:], len(code)
	}

	// Find the indentation of the next line
	nextLineStart := startIdx
	indentEnd := nextLineStart
	for indentEnd < len(code) && (code[indentEnd] == ' ' || code[indentEnd] == '\t') {
		indentEnd++
	}

	indentation := code[nextLineStart:indentEnd]
	if len(indentation) <= len(baseIndent) {
		// Not an indented block or the block is empty
		return code[startIdx-1 : startIdx], startIdx
	}

	// Process the indented block
	endOfBlock := startIdx
	for endOfBlock < len(code) {
		// Find the start of the line
		lineStart := endOfBlock

		// If we're at the end of the file, break
		if lineStart >= len(code) {
			break
		}

		// Skip empty lines
		if lineStart < len(code) && (code[lineStart] == '\n' || code[lineStart] == '\r') {
			endOfBlock = lineStart + 1
			continue
		}

		// Check the indentation of this line
		lineIndentEnd := lineStart
		for lineIndentEnd < len(code) && (code[lineIndentEnd] == ' ' || code[lineIndentEnd] == '\t') {
			lineIndentEnd++
		}

		// If this line is less indented than our block, we've reached the end
		lineIndent := code[lineStart:lineIndentEnd]
		if len(lineIndent) < len(indentation) {
			break
		}

		// Find the end of this line
		endOfLine = lineIndentEnd
		for endOfLine < len(code) && code[endOfLine] != '\n' {
			endOfLine++
		}

		// Move to the next line
		endOfBlock = endOfLine + 1
	}

	return code[startIdx-1 : endOfBlock], endOfBlock
}

// extractClasses extracts classes and their methods from Python code
func (p *PythonParser) extractClasses(code string, parentID string) ([]*chunking.CodeChunk, []*chunking.CodeChunk) {
	classChunks := []*chunking.CodeChunk{}
	methodChunks := []*chunking.CodeChunk{}

	// Find all class declarations
	classMatches := pythonClassRegex.FindAllStringSubmatchIndex(code, -1)

	for _, classMatch := range classMatches {
		if len(classMatch) < 2 {
			continue
		}

		// Get the class name
		var className string
		if classMatch[2] != -1 && classMatch[3] != -1 {
			className = code[classMatch[2]:classMatch[3]]
		} else {
			continue
		}

		// Get parent classes if any
		var parentClasses string
		if classMatch[4] != -1 && classMatch[5] != -1 {
			parentClasses = code[classMatch[4]:classMatch[5]]
		}

		// Get the indentation level before the class
		startPos := classMatch[0]
		baseIndent := ""
		if startPos > 0 {
			indentStart := startPos - 1
			for indentStart >= 0 && code[indentStart] != '\n' {
				indentStart--
			}
			if indentStart < 0 {
				indentStart = 0
			} else {
				indentStart++
			}
			baseIndent = code[indentStart:startPos]
		}

		// Get the class body (indented block)
		classBlock, endPos := p.getIndentedBlock(code, classMatch[1], baseIndent)
		classContent := code[startPos:endPos]

		// Get line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create class metadata
		classMetadata := map[string]interface{}{
			"type": "class",
		}

		if parentClasses != "" {
			classMetadata["parent_classes"] = parentClasses
		}

		// Create class chunk
		classChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeClass,
			Name:      className,
			Path:      className,
			Content:   classContent,
			Language:  chunking.LanguagePython,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  classMetadata,
		}
		classChunk.ID = generatePythonChunkID(classChunk)
		classChunks = append(classChunks, classChunk)

		// Find all methods in this class
		methodsInClass := p.extractMethodsFromClass(classBlock, className, classChunk.ID, startLine)
		methodChunks = append(methodChunks, methodsInClass...)
	}

	return classChunks, methodChunks
}

// extractMethodsFromClass extracts methods from a class
func (p *PythonParser) extractMethodsFromClass(classBlock, className, parentID string, classStartLine int) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all method declarations
	methodMatches := pythonMethodRegex.FindAllStringSubmatchIndex(classBlock, -1)

	for _, methodMatch := range methodMatches {
		if len(methodMatch) < 6 {
			continue
		}

		// Get the method indentation
		indent := classBlock[methodMatch[2]:methodMatch[3]]

		// Get the method name
		methodName := classBlock[methodMatch[4]:methodMatch[5]]

		// Get parameters
		parameters := ""
		if methodMatch[6] != -1 && methodMatch[7] != -1 {
			parameters = classBlock[methodMatch[6]:methodMatch[7]]
		}

		// Get the method body (indented block)
		startPos := methodMatch[0]
		_, endPos := p.getIndentedBlock(classBlock, methodMatch[1], indent)
		methodContent := classBlock[startPos:endPos]

		// Get line numbers relative to the file
		startLine := countLinesUpTo(classBlock, startPos) + 1 + classStartLine - 1
		endLine := countLinesUpTo(classBlock, endPos) + 1 + classStartLine - 1

		// Check for decorators before the method
		decorators := []string{}
		for i := startPos - 1; i >= 0; i-- {
			if classBlock[i] == '\n' {
				// Check if the previous line is a decorator
				decoratorStart := i + 1
				for decoratorStart < len(classBlock) && (classBlock[decoratorStart] == ' ' || classBlock[decoratorStart] == '\t') {
					decoratorStart++
				}

				if decoratorStart < len(classBlock) && classBlock[decoratorStart] == '@' {
					decoratorEnd := decoratorStart
					for decoratorEnd < len(classBlock) && classBlock[decoratorEnd] != '\n' {
						decoratorEnd++
					}

					decorator := classBlock[decoratorStart:decoratorEnd]
					decorators = append(decorators, decorator)
					startLine = countLinesUpTo(classBlock, decoratorStart) + 1 + classStartLine - 1
				} else {
					break
				}
			}
		}

		// Create method metadata
		methodMetadata := map[string]interface{}{
			"type":       "method",
			"parameters": parameters,
			"class":      className,
		}

		if len(decorators) > 0 {
			methodMetadata["decorators"] = decorators
		}

		// Create method chunk
		methodChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeMethod,
			Name:      methodName,
			Path:      fmt.Sprintf("%s.%s", className, methodName),
			Content:   methodContent,
			Language:  chunking.LanguagePython,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  methodMetadata,
		}
		methodChunk.ID = generatePythonChunkID(methodChunk)
		chunks = append(chunks, methodChunk)
	}

	return chunks
}

// extractFunctions extracts standalone functions from Python code
func (p *PythonParser) extractFunctions(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all function declarations (not indented, to avoid methods in classes)
	functionMatches := pythonFunctionRegex.FindAllStringSubmatchIndex(code, -1)

	for _, funcMatch := range functionMatches {
		if len(funcMatch) < 4 {
			continue
		}

		// Check if this is a top-level function (not indented)
		startPos := funcMatch[0]
		if startPos > 0 && code[startPos-1] != '\n' {
			// Check if there's indentation before this function
			lineStart := startPos
			for lineStart > 0 && code[lineStart-1] != '\n' {
				lineStart--
			}

			if lineStart != startPos {
				// This is an indented function (likely a method), skip it
				continue
			}
		}

		// Get the function name
		functionName := code[funcMatch[2]:funcMatch[3]]

		// Get parameters
		parameters := ""
		if funcMatch[4] != -1 && funcMatch[5] != -1 {
			parameters = code[funcMatch[4]:funcMatch[5]]
		}

		// Get the function body (indented block)
		_, endPos := p.getIndentedBlock(code, funcMatch[1], "")
		functionContent := code[startPos:endPos]

		// Get line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Check for decorators before the function
		decorators := []string{}
		for i := startPos - 1; i >= 0; i-- {
			if i == 0 || code[i-1] == '\n' {
				// Check if the previous line is a decorator
				decoratorStart := i
				if code[decoratorStart] == '@' {
					decoratorEnd := decoratorStart
					for decoratorEnd < len(code) && code[decoratorEnd] != '\n' {
						decoratorEnd++
					}

					decorator := code[decoratorStart:decoratorEnd]
					decorators = append(decorators, decorator)
					startLine = countLinesUpTo(code, decoratorStart) + 1
				} else {
					break
				}
			}
		}

		// Create function metadata
		functionMetadata := map[string]interface{}{
			"type":       "function",
			"parameters": parameters,
		}

		if len(decorators) > 0 {
			functionMetadata["decorators"] = decorators
		}

		// Create function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      functionName,
			Path:      functionName,
			Content:   functionContent,
			Language:  chunking.LanguagePython,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  functionMetadata,
		}
		functionChunk.ID = generatePythonChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}

	return chunks
}

// processDependencies analyzes chunks to identify dependencies between them
func (p *PythonParser) processDependencies(chunks []*chunking.CodeChunk) {
	// Map chunks by ID for quick lookup
	chunkMap := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}

	// Map chunks by name for dependency tracking
	chunksByName := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeClass ||
			chunk.Type == chunking.ChunkTypeFunction ||
			chunk.Type == chunking.ChunkTypeMethod {
			chunksByName[chunk.Name] = chunk
		}
	}

	// Analyze dependencies
	for _, chunk := range chunks {
		// Skip non-code chunks
		if chunk.Type != chunking.ChunkTypeClass &&
			chunk.Type != chunking.ChunkTypeFunction &&
			chunk.Type != chunking.ChunkTypeMethod {
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
			if chunk.Dependencies == nil {
				chunk.Dependencies = []string{}
			}
			chunk.Dependencies = append(chunk.Dependencies, chunk.ParentID)
		}

		// For methods, add the parent class as a dependency
		if chunk.Type == chunking.ChunkTypeMethod {
			if className, ok := chunk.Metadata["class"].(string); ok {
				for _, classChunk := range chunks {
					if classChunk.Type == chunking.ChunkTypeClass && classChunk.Name == className {
						if chunk.Dependencies == nil {
							chunk.Dependencies = []string{}
						}
						chunk.Dependencies = append(chunk.Dependencies, classChunk.ID)
						break
					}
				}
			}
		}
	}
}

// generatePythonChunkID generates a unique ID for a Python chunk
func generatePythonChunkID(chunk *chunking.CodeChunk) string {
	// Combine type, name, path, and line numbers for a unique identifier
	idString := string(chunk.Type) + ":" + chunk.Path + ":" + strconv.Itoa(chunk.StartLine) + "-" + strconv.Itoa(chunk.EndLine)

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(idString))
	return hex.EncodeToString(hash[:])
}

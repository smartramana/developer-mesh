package parsers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/S-Corkum/devops-mcp/internal/chunking"
)

// Java regular expressions for extracting code structure
var (
	// Match package declaration
	javaPackageRegex = regexp.MustCompile(`(?m)^package\s+([\w.]+)\s*;`)
	
	// Match import statements
	javaImportRegex = regexp.MustCompile(`(?m)^import\s+(?:static\s+)?([\w.]+(?:\.\*)?)\s*;`)
	
	// Match class declarations including visibility, abstract, final, etc.
	javaClassRegex = regexp.MustCompile(`(?m)^(?:public\s+|protected\s+|private\s+)?(?:abstract\s+|final\s+)?(?:static\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?(?:\s+implements\s+([^{]+))?`)
	
	// Match interface declarations
	javaInterfaceRegex = regexp.MustCompile(`(?m)^(?:public\s+|protected\s+|private\s+)?interface\s+(\w+)(?:\s+extends\s+([^{]+))?`)
	
	// Match method declarations including visibility, static, final, etc.
	javaMethodRegex = regexp.MustCompile(`(?m)^(?:public\s+|protected\s+|private\s+)?(?:abstract\s+|final\s+)?(?:static\s+)?(?:synchronized\s+)?(?:<[^>]+>\s+)?(?:[\w.<>[\]]+)\s+(\w+)\s*\(([^)]*)\)`)
	
	// Match constructor declarations
	javaConstructorRegex = regexp.MustCompile(`(?m)^(?:public\s+|protected\s+|private\s+)(\w+)\s*\(([^)]*)\)`)
	
	// Match field declarations
	javaFieldRegex = regexp.MustCompile(`(?m)^(?:public\s+|protected\s+|private\s+)?(?:static\s+|final\s+)*(?:[\w.<>[\]]+)\s+(\w+)(?:\s*=\s*[^;]+)?\s*;`)
	
	// Match JavaDoc comments
	javaDocRegex = regexp.MustCompile(`(?ms)/\*\*.*?\*/`)
	
	// Match braces for scope detection
	javaOpenBraceRegex = regexp.MustCompile(`\{`)
	javaCloseBraceRegex = regexp.MustCompile(`\}`)
)

// JavaParser is a parser for Java code
type JavaParser struct{}

// NewJavaParser creates a new JavaParser
func NewJavaParser() *JavaParser {
	return &JavaParser{}
}

// GetLanguage returns the language this parser handles
func (p *JavaParser) GetLanguage() chunking.Language {
	return chunking.LanguageJava
}

// Parse parses Java code and returns chunks
func (p *JavaParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	chunks := []*chunking.CodeChunk{}
	
	// Create a chunk for the entire file
	fileChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageJava,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata:  map[string]interface{}{},
	}
	fileChunk.ID = generateJavaChunkID(fileChunk)
	chunks = append(chunks, fileChunk)
	
	// Extract package declaration
	packageName := "default"
	packageMatches := javaPackageRegex.FindStringSubmatch(code)
	if len(packageMatches) > 1 {
		packageName = packageMatches[1]
		fileChunk.Metadata["package"] = packageName
	}
	
	// Extract imports
	importChunks := p.extractImports(code, packageName, fileChunk.ID)
	chunks = append(chunks, importChunks...)
	
	// Extract JavaDoc comments
	commentChunks := p.extractJavaDocComments(code, packageName, fileChunk.ID)
	chunks = append(chunks, commentChunks...)
	
	// Extract classes and interfaces
	classChunks, methodChunks := p.extractClasses(code, packageName, fileChunk.ID)
	chunks = append(chunks, classChunks...)
	chunks = append(chunks, methodChunks...)
	
	// Process dependencies
	p.processDependencies(chunks)
	
	return chunks, nil
}

// extractImports extracts import statements from Java code
func (p *JavaParser) extractImports(code, packageName, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all import statements
	importMatches := javaImportRegex.FindAllStringSubmatchIndex(code, -1)
	
	for i, match := range importMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the import path
		importPath := code[match[2]:match[3]]
		
		// Get the import statement
		importStatement := code[match[0]:match[1]]
		
		// Find the line numbers
		startLine := countLinesUpTo(code, match[0]) + 1
		endLine := countLinesUpTo(code, match[1]) + 1
		
		// Create import chunk
		importChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeImport,
			Name:      fmt.Sprintf("import_%d", i+1),
			Path:      fmt.Sprintf("%s:import:%s", packageName, importPath),
			Content:   importStatement,
			Language:  chunking.LanguageJava,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata: map[string]interface{}{
				"import_path": importPath,
			},
		}
		importChunk.ID = generateJavaChunkID(importChunk)
		chunks = append(chunks, importChunk)
	}
	
	return chunks
}

// extractJavaDocComments extracts JavaDoc comments from Java code
func (p *JavaParser) extractJavaDocComments(code, packageName, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all JavaDoc comments
	commentMatches := javaDocRegex.FindAllStringIndex(code, -1)
	
	for i, match := range commentMatches {
		if len(match) < 2 {
			continue
		}
		
		// Get the comment
		commentText := code[match[0]:match[1]]
		
		// Find the line numbers
		startLine := countLinesUpTo(code, match[0]) + 1
		endLine := countLinesUpTo(code, match[1]) + 1
		
		// Create comment chunk
		commentChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("JavaDoc_%d", i+1),
			Path:      fmt.Sprintf("%s:comment:%d", packageName, startLine),
			Content:   commentText,
			Language:  chunking.LanguageJava,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
		}
		commentChunk.ID = generateJavaChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}
	
	return chunks
}

// extractClasses extracts classes and interfaces from Java code
func (p *JavaParser) extractClasses(code, packageName, parentID string) ([]*chunking.CodeChunk, []*chunking.CodeChunk) {
	classChunks := []*chunking.CodeChunk{}
	methodChunks := []*chunking.CodeChunk{}
	
	// Extract classes
	classMatches := javaClassRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, classMatch := range classMatches {
		if len(classMatch) < 2 {
			continue
		}
		
		// Get the class name
		className := code[classMatch[2]:classMatch[3]]
		
		// Find the class body by counting braces
		classStartPos := classMatch[0]
		braceCount := 0
		classEndPos := classStartPos
		
		// Find opening brace
		for i := classStartPos; i < len(code); i++ {
			if code[i] == '{' {
				braceCount = 1
				classEndPos = i + 1
				break
			}
		}
		
		// Find closing brace
		for i := classEndPos; i < len(code) && braceCount > 0; i++ {
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
		
		// Get class content
		classContent := code[classStartPos:classEndPos]
		
		// Get line numbers
		startLine := countLinesUpTo(code, classStartPos) + 1
		endLine := countLinesUpTo(code, classEndPos) + 1
		
		// Get class metadata
		classMetadata := map[string]interface{}{
			"type":    "class",
			"package": packageName,
		}
		
		// Add parent class if exists
		if len(classMatch) >= 6 && classMatch[4] != -1 && classMatch[5] != -1 {
			parentClass := code[classMatch[4]:classMatch[5]]
			classMetadata["extends"] = parentClass
		}
		
		// Add interfaces if implemented
		if len(classMatch) >= 8 && classMatch[6] != -1 && classMatch[7] != -1 {
			interfaces := code[classMatch[6]:classMatch[7]]
			classMetadata["implements"] = interfaces
		}
		
		// Create class chunk
		classChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeClass,
			Name:      className,
			Path:      fmt.Sprintf("%s.%s", packageName, className),
			Content:   classContent,
			Language:  chunking.LanguageJava,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  classMetadata,
		}
		classChunk.ID = generateJavaChunkID(classChunk)
		classChunks = append(classChunks, classChunk)
		
		// Extract methods within the class
		methods := p.extractMethods(classContent, packageName, className, classChunk.ID, startLine)
		methodChunks = append(methodChunks, methods...)
	}
	
	// Extract interfaces
	interfaceMatches := javaInterfaceRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, interfaceMatch := range interfaceMatches {
		if len(interfaceMatch) < 2 {
			continue
		}
		
		// Get the interface name
		interfaceName := code[interfaceMatch[2]:interfaceMatch[3]]
		
		// Find the interface body by counting braces
		interfaceStartPos := interfaceMatch[0]
		braceCount := 0
		interfaceEndPos := interfaceStartPos
		
		// Find opening brace
		for i := interfaceStartPos; i < len(code); i++ {
			if code[i] == '{' {
				braceCount = 1
				interfaceEndPos = i + 1
				break
			}
		}
		
		// Find closing brace
		for i := interfaceEndPos; i < len(code) && braceCount > 0; i++ {
			if code[i] == '{' {
				braceCount++
			} else if code[i] == '}' {
				braceCount--
				if braceCount == 0 {
					interfaceEndPos = i + 1
					break
				}
			}
		}
		
		// Get interface content
		interfaceContent := code[interfaceStartPos:interfaceEndPos]
		
		// Get line numbers
		startLine := countLinesUpTo(code, interfaceStartPos) + 1
		endLine := countLinesUpTo(code, interfaceEndPos) + 1
		
		// Get interface metadata
		interfaceMetadata := map[string]interface{}{
			"type":    "interface",
			"package": packageName,
		}
		
		// Add parent interfaces if extended
		if len(interfaceMatch) >= 6 && interfaceMatch[4] != -1 && interfaceMatch[5] != -1 {
			parentInterfaces := code[interfaceMatch[4]:interfaceMatch[5]]
			interfaceMetadata["extends"] = parentInterfaces
		}
		
		// Create interface chunk
		interfaceChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeInterface,
			Name:      interfaceName,
			Path:      fmt.Sprintf("%s.%s", packageName, interfaceName),
			Content:   interfaceContent,
			Language:  chunking.LanguageJava,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  interfaceMetadata,
		}
		interfaceChunk.ID = generateJavaChunkID(interfaceChunk)
		classChunks = append(classChunks, interfaceChunk)
	}
	
	return classChunks, methodChunks
}

// extractMethods extracts methods and constructors from a class body
func (p *JavaParser) extractMethods(classContent, packageName, className, parentID string, classStartLine int) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Extract methods
	methodMatches := javaMethodRegex.FindAllStringSubmatchIndex(classContent, -1)
	
	for _, methodMatch := range methodMatches {
		if len(methodMatch) < 4 {
			continue
		}
		
		// Get method name
		methodName := classContent[methodMatch[2]:methodMatch[3]]
		
		// Get parameters
		parameters := ""
		if methodMatch[4] != -1 && methodMatch[5] != -1 {
			parameters = classContent[methodMatch[4]:methodMatch[5]]
		}
		
		// Find the method body by counting braces
		methodStartPos := methodMatch[0]
		braceCount := 0
		methodEndPos := methodStartPos
		
		// Find opening brace
		for i := methodStartPos; i < len(classContent); i++ {
			if classContent[i] == '{' {
				braceCount = 1
				methodEndPos = i + 1
				break
			} else if classContent[i] == ';' { // Abstract method
				methodEndPos = i + 1
				break
			}
		}
		
		// If not abstract method, find closing brace
		if braceCount > 0 {
			for i := methodEndPos; i < len(classContent) && braceCount > 0; i++ {
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
		}
		
		// Get method content
		methodContent := classContent[methodStartPos:methodEndPos]
		
		// Get line numbers
		startLine := countLinesUpTo(classContent, methodStartPos) + 1 + classStartLine - 1
		endLine := countLinesUpTo(classContent, methodEndPos) + 1 + classStartLine - 1
		
		// Create method metadata
		methodMetadata := map[string]interface{}{
			"type":       "method",
			"parameters": parameters,
			"class":      className,
		}
		
		// Create method chunk
		methodChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeMethod,
			Name:      methodName,
			Path:      fmt.Sprintf("%s.%s.%s", packageName, className, methodName),
			Content:   methodContent,
			Language:  chunking.LanguageJava,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  methodMetadata,
		}
		methodChunk.ID = generateJavaChunkID(methodChunk)
		chunks = append(chunks, methodChunk)
	}
	
	// Extract constructors
	constructorMatches := javaConstructorRegex.FindAllStringSubmatchIndex(classContent, -1)
	
	for _, constructorMatch := range constructorMatches {
		if len(constructorMatch) < 4 {
			continue
		}
		
		// Get constructor name
		constructorName := classContent[constructorMatch[2]:constructorMatch[3]]
		
		// Skip if not a constructor of this class
		if constructorName != className {
			continue
		}
		
		// Get parameters
		parameters := ""
		if constructorMatch[4] != -1 && constructorMatch[5] != -1 {
			parameters = classContent[constructorMatch[4]:constructorMatch[5]]
		}
		
		// Find the constructor body by counting braces
		constructorStartPos := constructorMatch[0]
		braceCount := 0
		constructorEndPos := constructorStartPos
		
		// Find opening brace
		for i := constructorStartPos; i < len(classContent); i++ {
			if classContent[i] == '{' {
				braceCount = 1
				constructorEndPos = i + 1
				break
			}
		}
		
		// Find closing brace
		for i := constructorEndPos; i < len(classContent) && braceCount > 0; i++ {
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
		
		// Get constructor content
		constructorContent := classContent[constructorStartPos:constructorEndPos]
		
		// Get line numbers
		startLine := countLinesUpTo(classContent, constructorStartPos) + 1 + classStartLine - 1
		endLine := countLinesUpTo(classContent, constructorEndPos) + 1 + classStartLine - 1
		
		// Create constructor metadata
		constructorMetadata := map[string]interface{}{
			"type":       "constructor",
			"parameters": parameters,
			"class":      className,
		}
		
		// Create constructor chunk
		constructorChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeMethod,  // Use method type for constructors
			Name:      constructorName,
			Path:      fmt.Sprintf("%s.%s.%s", packageName, className, constructorName),
			Content:   constructorContent,
			Language:  chunking.LanguageJava,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  constructorMetadata,
		}
		constructorChunk.ID = generateJavaChunkID(constructorChunk)
		chunks = append(chunks, constructorChunk)
	}
	
	return chunks
}

// processDependencies analyzes chunks to identify dependencies between them
func (p *JavaParser) processDependencies(chunks []*chunking.CodeChunk) {
	// Map chunks by ID for quick lookup
	chunkMap := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}
	
	// Map chunks by name for dependency tracking
	chunksByName := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeClass || 
		   chunk.Type == chunking.ChunkTypeInterface ||
		   chunk.Type == chunking.ChunkTypeMethod {
			chunksByName[chunk.Name] = chunk
		}
	}
	
	// Analyze dependencies
	for _, chunk := range chunks {
		// Skip non-code chunks
		if chunk.Type != chunking.ChunkTypeClass && 
		   chunk.Type != chunking.ChunkTypeInterface &&
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
	}
}



// generateJavaChunkID generates a unique ID for a Java chunk
func generateJavaChunkID(chunk *chunking.CodeChunk) string {
	// Combine type, name, path, and line numbers for a unique identifier
	idString := string(chunk.Type) + ":" + chunk.Path + ":" + strconv.Itoa(chunk.StartLine) + "-" + strconv.Itoa(chunk.EndLine)
	
	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(idString))
	return hex.EncodeToString(hash[:])
}

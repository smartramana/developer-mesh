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

// Regex patterns for TypeScript code elements
var (
	// Match import statements (same as JavaScript)
	tsImportRegex = regexp.MustCompile(`(?m)^(?:import\s+(?:\{[^}]*\}|\*\s+as\s+\w+|\w+)\s+from\s+['"]([^'"]+)['"]|const\s+(?:\{[^}]*\}|\w+)\s*=\s*require\(['"]([^'"]+)['"]\))`)

	// Match class declarations with type parameters
	tsClassRegex = regexp.MustCompile(`(?m)^(?:export\s+)?(?:abstract\s+)?class\s+(\w+)(?:<[^>]*>)?(?:\s+extends\s+(?:\w+)(?:<[^>]*>)?)?(?:\s+implements\s+(?:\w+)(?:<[^>]*>)?(?:\s*,\s*(?:\w+)(?:<[^>]*>)?)*)?\s*\{`)

	// Match decorators (commonly used in Angular, NestJS, TypeORM, etc.)
	tsDecoratorRegex = regexp.MustCompile(`(?m)^\s*@(\w+)(?:\(([^)]*)\))?`)

	// Match interface declarations
	tsInterfaceRegex = regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)(?:<[^>]*>)?(?:\s+extends\s+(?:\w+)(?:<[^>]*>)?(?:\s*,\s*(?:\w+)(?:<[^>]*>)?)*)?\s*\{`)

	// Match type alias declarations
	tsTypeAliasRegex = regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)(?:<[^>]*>)?\s*=\s*(.+?)(?:;|$)`)

	// Match enum declarations
	tsEnumRegex = regexp.MustCompile(`(?m)^(?:export\s+)?(?:const\s+)?enum\s+(\w+)\s*\{`)

	// Match namespace declarations
	tsNamespaceRegex = regexp.MustCompile(`(?m)^(?:export\s+)?namespace\s+(\w+)\s*\{`)

	// Match function declarations with type annotations
	tsFunctionRegex = regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)(?:<[^>]*>)?\s*\(([^)]*)\)(?:\s*:\s*([^{;]*))?`)

	// Match arrow functions assigned to variables with type annotations
	tsArrowFunctionRegex = regexp.MustCompile(`(?m)^(?:export\s+)?(?:const|let|var)\s+(\w+)(?:\s*:\s*([^=]*))?\s*=\s*(?:async\s*)?(?:\([^)]*\)|\w+)\s*=>`)

	// Match method declarations inside classes with type annotations
	tsMethodRegex = regexp.MustCompile(`(?m)^\s+(?:private\s+|protected\s+|public\s+|readonly\s+|static\s+|abstract\s+|async\s+)*\s*(\w+)(?:<[^>]*>)?\s*\(([^)]*)\)(?:\s*:\s*([^{;]*))?`)

	// Match property declarations in classes with type annotations
	tsPropertyRegex = regexp.MustCompile(`(?m)^\s+(?:private\s+|protected\s+|public\s+|readonly\s+|static\s+)*\s*(\w+)\s*:\s*([^=;]+)(?:\s*=\s*([^;]+))?;?`)

	// Match JSDoc comments (same as JavaScript)
	tsDocRegex = regexp.MustCompile(`(?ms)/\\*\\*.*?\\*/`)
)

// TypeScriptParser handles parsing TypeScript code
type TypeScriptParser struct{}

// NewTypeScriptParser creates a new TypeScript parser instance
func NewTypeScriptParser() *TypeScriptParser {
	return &TypeScriptParser{}
}

// GetLanguage returns the language this parser handles
func (p *TypeScriptParser) GetLanguage() chunking.Language {
	return chunking.LanguageTypeScript
}

// generateTSChunkID generates a unique ID for a TypeScript chunk
func generateTSChunkID(chunk *chunking.CodeChunk) string {
	// Combine type, name, path, and line numbers for a unique identifier
	idString := string(chunk.Type) + ":" + chunk.Path + ":" + strconv.Itoa(chunk.StartLine) + "-" + strconv.Itoa(chunk.EndLine)

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(idString))
	return hex.EncodeToString(hash[:])
}

// Parse parses TypeScript code and returns chunks
// extractInterfaces extracts interface declarations from TypeScript code
func (p *TypeScriptParser) extractInterfaces(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all interface declarations
	interfaceMatches := tsInterfaceRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range interfaceMatches {
		if len(match) < 4 {
			continue
		}

		// Get the interface name
		interfaceName := code[match[2]:match[3]]

		// Find the interface content
		startPos := match[0]
		interfaceContent, endPos := p.findBlockContent(code, startPos, -1)

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Determine if the interface extends other interfaces
		var extendedInterfaces string
		extendsMatch := regexp.MustCompile(`extends\s+([^{]+)`).FindStringSubmatch(interfaceContent)
		if len(extendsMatch) > 1 {
			extendedInterfaces = strings.TrimSpace(extendsMatch[1])
		}

		// Create interface metadata
		interfaceMetadata := map[string]interface{}{
			"type": "interface",
		}
		if extendedInterfaces != "" {
			interfaceMetadata["extends"] = extendedInterfaces
		}

		// Create interface chunk
		interfaceChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeInterface,
			Name:      interfaceName,
			Path:      interfaceName,
			Content:   interfaceContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  interfaceMetadata,
		}
		interfaceChunk.ID = generateTSChunkID(interfaceChunk)
		chunks = append(chunks, interfaceChunk)
	}

	return chunks
}

// extractTypeAliases extracts type alias declarations from TypeScript code
func (p *TypeScriptParser) extractTypeAliases(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all type alias declarations
	typeAliasMatches := tsTypeAliasRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range typeAliasMatches {
		if len(match) < 6 {
			continue
		}

		// Get the type alias name
		typeName := code[match[2]:match[3]]

		// Get the type definition
		typeValue := code[match[4]:match[5]]

		// Find the complete statement
		startPos := match[0]
		endPos := match[1]
		typeContent := code[startPos:endPos]

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Create type alias metadata
		typeMetadata := map[string]interface{}{
			"type":  "type_alias",
			"value": typeValue,
		}

		// Create type alias chunk
		typeChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block since there's no specific TypeAlias type
			Name:      typeName,
			Path:      fmt.Sprintf("type:%s", typeName),
			Content:   typeContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  typeMetadata,
		}
		typeChunk.ID = generateTSChunkID(typeChunk)
		chunks = append(chunks, typeChunk)
	}

	return chunks
}

// extractEnums extracts enum declarations from TypeScript code
func (p *TypeScriptParser) extractEnums(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all enum declarations
	enumMatches := tsEnumRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range enumMatches {
		if len(match) < 4 {
			continue
		}

		// Get the enum name
		enumName := code[match[2]:match[3]]

		// Find the enum content
		startPos := match[0]
		enumContent, endPos := p.findBlockContent(code, startPos, -1)

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Extract enum members
		enumsMembers := []map[string]string{}
		memberMatch := regexp.MustCompile(`(\w+)(?:\s*=\s*([^,}]*))?`).FindAllStringSubmatch(enumContent, -1)
		for _, m := range memberMatch {
			if len(m) > 1 {
				member := map[string]string{"name": m[1]}
				if len(m) > 2 && m[2] != "" {
					member["value"] = strings.TrimSpace(m[2])
				}
				enumsMembers = append(enumsMembers, member)
			}
		}

		// Create enum metadata
		enumMetadata := map[string]interface{}{
			"type":    "enum",
			"members": enumsMembers,
		}

		// Check if this is a const enum
		isConst := regexp.MustCompile(`const\s+enum`).MatchString(enumContent)
		if isConst {
			enumMetadata["const"] = true
		}

		// Create enum chunk
		enumChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block since there's no specific Enum type
			Name:      enumName,
			Path:      fmt.Sprintf("enum:%s", enumName),
			Content:   enumContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  enumMetadata,
		}
		enumChunk.ID = generateTSChunkID(enumChunk)
		chunks = append(chunks, enumChunk)
	}

	return chunks
}

// Parse parses TypeScript code and returns chunks
func (p *TypeScriptParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	// Create a chunk for the entire file
	fileChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageTypeScript,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata:  map[string]interface{}{},
	}
	fileChunk.ID = generateTSChunkID(fileChunk)

	// Split code into lines for easier processing
	lines := strings.Split(code, "\n")

	// Extract all code chunks
	allChunks := []*chunking.CodeChunk{fileChunk}

	// Extract imports
	importChunks := p.extractImports(code, lines, fileChunk.ID)
	allChunks = append(allChunks, importChunks...)

	// Extract JSDoc comments
	commentChunks := p.extractTSDocComments(code, lines, fileChunk.ID)
	allChunks = append(allChunks, commentChunks...)

	// Extract decorators
	decoratorChunks := p.extractDecorators(code, lines, fileChunk.ID)
	allChunks = append(allChunks, decoratorChunks...)

	// Extract interfaces
	interfaceChunks := p.extractInterfaces(code, lines, fileChunk.ID)
	allChunks = append(allChunks, interfaceChunks...)

	// Extract type aliases
	typeAliasChunks := p.extractTypeAliases(code, lines, fileChunk.ID)
	allChunks = append(allChunks, typeAliasChunks...)

	// Extract enums
	enumChunks := p.extractEnums(code, lines, fileChunk.ID)
	allChunks = append(allChunks, enumChunks...)

	// Extract namespaces
	namespaceChunks := p.extractNamespaces(code, lines, fileChunk.ID)
	allChunks = append(allChunks, namespaceChunks...)

	// Extract classes, methods, and properties
	classChunks, methodChunks, propertyChunks := p.extractClasses(code, lines, fileChunk.ID)
	allChunks = append(allChunks, classChunks...)
	allChunks = append(allChunks, methodChunks...)
	allChunks = append(allChunks, propertyChunks...)

	// Extract functions
	functionChunks := p.extractFunctions(code, lines, fileChunk.ID)
	allChunks = append(allChunks, functionChunks...)

	// Process dependencies between chunks
	p.processDependencies(allChunks)

	return allChunks, nil
}

// extractImports extracts import statements from TypeScript code
func (p *TypeScriptParser) extractImports(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all import statements
	importMatches := tsImportRegex.FindAllStringSubmatchIndex(code, -1)

	for i, match := range importMatches {
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
			importPath = fmt.Sprintf("import_%d", i+1)
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
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata: map[string]interface{}{
				"import_path": importPath,
			},
		}
		importChunk.ID = generateTSChunkID(importChunk)
		chunks = append(chunks, importChunk)
	}

	return chunks
}

// extractTSDocComments extracts JSDoc/TSDoc comments from TypeScript code
func (p *TypeScriptParser) extractTSDocComments(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all documentation comments
	docMatches := tsDocRegex.FindAllStringIndex(code, -1)

	for i, match := range docMatches {
		if len(match) < 2 {
			continue
		}

		// Get the comment content
		commentStartPos := match[0]
		commentEndPos := match[1]
		commentContent := code[commentStartPos:commentEndPos]

		// Find the line numbers
		startLine := getLineNumberFromPos(code, commentStartPos) + 1
		endLine := getLineNumberFromPos(code, commentEndPos) + 1

		// Extract JSDoc metadata
		metadata := extractJSDocMetadata(commentContent)

		// Create chunk for the comment
		commentChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("comment-%d", i),
			Path:      fmt.Sprintf("comment-%d", i),
			Content:   commentContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  metadata,
		}
		commentChunk.ID = generateTSChunkID(commentChunk)
		chunks = append(chunks, commentChunk)

		// Look for the next code item after this comment to associate them
		if commentEndPos < len(code) {
			// Check if there's a code item after this comment (class, interface, function, etc.)
			afterComment := code[commentEndPos:]
			lineAfterComment := getLineNumberFromPos(code, commentEndPos) + 1

			// Patterns to check for
			patterns := []struct {
				regex   *regexp.Regexp
				typeStr string
			}{
				{tsClassRegex, "class"},
				{tsInterfaceRegex, "interface"},
				{tsEnumRegex, "enum"},
				{tsFunctionRegex, "function"},
				{tsTypeAliasRegex, "type"},
				{tsMethodRegex, "method"},
				{tsPropertyRegex, "property"},
			}

			for _, pattern := range patterns {
				if matches := pattern.regex.FindStringSubmatchIndex(afterComment); matches != nil && len(matches) >= 2 {
					// Check if the match is within a few lines of the comment (to ensure they're related)
					matchPos := commentEndPos + matches[0]
					matchLine := getLineNumberFromPos(code, matchPos) + 1

					if matchLine-lineAfterComment <= 3 { // Allow up to 3 lines between comment and code
						commentChunk.Metadata["describes"] = pattern.typeStr
						break
					}
				}
			}
		}
	}

	return chunks
}

// extractJSDocMetadata parses JSDoc content and extracts meaningful metadata
func extractJSDocMetadata(comment string) map[string]interface{} {
	metadata := map[string]interface{}{
		"type": "jsdoc",
	}

	// Extract @param tags
	paramRegex := regexp.MustCompile(`@param\s+(?:\{([^}]+)\}\s+)?([\w$]+)(?:\s+(.+))?`)
	paramMatches := paramRegex.FindAllStringSubmatch(comment, -1)

	if len(paramMatches) > 0 {
		params := []map[string]string{}
		for _, match := range paramMatches {
			param := map[string]string{}
			if len(match) > 1 && match[1] != "" {
				param["type"] = strings.TrimSpace(match[1])
			}
			if len(match) > 2 {
				param["name"] = strings.TrimSpace(match[2])
			}
			if len(match) > 3 && match[3] != "" {
				param["description"] = strings.TrimSpace(match[3])
			}
			params = append(params, param)
		}
		metadata["params"] = params
	}

	// Extract @returns tag
	returnsRegex := regexp.MustCompile(`@returns?\s+(?:\{([^}]+)\}\s*)?(.+)?`)
	returnsMatch := returnsRegex.FindStringSubmatch(comment)
	if len(returnsMatch) > 1 {
		returns := map[string]string{}
		if returnsMatch[1] != "" {
			returns["type"] = strings.TrimSpace(returnsMatch[1])
		}
		if len(returnsMatch) > 2 && returnsMatch[2] != "" {
			returns["description"] = strings.TrimSpace(returnsMatch[2])
		}
		metadata["returns"] = returns
	}

	// Extract @example tag
	exampleRegex := regexp.MustCompile(`@example\s+([\s\S]+?)(?:@|\*/)`)
	exampleMatch := exampleRegex.FindStringSubmatch(comment)
	if len(exampleMatch) > 1 {
		metadata["example"] = strings.TrimSpace(exampleMatch[1])
	}

	// Extract @deprecated tag
	if strings.Contains(comment, "@deprecated") {
		metadata["deprecated"] = true
		// Try to extract deprecation message
		deprecatedRegex := regexp.MustCompile(`@deprecated\s+(.+?)(?:@|\*/)`)
		deprecatedMatch := deprecatedRegex.FindStringSubmatch(comment)
		if len(deprecatedMatch) > 1 {
			metadata["deprecation_message"] = strings.TrimSpace(deprecatedMatch[1])
		}
	}

	// Extract description (content before any tags)
	descRegex := regexp.MustCompile(`/\*\*\s*\n?(.*?)(?:@|\*/)`)
	descMatch := descRegex.FindStringSubmatch(comment)
	if len(descMatch) > 1 {
		// Clean up the description
		desc := descMatch[1]
		// Remove leading asterisks and whitespace from each line
		lines := strings.Split(desc, "\n")
		for i, line := range lines {
			lines[i] = regexp.MustCompile(`^\s*\*?\s*`).ReplaceAllString(line, "")
		}
		cleanDesc := strings.TrimSpace(strings.Join(lines, " "))
		if cleanDesc != "" {
			metadata["description"] = cleanDesc
		}
	}

	return metadata
}

// extractDecorators extracts decorator declarations from TypeScript code
func (p *TypeScriptParser) extractDecorators(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all decorator declarations
	decoratorMatches := tsDecoratorRegex.FindAllStringSubmatchIndex(code, -1)

	for i, match := range decoratorMatches {
		if len(match) < 4 {
			continue
		}

		// Get the decorator name
		decoratorName := code[match[2]:match[3]]

		// Get the decorator arguments if available
		var decoratorArgs string
		if len(match) >= 6 {
			decoratorArgs = code[match[4]:match[5]]
		}

		// Get the full decorator declaration
		startPos := match[0]
		endPos := match[1]
		decoratorContent := code[startPos:endPos]

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Try to determine what the decorator is applied to (class, method, property, etc.)
		targetType := "unknown"
		targetName := ""

		// Look at code after the decorator to identify what it's decorating
		afterDecorator := code[endPos:]

		// Check for different target types
		targetPatterns := map[string]*regexp.Regexp{
			"class":     regexp.MustCompile(`^\s*(?:export\s+)?class\s+(\w+)`),
			"property":  regexp.MustCompile(`^\s*(?:private|public|protected|readonly|static)?\s*(\w+)\s*:`),
			"method":    regexp.MustCompile(`^\s*(?:private|public|protected|static|async)?\s*(\w+)\s*\(`),
			"parameter": regexp.MustCompile(`^\s*(\w+)\s*:`),
		}

		for typeName, pattern := range targetPatterns {
			if match := pattern.FindStringSubmatch(afterDecorator); len(match) > 1 {
				targetType = typeName
				targetName = match[1]
				break
			}
		}

		// Create decorator metadata
		decoratorMetadata := map[string]interface{}{
			"type":   "decorator",
			"target": targetType,
		}

		if targetName != "" {
			decoratorMetadata["target_name"] = targetName
		}

		if decoratorArgs != "" {
			decoratorMetadata["arguments"] = decoratorArgs
		}

		// Create decorator chunk
		decoratorChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block since there's no specific Decorator type
			Name:      fmt.Sprintf("%s-decorator-%d", decoratorName, i),
			Path:      fmt.Sprintf("decorator:%s-%d", decoratorName, i),
			Content:   decoratorContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  decoratorMetadata,
		}
		decoratorChunk.ID = generateTSChunkID(decoratorChunk)
		chunks = append(chunks, decoratorChunk)
	}

	return chunks
}

// extractNamespaces extracts namespace declarations from TypeScript code
func (p *TypeScriptParser) extractNamespaces(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all namespace declarations
	namespaceMatches := tsNamespaceRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range namespaceMatches {
		if len(match) < 4 {
			continue
		}

		// Get the namespace name
		namespaceName := code[match[2]:match[3]]

		// Find the namespace content
		startPos := match[0]
		namespaceContent, endPos := p.findBlockContent(code, startPos, -1)

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Create namespace metadata
		namespaceMetadata := map[string]interface{}{
			"type": "namespace",
		}

		// Create namespace chunk
		namespaceChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block since there's no specific Namespace type
			Name:      namespaceName,
			Path:      fmt.Sprintf("namespace:%s", namespaceName),
			Content:   namespaceContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  namespaceMetadata,
		}
		namespaceChunk.ID = generateTSChunkID(namespaceChunk)
		chunks = append(chunks, namespaceChunk)
	}

	return chunks
}

// extractClasses extracts classes and their methods/properties from TypeScript code
func (p *TypeScriptParser) extractClasses(code string, lines []string, parentID string) ([]*chunking.CodeChunk, []*chunking.CodeChunk, []*chunking.CodeChunk) {
	classChunks := []*chunking.CodeChunk{}
	methodChunks := []*chunking.CodeChunk{}
	propertyChunks := []*chunking.CodeChunk{}

	// Find all class declarations
	classMatches := tsClassRegex.FindAllStringSubmatchIndex(code, -1)

	for _, classMatch := range classMatches {
		if len(classMatch) < 4 {
			continue
		}

		// Get the class name
		className := code[classMatch[2]:classMatch[3]]

		// Find the class body (indented block)
		startPos := classMatch[0]
		classContent, endPos := p.findBlockContent(code, startPos, -1)

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Determine if this is an abstract class
		isAbstract := regexp.MustCompile(`abstract\s+class`).MatchString(classContent)

		// Determine if the class extends another class
		var parentClass string
		extendsMatch := regexp.MustCompile(`extends\s+([^\s<{]+)`).FindStringSubmatch(classContent)
		if len(extendsMatch) > 1 {
			parentClass = strings.TrimSpace(extendsMatch[1])
		}

		// Determine if the class implements interfaces
		var implementedInterfaces string
		implementsMatch := regexp.MustCompile(`implements\s+([^{]+)`).FindStringSubmatch(classContent)
		if len(implementsMatch) > 1 {
			implementedInterfaces = strings.TrimSpace(implementsMatch[1])
		}

		// Create class metadata
		classMetadata := map[string]interface{}{
			"type": "class",
		}
		if isAbstract {
			classMetadata["abstract"] = true
		}
		if parentClass != "" {
			classMetadata["extends"] = parentClass
		}
		if implementedInterfaces != "" {
			classMetadata["implements"] = implementedInterfaces
		}

		// Create class chunk
		classChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeClass,
			Name:      className,
			Path:      className,
			Content:   classContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  classMetadata,
		}
		classChunk.ID = generateTSChunkID(classChunk)
		classChunks = append(classChunks, classChunk)

		// Extract methods from class
		methods := p.extractMethods(classContent, classChunk.ID, className, startLine)
		methodChunks = append(methodChunks, methods...)

		// Extract properties from class
		properties := p.extractProperties(classContent, classChunk.ID, className, startLine)
		propertyChunks = append(propertyChunks, properties...)
	}

	return classChunks, methodChunks, propertyChunks
}

// extractMethods extracts methods from a class
func (p *TypeScriptParser) extractMethods(classContent string, parentID, className string, classStartLine int) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all method declarations
	methodMatches := tsMethodRegex.FindAllStringSubmatchIndex(classContent, -1)

	for _, methodMatch := range methodMatches {
		if len(methodMatch) < 6 {
			continue
		}

		// Get the method name
		methodName := classContent[methodMatch[2]:methodMatch[3]]

		// Get parameters
		params := classContent[methodMatch[4]:methodMatch[5]]

		// Find the method body (indented block)
		startPos := methodMatch[0]
		methodContent, endPos := p.findBlockContent(classContent, startPos, -1)

		// Find the line numbers relative to the file
		startLine := getLineNumberFromPos(classContent, startPos) + 1 + classStartLine - 1
		endLine := getLineNumberFromPos(classContent, endPos) + 1 + classStartLine - 1

		// Determine method modifiers (public, private, etc.)
		modifiers := []string{}
		modifierRegex := regexp.MustCompile(`(private|public|protected|readonly|static|abstract|async)\s+`)
		modifierMatches := modifierRegex.FindAllString(methodContent, -1)
		for _, m := range modifierMatches {
			modifiers = append(modifiers, strings.TrimSpace(m))
		}

		// Get return type if specified
		var returnType string
		returnTypeMatch := regexp.MustCompile(`\)\s*:\s*([^{;]+)`).FindStringSubmatch(methodContent)
		if len(returnTypeMatch) > 1 {
			returnType = strings.TrimSpace(returnTypeMatch[1])
		}

		// Create method metadata
		methodMetadata := map[string]interface{}{
			"type":       "method",
			"parameters": params,
			"class":      className,
		}

		if len(modifiers) > 0 {
			methodMetadata["modifiers"] = modifiers
		}

		if returnType != "" {
			methodMetadata["return_type"] = returnType
		}

		// Create method chunk
		methodChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeMethod,
			Name:      methodName,
			Path:      fmt.Sprintf("%s.%s", className, methodName),
			Content:   methodContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  methodMetadata,
		}
		methodChunk.ID = generateTSChunkID(methodChunk)
		chunks = append(chunks, methodChunk)
	}

	return chunks
}

// extractProperties extracts property declarations from a class
func (p *TypeScriptParser) extractProperties(classContent string, parentID, className string, classStartLine int) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all property declarations
	propertyMatches := tsPropertyRegex.FindAllStringSubmatchIndex(classContent, -1)

	for _, propertyMatch := range propertyMatches {
		if len(propertyMatch) < 6 {
			continue
		}

		// Get the property name
		propertyName := classContent[propertyMatch[2]:propertyMatch[3]]

		// Get property type
		propertyType := classContent[propertyMatch[4]:propertyMatch[5]]

		// Find the complete declaration
		startPos := propertyMatch[0]
		propertyContent := classContent[startPos:propertyMatch[1]]

		// Find the line numbers
		startLine := getLineNumberFromPos(classContent, startPos) + 1 + classStartLine - 1
		endLine := startLine // Properties are typically one line

		// Determine property modifiers (public, private, etc.)
		modifiers := []string{}
		modifierRegex := regexp.MustCompile(`(private|public|protected|readonly|static)\s+`)
		modifierMatches := modifierRegex.FindAllString(propertyContent, -1)
		for _, m := range modifierMatches {
			modifiers = append(modifiers, strings.TrimSpace(m))
		}

		// Create property metadata
		propertyMetadata := map[string]interface{}{
			"type":          "property",
			"property_type": propertyType,
			"class":         className,
		}

		if len(modifiers) > 0 {
			propertyMetadata["modifiers"] = modifiers
		}

		// Create property chunk
		propertyChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block since there's no specific Property type
			Name:      propertyName,
			Path:      fmt.Sprintf("%s.%s", className, propertyName),
			Content:   propertyContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  propertyMetadata,
		}
		propertyChunk.ID = generateTSChunkID(propertyChunk)
		chunks = append(chunks, propertyChunk)
	}

	return chunks
}

// extractFunctions extracts standalone functions from TypeScript code
func (p *TypeScriptParser) extractFunctions(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all function declarations
	functionMatches := tsFunctionRegex.FindAllStringSubmatchIndex(code, -1)

	for _, functionMatch := range functionMatches {
		if len(functionMatch) < 6 {
			continue
		}

		// Get the function name
		functionName := code[functionMatch[2]:functionMatch[3]]

		// Get parameters
		params := code[functionMatch[4]:functionMatch[5]]

		// Find the function body
		startPos := functionMatch[0]
		functionContent, endPos := p.findBlockContent(code, startPos, -1)

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Determine if this is an async function
		isAsync := regexp.MustCompile(`async\s+function`).MatchString(functionContent)

		// Get return type if specified
		var returnType string
		returnTypeMatch := regexp.MustCompile(`\)\s*:\s*([^{;]+)`).FindStringSubmatch(functionContent)
		if len(returnTypeMatch) > 1 {
			returnType = strings.TrimSpace(returnTypeMatch[1])
		}

		// Create function metadata
		functionMetadata := map[string]interface{}{
			"type":       "function",
			"parameters": params,
		}

		if isAsync {
			functionMetadata["async"] = true
		}

		if returnType != "" {
			functionMetadata["return_type"] = returnType
		}

		// Create function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      functionName,
			Path:      functionName,
			Content:   functionContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  functionMetadata,
		}
		functionChunk.ID = generateTSChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}

	// Find all arrow functions assigned to variables
	arrowFunctionMatches := tsArrowFunctionRegex.FindAllStringSubmatchIndex(code, -1)

	for _, arrowMatch := range arrowFunctionMatches {
		if len(arrowMatch) < 4 {
			continue
		}

		// Get the function name (variable name)
		functionName := code[arrowMatch[2]:arrowMatch[3]]

		// Find the complete statement
		startPos := arrowMatch[0]
		endPos := startPos
		braceCount := 0
		insideBlock := false

		// Find the end of the arrow function by looking for the closing brace or semicolon
		for endPos < len(code) {
			if code[endPos] == '{' {
				braceCount++
				insideBlock = true
			} else if code[endPos] == '}' {
				braceCount--
				if braceCount == 0 && insideBlock {
					endPos++
					break
				}
			} else if code[endPos] == ';' && !insideBlock {
				endPos++
				break
			}
			endPos++
		}

		// Get the function content
		functionContent := code[startPos:endPos]

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Determine if this is an async arrow function
		isAsync := regexp.MustCompile(`=\s*async\s*(?:\(|\w+\s*=>)`).MatchString(functionContent)

		// Get type annotation if specified
		var typeAnnotation string
		typeMatch := regexp.MustCompile(`:\s*([^=]+)\s*=`).FindStringSubmatch(functionContent)
		if len(typeMatch) > 1 {
			typeAnnotation = strings.TrimSpace(typeMatch[1])
		}

		// Create function metadata
		functionMetadata := map[string]interface{}{
			"type":           "function",
			"arrow_function": true,
		}

		if isAsync {
			functionMetadata["async"] = true
		}

		if typeAnnotation != "" {
			functionMetadata["type_annotation"] = typeAnnotation
		}

		// Create function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      functionName,
			Path:      functionName,
			Content:   functionContent,
			Language:  chunking.LanguageTypeScript,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  functionMetadata,
		}
		functionChunk.ID = generateTSChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}

	return chunks
}

// processDependencies analyzes chunks to identify dependencies between them
func (p *TypeScriptParser) processDependencies(chunks []*chunking.CodeChunk) {
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
			chunk.Type == chunking.ChunkTypeClass ||
			chunk.Type == chunking.ChunkTypeInterface {
			chunksByName[chunk.Name] = chunk
		}
	}

	// Analyze dependencies
	for _, chunk := range chunks {
		// Skip non-code chunks
		if chunk.Type != chunking.ChunkTypeFunction &&
			chunk.Type != chunking.ChunkTypeMethod &&
			chunk.Type != chunking.ChunkTypeClass &&
			chunk.Type != chunking.ChunkTypeInterface {
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

// Helper function to find the content of a block (includes nested blocks)
func (p *TypeScriptParser) findBlockContent(code string, startPos, openBracePos int) (string, int) {
	if openBracePos == -1 {
		// Find the opening brace
		for openBracePos = startPos; openBracePos < len(code) && code[openBracePos] != '{'; openBracePos++ {
		}

		if openBracePos >= len(code) {
			return "", startPos
		}
	}

	// Count braces to find the matching closing brace
	braceCount := 1
	endPos := openBracePos + 1

	for endPos < len(code) && braceCount > 0 {
		if code[endPos] == '{' {
			braceCount++
		} else if code[endPos] == '}' {
			braceCount--
		}
		endPos++
	}

	return code[startPos:endPos], endPos
}

// Rust parser for parsing Rust code into code chunks
package parsers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// Regex patterns for Rust code elements
var (
	// Match module declarations
	rustModuleRegex = regexp.MustCompile(`(?m)^(?:pub(?:\s*\([^)]*\))?\s+)?mod\s+(\w+)(?:\s*\{|;)`)

	// Match struct declarations
	rustStructRegex = regexp.MustCompile(`(?m)^(?:pub(?:\s*\([^)]*\))?\s+)?struct\s+(\w+)(?:<[^>]*>)?\s*(?:\{|;|where)`)

	// Match enum declarations
	rustEnumRegex = regexp.MustCompile(`(?m)^(?:pub(?:\s*\([^)]*\))?\s+)?enum\s+(\w+)(?:<[^>]*>)?\s*(?:\{|where)`)

	// Match trait declarations
	rustTraitRegex = regexp.MustCompile(`(?m)^(?:pub(?:\s*\([^)]*\))?\s+)?trait\s+(\w+)(?:<[^>]*>)?(?:\s*:\s*[^{]+)?\s*\{`)

	// Match impl blocks
	rustImplRegex = regexp.MustCompile(`(?m)^impl(?:<[^>]*>)?\s+(?:(?:\w+::)*\w+)?(?:<[^>]*>)?\s+for\s+(?:(?:\w+::)*\w+)(?:<[^>]*>)?\s*(?:where\s+[^{]+)?\s*\{`)
	rustImplRegex2 = regexp.MustCompile(`(?m)^impl(?:<[^>]*>)?\s+(?:(?:\w+::)*\w+)(?:<[^>]*>)?\s*(?:where\s+[^{]+)?\s*\{`)

	// Match function declarations
	rustFunctionRegex = regexp.MustCompile(`(?m)^(?:pub(?:\s*\([^)]*\))?\s+)?(?:async\s+)?fn\s+(\w+)(?:<[^>]*>)?\s*\(([^)]*)\)(?:\s*->\s*[^{;]+)?\s*(?:\{|;|where)`)

	// Match use statements (imports)
	rustUseRegex = regexp.MustCompile(`(?m)^(?:pub\s+)?use\s+([^;]+);`)

	// Match macro definitions
	rustMacroRegex = regexp.MustCompile(`(?m)^(?:pub\s+)?macro_rules!\s+(\w+)\s*\{`)

	// Match const and static declarations
	rustConstRegex = regexp.MustCompile(`(?m)^(?:pub(?:\s*\([^)]*\))?\s+)?(?:const|static)\s+(\w+)\s*:\s*([^=]+)=`)

	// Match comments (line and block)
	rustCommentRegex = regexp.MustCompile(`(?m)^(?://[^\n]*$|/\*[\s\S]*?\*/)`)
	
	// Match doc comments
	rustDocCommentRegex = regexp.MustCompile(`(?m)^(?:///[^\n]*$|//![^\n]*$|/\*\*[\s\S]*?\*/|/\*![\s\S]*?\*/)`)
)

// RustParser handles parsing Rust code
type RustParser struct{}

// NewRustParser creates a new Rust parser instance
func NewRustParser() *RustParser {
	return &RustParser{}
}

// GetLanguage returns the language this parser supports
func (p *RustParser) GetLanguage() chunking.Language {
	return chunking.LanguageRust
}

// Parse parses Rust code and returns code chunks
func (p *RustParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	// Create a chunk for the entire file
	fileChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageRust,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata:  map[string]interface{}{},
	}
	fileChunk.ID = generateRustChunkID(fileChunk)
	
	// Split code into lines for easier processing
	lines := strings.Split(code, "\n")
	
	// Extract all code chunks
	allChunks := []*chunking.CodeChunk{fileChunk}
	
	// Extract imports (use statements)
	importChunks := p.extractImports(code, lines, fileChunk.ID)
	allChunks = append(allChunks, importChunks...)
	
	// Extract doc comments
	docChunks := p.extractDocComments(code, lines, fileChunk.ID)
	allChunks = append(allChunks, docChunks...)
	
	// Extract regular comments
	commentChunks := p.extractComments(code, lines, fileChunk.ID)
	allChunks = append(allChunks, commentChunks...)
	
	// Extract modules
	moduleChunks := p.extractModules(code, lines, fileChunk.ID)
	allChunks = append(allChunks, moduleChunks...)
	
	// Extract structs
	structChunks := p.extractStructs(code, lines, fileChunk.ID)
	allChunks = append(allChunks, structChunks...)
	
	// Extract enums
	enumChunks := p.extractEnums(code, lines, fileChunk.ID)
	allChunks = append(allChunks, enumChunks...)
	
	// Extract traits
	traitChunks := p.extractTraits(code, lines, fileChunk.ID)
	allChunks = append(allChunks, traitChunks...)
	
	// Extract implementations
	implChunks := p.extractImpls(code, lines, fileChunk.ID)
	allChunks = append(allChunks, implChunks...)
	
	// Extract functions
	functionChunks := p.extractFunctions(code, lines, fileChunk.ID)
	allChunks = append(allChunks, functionChunks...)
	
	// Extract constants and static variables
	constChunks := p.extractConsts(code, lines, fileChunk.ID)
	allChunks = append(allChunks, constChunks...)
	
	// Extract macros
	macroChunks := p.extractMacros(code, lines, fileChunk.ID)
	allChunks = append(allChunks, macroChunks...)
	
	// Process dependencies between chunks
	p.processDependencies(allChunks)
	
	return allChunks, nil
}

// extractDocComments extracts documentation comments from Rust code
func (p *RustParser) extractDocComments(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all doc comments
	docMatches := rustDocCommentRegex.FindAllStringIndex(code, -1)
	
	for i, match := range docMatches {
		if len(match) < 2 {
			continue
		}
		
		// Get the comment content
		commentContent := code[match[0]:match[1]]
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := getLineNumberFromPos(code, match[1]) + 1
		
		// Determine the type of doc comment
		docType := "regular"
		if strings.HasPrefix(commentContent, "//!") || strings.HasPrefix(commentContent, "/*!\n") {
			docType = "module"
		}
		
		// Clean up the comment text
		commentText := commentContent
		if strings.HasPrefix(commentContent, "//") {
			// Line doc comment
			commentText = strings.TrimPrefix(commentText, "///")
			commentText = strings.TrimPrefix(commentText, "//!")
			commentText = strings.TrimSpace(commentText)
		} else {
			// Block doc comment
			commentText = strings.TrimPrefix(commentText, "/**")
			commentText = strings.TrimPrefix(commentText, "/*!") 
			commentText = strings.TrimSuffix(commentText, "*/")
			
			// Clean up lines in block comments
			lines := strings.Split(commentText, "\n")
			for i, line := range lines {
				// Remove leading asterisks and whitespace
				lines[i] = regexp.MustCompile(`^\s*\*?\s*`).ReplaceAllString(line, "")
			}
			commentText = strings.Join(lines, "\n")
			commentText = strings.TrimSpace(commentText)
		}
		
		// Create comment metadata
		commentMetadata := map[string]interface{}{
			"type": "doc",
			"doc_type": docType,
			"text": commentText,
		}
		
		// Create doc comment chunk
		commentChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("doc-%d", i+1),
			Path:      fmt.Sprintf("comment:%d", startLine),
			Content:   commentContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  commentMetadata,
		}
		commentChunk.ID = generateRustChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}
	
	return chunks
}

// extractComments extracts regular comments from Rust code
func (p *RustParser) extractComments(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all comments that are not doc comments
	commentMatches := rustCommentRegex.FindAllStringIndex(code, -1)
	
	for i, match := range commentMatches {
		if len(match) < 2 {
			continue
		}
		
		// Get the comment content
		commentContent := code[match[0]:match[1]]
		
		// Skip doc comments as they're handled separately
		if strings.HasPrefix(commentContent, "//!") || 
		   strings.HasPrefix(commentContent, "///") || 
		   strings.HasPrefix(commentContent, "/**") || 
		   strings.HasPrefix(commentContent, "/*!\n") {
			continue
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := getLineNumberFromPos(code, match[1]) + 1
		
		// Clean up the comment text
		commentText := commentContent
		if strings.HasPrefix(commentContent, "//") {
			// Line comment
			commentText = strings.TrimPrefix(commentText, "//")
			commentText = strings.TrimSpace(commentText)
		} else {
			// Block comment
			commentText = strings.TrimPrefix(commentText, "/*")
			commentText = strings.TrimSuffix(commentText, "*/")
			commentText = strings.TrimSpace(commentText)
		}
		
		// Create comment metadata
		commentMetadata := map[string]interface{}{
			"type": "comment",
			"text": commentText,
		}
		
		// Create comment chunk
		commentChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("comment-%d", i+1),
			Path:      fmt.Sprintf("comment:%d", startLine),
			Content:   commentContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  commentMetadata,
		}
		commentChunk.ID = generateRustChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}
	
	return chunks
}

// extractStructs extracts struct declarations from Rust code
func (p *RustParser) extractStructs(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all struct declarations
	structMatches := rustStructRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range structMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the struct name
		structName := code[match[2]:match[3]]
		
		// Find the struct content
		startPos := match[0]
		
		// Check if this is a struct with a body
		hasBody := false
		structContent := code[startPos:match[1]]
		endPos := match[1]
		
		// Only get full body for structs with definition, not just declarations
		if strings.Contains(structContent, "{") {
			hasBody = true
			// Find the complete struct definition including body
			structContent, endPos = p.findBlockContent(code, startPos)
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Extract generics if present
		generics := ""
		if strings.Contains(structName, "<") {
			genericStart := strings.Index(structName, "<")
			if genericStart != -1 {
				generics = structName[genericStart:]
				structName = structName[:genericStart]
			}
		}
		
		// Check if the struct is public
		isPublic := false
		if strings.HasPrefix(strings.TrimSpace(code[match[0]:]), "pub") {
			isPublic = true
		}
		
		// Create struct metadata
		structMetadata := map[string]interface{}{
			"type":    "struct",
			"public":  isPublic,
		}
		
		if generics != "" {
			structMetadata["generics"] = generics
		}
		
		if hasBody {
			structMetadata["has_body"] = true
		}
		
		// Create struct chunk
		structChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeStruct,
			Name:      structName,
			Path:      fmt.Sprintf("struct:%s", structName),
			Content:   structContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  structMetadata,
		}
		structChunk.ID = generateRustChunkID(structChunk)
		chunks = append(chunks, structChunk)
	}
	
	return chunks
}

// extractEnums extracts enum declarations from Rust code
func (p *RustParser) extractEnums(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all enum declarations
	enumMatches := rustEnumRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range enumMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the enum name
		enumName := code[match[2]:match[3]]
		
		// Find the enum content
		startPos := match[0]
		enumContent, endPos := p.findBlockContent(code, startPos)
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Extract generics if present
		generics := ""
		if strings.Contains(enumName, "<") {
			genericStart := strings.Index(enumName, "<")
			if genericStart != -1 {
				generics = enumName[genericStart:]
				enumName = enumName[:genericStart]
			}
		}
		
		// Check if the enum is public
		isPublic := false
		if strings.HasPrefix(strings.TrimSpace(code[match[0]:]), "pub") {
			isPublic = true
		}
		
		// Try to extract enum variants
		variantRegex := regexp.MustCompile(`(?m)\s*(\w+)(?:\s*=\s*[^,}]+|\s*\([^)]*\)|\s*\{[^}]*\})?`)  
		variantMatches := variantRegex.FindAllStringSubmatch(enumContent, -1)
		
		var variants []string
		if len(variantMatches) > 0 {
			for _, vMatch := range variantMatches {
				if len(vMatch) > 1 && vMatch[1] != enumName && vMatch[1] != "pub" && vMatch[1] != "enum" {
					variants = append(variants, vMatch[1])
				}
			}
		}
		
		// Create enum metadata
		enumMetadata := map[string]interface{}{
			"type":   "enum",
			"public": isPublic,
		}
		
		if generics != "" {
			enumMetadata["generics"] = generics
		}
		
		if len(variants) > 0 {
			enumMetadata["variants"] = variants
		}
		
		// Create enum chunk
		enumChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block since there's no specific Enum type
			Name:      enumName,
			Path:      fmt.Sprintf("enum:%s", enumName),
			Content:   enumContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  enumMetadata,
		}
		enumChunk.ID = generateRustChunkID(enumChunk)
		chunks = append(chunks, enumChunk)
	}
	
	return chunks
}

// extractImports extracts use statements (imports) from Rust code
func (p *RustParser) extractImports(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all import statements
	importMatches := rustUseRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range importMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the import path
		importPath := strings.TrimSpace(code[match[2]:match[3]])
		
		// Get the full import statement
		startPos := match[0]
		endPos := match[1]
		importContent := code[startPos:endPos]
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Try to extract the imported name
		importedName := importPath
		if strings.Contains(importPath, "::{") {
			// Handle glob imports: use std::collections::{HashMap, HashSet}
			importedName = importPath[:strings.Index(importPath, "::{")]
		} else if strings.Contains(importPath, "::*") {
			// Handle star imports: use std::collections::*
			importedName = importPath[:strings.Index(importPath, "::*")]
		} else if strings.Contains(importPath, " as ") {
			// Handle alias imports: use std::collections::HashMap as HMap
			importedName = strings.TrimSpace(strings.Split(importPath, " as ")[1])
		} else if strings.LastIndex(importPath, ":") > 0 {
			// Handle normal imports: use std::collections::HashMap
			importedName = importPath[strings.LastIndex(importPath, ":")+1:]
		}
		
		// Create import metadata
		importMetadata := map[string]interface{}{
			"type": "use",
			"path": importPath,
		}
		
		// Create import chunk
		importChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeImport,
			Name:      importedName,
			Path:      importPath,
			Content:   importContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  importMetadata,
		}
		importChunk.ID = generateRustChunkID(importChunk)
		chunks = append(chunks, importChunk)
	}
	
	return chunks
}

// extractModules extracts module declarations from Rust code
func (p *RustParser) extractModules(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all module declarations
	moduleMatches := rustModuleRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range moduleMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the module name
		modName := code[match[2]:match[3]]
		
		// Find the module content if it's an inline module
		startPos := match[0]
		modContent := code[startPos:match[1]]
		endPos := match[1]
		
		// Check if this is an inline module with a body
		isInline := false
		if strings.Contains(modContent, "{") {
			isInline = true
			// Find the full module body with balanced braces
			modContent, endPos = p.findBlockContent(code, startPos)
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Create module metadata
		modMetadata := map[string]interface{}{
			"type":    "module",
			"inline": isInline,
		}
		
		// Create module chunk
		moduleChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      modName,
			Path:      fmt.Sprintf("mod:%s", modName),
			Content:   modContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  modMetadata,
		}
		moduleChunk.ID = generateRustChunkID(moduleChunk)
		chunks = append(chunks, moduleChunk)
		
		// If it's an inline module, recursively extract its contents
		if isInline {
			// Extract the module's code body (removing the module declaration and braces)
			openingBracePos := strings.Index(modContent, "{")
			if openingBracePos != -1 && len(modContent) > openingBracePos+1 {
				modBody := modContent[openingBracePos+1 : len(modContent)-1] // Remove closing brace
				
				// Extract functions inside this module
				modFuncs := p.extractFunctionsWithOffset(modBody, nil, moduleChunk.ID, true) // Pass nil lines as we're only processing a substring
				chunks = append(chunks, modFuncs...)
			}
		}
	}
	
	return chunks
}

// Helper function to find the content of a block (includes nested blocks)
func (p *RustParser) findBlockContent(code string, startPos int) (string, int) {
	// Find the opening brace
	bracePos := strings.Index(code[startPos:], "{")
	if bracePos == -1 {
		return code[startPos:], len(code)
	}
	
	bracePos += startPos
	
	// Track nested braces
	braceCount := 1
	pos := bracePos + 1
	
	// Find the matching closing brace
	for pos < len(code) && braceCount > 0 {
		if pos+1 < len(code) && code[pos:pos+2] == "//" {
			// Skip line comments
			newlinePos := strings.Index(code[pos:], "\n")
			if newlinePos == -1 {
				break
			}
			pos += newlinePos + 1
		} else if pos+1 < len(code) && code[pos:pos+2] == "/*" {
			// Skip block comments
			commentEndPos := strings.Index(code[pos:], "*/")
			if commentEndPos == -1 {
				break
			}
			pos += commentEndPos + 2
		} else if code[pos] == '{' {
			braceCount++
			pos++
		} else if code[pos] == '}' {
			braceCount--
			pos++
		} else {
			pos++
		}
	}
	
	return code[startPos:pos], pos
}

// generateRustChunkID generates a unique ID for a Rust chunk
func generateRustChunkID(chunk *chunking.CodeChunk) string {
	// Create a hash from the chunk's name, path, and content
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%s", chunk.Name, chunk.Path, chunk.Content)))
	return hex.EncodeToString(h.Sum(nil))
}

// Shell parser for parsing shell scripts into code chunks
package parsers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/S-Corkum/devops-mcp/internal/chunking"
)

// Regex patterns for shell script elements
var (
	// Match function declarations
	shellFunctionRegex = regexp.MustCompile(`(?m)^(?:function\s+)?(\w+)\s*\(\s*\)\s*\{`)

	// Match variable declarations
	shellVarRegex = regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

	// Match sourcing of other files
	shellSourceRegex = regexp.MustCompile(`(?m)^(?:source|\.)\s+([^\s;]+)`)

	// Match if statements
	shellIfRegex = regexp.MustCompile(`(?m)^if\s+(.+);\s*then`)

	// Match loop statements (for, while)
	shellLoopRegex = regexp.MustCompile(`(?m)^(?:for|while)\s+(.+);\s*do`)

	// Match case statements
	shellCaseRegex = regexp.MustCompile(`(?m)^case\s+(.+)\s+in`)

	// Match comments
	shellCommentRegex = regexp.MustCompile(`(?m)^#(.*)$`)

	// Match heredoc
	shellHeredocRegex = regexp.MustCompile(`(?m)<<[-~]?(\w+)`)
)

// ShellParser handles parsing shell scripts
type ShellParser struct{}

// NewShellParser creates a new Shell parser instance
func NewShellParser() *ShellParser {
	return &ShellParser{}
}

// GetLanguage returns the language this parser supports
func (p *ShellParser) GetLanguage() chunking.Language {
	return chunking.LanguageShell
}

// Parse parses shell script code and returns code chunks
func (p *ShellParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	// Create a chunk for the entire file
	fileChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageShell,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata:  map[string]interface{}{},
	}
	fileChunk.ID = generateShellChunkID(fileChunk)
	
	// Split code into lines for easier processing
	lines := strings.Split(code, "\n")
	
	// Extract all code chunks
	allChunks := []*chunking.CodeChunk{fileChunk}
	
	// Extract comments
	commentChunks := p.extractComments(code, lines, fileChunk.ID)
	allChunks = append(allChunks, commentChunks...)
	
	// Extract functions
	functionChunks := p.extractFunctions(code, lines, fileChunk.ID)
	allChunks = append(allChunks, functionChunks...)
	
	// Extract variable declarations
	varChunks := p.extractVariables(code, lines, fileChunk.ID)
	allChunks = append(allChunks, varChunks...)
	
	// Extract source statements
	sourceChunks := p.extractSources(code, lines, fileChunk.ID)
	allChunks = append(allChunks, sourceChunks...)
	
	// Extract control structures (if, loops, case)
	controlChunks := p.extractControlStructures(code, lines, fileChunk.ID)
	allChunks = append(allChunks, controlChunks...)
	
	// Process dependencies between chunks
	p.processDependencies(allChunks)
	
	return allChunks, nil
}

// extractComments extracts comments from shell code
func (p *ShellParser) extractComments(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all comment lines
	commentMatches := shellCommentRegex.FindAllStringSubmatchIndex(code, -1)
	
	for i, match := range commentMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the comment content
		commentContent := code[match[0]:match[1]]
		// Extract the actual comment text (without the # prefix)
		commentText := strings.TrimSpace(code[match[2]:match[3]])
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := getLineNumberFromPos(code, match[1]) + 1
		
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
			Language:  chunking.LanguageShell,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  commentMetadata,
		}
		commentChunk.ID = generateShellChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}
	
	// Also extract heredocs which may contain comments
	heredocMatches := shellHeredocRegex.FindAllStringSubmatchIndex(code, -1)
	
	for i, match := range heredocMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the heredoc delimiter
		delimiter := code[match[2]:match[3]]
		
		// Find the start of the heredoc content
		startPos := match[1]
		
		// Find the end of the heredoc
		endRegex := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(delimiter) + `$`)
		endMatch := endRegex.FindStringIndex(code[startPos:])
		
		if endMatch == nil {
			continue
		}
		
		// Calculate the absolute position of the end
		endPos := startPos + endMatch[1]
		
		// Get the heredoc content
		heredocContent := code[startPos:endPos]
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Create heredoc metadata
		heredocMetadata := map[string]interface{}{
			"type":      "heredoc",
			"delimiter": delimiter,
		}
		
		// Create heredoc chunk
		heredocChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("heredoc-%d", i+1),
			Path:      fmt.Sprintf("heredoc:%d", startLine),
			Content:   heredocContent,
			Language:  chunking.LanguageShell,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  heredocMetadata,
		}
		heredocChunk.ID = generateShellChunkID(heredocChunk)
		chunks = append(chunks, heredocChunk)
	}
	
	return chunks
}

// extractFunctions extracts function declarations from shell code
func (p *ShellParser) extractFunctions(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all function declarations
	functionMatches := shellFunctionRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range functionMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the function name
		functionName := code[match[2]:match[3]]
		
		// Find the function body
		startPos := match[0]
		
		// Find the end of the function (closing brace)
		functionContent, endPos := findShellBlockEnd(code, startPos)
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Create function metadata
		functionMetadata := map[string]interface{}{
			"type": "function",
		}
		
		// Create function chunk
		functionChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeFunction,
			Name:      functionName,
			Path:      functionName,
			Content:   functionContent,
			Language:  chunking.LanguageShell,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  functionMetadata,
		}
		functionChunk.ID = generateShellChunkID(functionChunk)
		chunks = append(chunks, functionChunk)
	}
	
	return chunks
}

// extractVariables extracts variable declarations from shell code
func (p *ShellParser) extractVariables(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all variable declarations
	varMatches := shellVarRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range varMatches {
		if len(match) < 6 {
			continue
		}
		
		// Get the variable name and value
		varName := code[match[2]:match[3]]
		varValue := code[match[4]:match[5]]
		
		// Get the full variable declaration
		varContent := code[match[0]:match[1]]
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := startLine // Variables are typically one line
		
		// Create variable metadata
		varMetadata := map[string]interface{}{
			"type":  "variable",
			"value": strings.TrimSpace(varValue),
		}
		
		// Create variable chunk
		varChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      varName,
			Path:      fmt.Sprintf("var:%s", varName),
			Content:   varContent,
			Language:  chunking.LanguageShell,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  varMetadata,
		}
		varChunk.ID = generateShellChunkID(varChunk)
		chunks = append(chunks, varChunk)
	}
	
	return chunks
}

// extractSources extracts source statements from shell code
func (p *ShellParser) extractSources(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all source statements
	sourceMatches := shellSourceRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range sourceMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the source path
		sourcePath := code[match[2]:match[3]]
		
		// Get the full source statement
		sourceContent := code[match[0]:match[1]]
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := startLine // Source statements are typically one line
		
		// Create source metadata
		sourceMetadata := map[string]interface{}{
			"type": "source",
			"path": sourcePath,
		}
		
		// Create source chunk
		sourceChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeImport,
			Name:      fmt.Sprintf("source:%s", sourcePath),
			Path:      sourcePath,
			Content:   sourceContent,
			Language:  chunking.LanguageShell,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  sourceMetadata,
		}
		sourceChunk.ID = generateShellChunkID(sourceChunk)
		chunks = append(chunks, sourceChunk)
	}
	
	return chunks
}

// extractControlStructures extracts control structures from shell code
func (p *ShellParser) extractControlStructures(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Extract if statements
	chunks = append(chunks, p.extractControlStructure(code, lines, parentID, shellIfRegex, "if", "fi")...)
	
	// Extract loop statements
	chunks = append(chunks, p.extractControlStructure(code, lines, parentID, shellLoopRegex, "loop", "done")...)
	
	// Extract case statements
	chunks = append(chunks, p.extractControlStructure(code, lines, parentID, shellCaseRegex, "case", "esac")...)
	
	return chunks
}

// extractControlStructure extracts a specific type of control structure from shell code
func (p *ShellParser) extractControlStructure(code string, lines []string, parentID string, regex *regexp.Regexp, structType string, endKeyword string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all control structures of this type
	matches := regex.FindAllStringSubmatchIndex(code, -1)
	
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}
		
		// Get the condition
		condition := code[match[2]:match[3]]
		
		// Find the start of the control structure
		startPos := match[0]
		
		// Find the end of the control structure
		endRegex := regexp.MustCompile(`(?m)^(?:\s*|\t*)` + endKeyword + `\b`)
		endMatch := endRegex.FindStringIndex(code[startPos:])
		
		if endMatch == nil {
			continue
		}
		
		// Calculate the absolute position of the end
		endPos := startPos + endMatch[1]
		
		// Get the control structure content
		controlContent := code[startPos:endPos]
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Create control structure metadata
		controlMetadata := map[string]interface{}{
			"type":      "control_structure",
			"subtype":   structType,
			"condition": strings.TrimSpace(condition),
		}
		
		// Create control structure chunk
		controlChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("%s-%d", structType, i+1),
			Path:      fmt.Sprintf("%s:%d", structType, startLine),
			Content:   controlContent,
			Language:  chunking.LanguageShell,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  controlMetadata,
		}
		controlChunk.ID = generateShellChunkID(controlChunk)
		chunks = append(chunks, controlChunk)
	}
	
	return chunks
}

// processDependencies analyzes chunks to identify dependencies
func (p *ShellParser) processDependencies(chunks []*chunking.CodeChunk) {
	// Map chunks by ID for quick lookup
	chunkMap := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}
	
	// Map function chunks by name for dependency tracking
	functionsByName := make(map[string]*chunking.CodeChunk)
	variablesByName := make(map[string]*chunking.CodeChunk)
	
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeFunction {
			functionsByName[chunk.Name] = chunk
		} else if chunk.Metadata != nil && chunk.Metadata["type"] == "variable" {
			variablesByName[chunk.Name] = chunk
		}
	}
	
	// Analyze dependencies
	for _, chunk := range chunks {
		// Skip the file chunk
		if chunk.Type == chunking.ChunkTypeFile {
			continue
		}
		
		// Add parent dependency if it exists
		if chunk.ParentID != "" && chunk.ParentID != chunk.ID {
			if chunk.Dependencies == nil {
				chunk.Dependencies = []string{}
			}
			chunk.Dependencies = append(chunk.Dependencies, chunk.ParentID)
		}
		
		// Check for function calls in this chunk
		for funcName, funcChunk := range functionsByName {
			// Skip self-reference
			if chunk.ID == funcChunk.ID {
				continue
			}
			
			// Look for function call pattern
			callPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(funcName) + `\b\s*\(?\s*\)?`)
			if callPattern.MatchString(chunk.Content) {
				if chunk.Dependencies == nil {
					chunk.Dependencies = []string{}
				}
				chunk.Dependencies = append(chunk.Dependencies, funcChunk.ID)
			}
		}
		
		// Check for variable references
		for varName, varChunk := range variablesByName {
			// Skip self-reference
			if chunk.ID == varChunk.ID {
				continue
			}
			
			// Look for variable reference pattern (both $VAR and ${VAR} forms)
			refPattern := regexp.MustCompile(`\$(` + regexp.QuoteMeta(varName) + `\b|\{` + regexp.QuoteMeta(varName) + `\})`)
			if refPattern.MatchString(chunk.Content) {
				if chunk.Dependencies == nil {
					chunk.Dependencies = []string{}
				}
				chunk.Dependencies = append(chunk.Dependencies, varChunk.ID)
			}
		}
	}
}

// Helper function to find the end of a shell block
func findShellBlockEnd(code string, startPos int) (string, int) {
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
		if code[pos] == '{' {
			braceCount++
		} else if code[pos] == '}' {
			braceCount--
		}
		pos++
	}
	
	if braceCount > 0 {
		// Couldn't find matching closing brace, return the rest of the code
		return code[startPos:], len(code)
	}
	
	return code[startPos:pos], pos
}

// generateShellChunkID generates a unique ID for a shell chunk
func generateShellChunkID(chunk *chunking.CodeChunk) string {
	// Create a hash from the chunk's name, path, and content
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%s", chunk.Name, chunk.Path, chunk.Content)))
	return hex.EncodeToString(h.Sum(nil))
}

// Kotlin parser for parsing Kotlin code into code chunks
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

// Regex patterns for Kotlin code elements
var (
	// Match package declarations
	kotlinPackageRegex = regexp.MustCompile(`(?m)^package\s+([\w.]+)`)

	// Match import statements
	kotlinImportRegex = regexp.MustCompile(`(?m)^import\s+([\w.]+(?:\.\*)?|\w+\s+as\s+\w+)`)

	// Match class declarations (including data, sealed, abstract, etc.)
	kotlinClassRegex = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+)*(?:data\s+|sealed\s+|abstract\s+|open\s+|enum\s+|annotation\s+)*class\s+(\w+)(?:<[^>]*>)?(?:\s*:\s*[^{]*)?(\s*\{|\s*$)`)

	// Match interface declarations
	kotlinInterfaceRegex = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+)*(?:sealed\s+)?interface\s+(\w+)(?:<[^>]*>)?(?:\s*:\s*[^{]*)?(\s*\{|\s*$)`)

	// Match object declarations (including companion object)
	kotlinObjectRegex = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+)*(?:companion\s+)?object(?:\s+(\w+))?(?:\s*:\s*[^{]*)?(?:\s*\{|\s*$)`)

	// Match function declarations
	kotlinFunctionRegex = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+|inline\s+|suspend\s+|tailrec\s+|external\s+|override\s+|open\s+|abstract\s+)*fun\s+(?:<[^>]*>\s+)?(\w+)\s*\(([^)]*)\)(?:\s*:\s*[^{=]*)?(?:\s*=|\s*\{|\s*$)`)

	// Match property declarations
	kotlinPropertyRegex = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+|const\s+|val\s+|var\s+|lateinit\s+)*(?:val|var)\s+(\w+)(?:\s*:\s*[^=]*)?(?:\s*=\s*[^{]*)?\s*(?:\{|\S|$)`)

	// Match extension functions
	kotlinExtensionRegex = regexp.MustCompile(`(?m)^(?:public\s+|private\s+|protected\s+|internal\s+|inline\s+|suspend\s+|tailrec\s+)*fun\s+([^(]+?)\.(\w+)\s*\(([^)]*)\)(?:\s*:\s*[^{=]*)?(?:\s*=|\s*\{|\s*$)`)

	// Match single-line comments
	kotlinLineCommentRegex = regexp.MustCompile(`(?m)//.*`)

	// Match block comments and KDoc comments
	kotlinBlockCommentRegex = regexp.MustCompile(`(?sm)/\*.*?\*/`)

	// Match KDoc comments specifically
	kotlinKDocRegex = regexp.MustCompile(`(?sm)/\*\*.*?\*/`)
)

// KotlinParser handles parsing Kotlin code
type KotlinParser struct{}

// NewKotlinParser creates a new Kotlin parser instance
func NewKotlinParser() *KotlinParser {
	return &KotlinParser{}
}

// GetLanguage returns the language this parser supports
func (p *KotlinParser) GetLanguage() chunking.Language {
	return chunking.LanguageKotlin
}

// Parse parses Kotlin code and returns code chunks
func (p *KotlinParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	// Create a chunk for the entire file
	fileChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageKotlin,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata:  map[string]interface{}{
			"chunking_method": "kotlin",
		},
	}
	fileChunk.ID = generateKotlinChunkID(fileChunk)
	
	// Split code into lines for easier processing
	lines := strings.Split(code, "\n")
	
	// Extract all code chunks
	allChunks := []*chunking.CodeChunk{fileChunk}
	
	// Extract package declarations
	packageChunks := p.extractPackages(code, lines, fileChunk.ID)
	allChunks = append(allChunks, packageChunks...)
	
	// Extract imports
	importChunks := p.extractImports(code, lines, fileChunk.ID)
	allChunks = append(allChunks, importChunks...)
	
	// Extract KDoc comments
	kdocChunks := p.extractKDocs(code, lines, fileChunk.ID)
	allChunks = append(allChunks, kdocChunks...)
	
	// Extract regular comments
	commentChunks := p.extractComments(code, lines, fileChunk.ID)
	allChunks = append(allChunks, commentChunks...)
	
	// Extract classes and interfaces
	classChunks := p.extractClasses(code, lines, fileChunk.ID)
	allChunks = append(allChunks, classChunks...)
	
	interfaceChunks := p.extractInterfaces(code, lines, fileChunk.ID)
	allChunks = append(allChunks, interfaceChunks...)
	
	// Extract objects
	objectChunks := p.extractObjects(code, lines, fileChunk.ID)
	allChunks = append(allChunks, objectChunks...)
	
	// Extract top-level functions
	functionChunks := p.extractFunctions(code, lines, fileChunk.ID)
	allChunks = append(allChunks, functionChunks...)
	
	// Extract top-level properties
	propertyChunks := p.extractProperties(code, lines, fileChunk.ID)
	allChunks = append(allChunks, propertyChunks...)
	
	// Extract extension functions
	extensionChunks := p.extractExtensions(code, lines, fileChunk.ID)
	allChunks = append(allChunks, extensionChunks...)
	
	// Process dependencies between chunks
	p.processDependencies(allChunks)
	
	return allChunks, nil
}

// Helper function to find the content of a block (includes nested blocks)
func (p *KotlinParser) findBlockContent(code string, startPos int) (string, int) {
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

// generateKotlinChunkID generates a unique ID for a Kotlin chunk
func generateKotlinChunkID(chunk *chunking.CodeChunk) string {
	// Create a hash from the chunk's name, path, and content
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%s", chunk.Name, chunk.Path, chunk.Content)))
	return hex.EncodeToString(h.Sum(nil))
}

// Note: getLineNumberFromPos and countLines functions are used from utils.go

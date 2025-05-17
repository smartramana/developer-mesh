package parsers

import (
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractMacros extracts macro declarations from Rust code
func (p *RustParser) extractMacros(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all macro declarations
	macroMatches := rustMacroRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range macroMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the macro name
		macroName := code[match[2]:match[3]]
		
		// Find the macro content (including body)
		startPos := match[0]
		macroContent, endPos := p.findBlockContent(code, startPos)
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Check if the macro is public
		isPublic := false
		if strings.HasPrefix(strings.TrimSpace(code[match[0]:]), "pub") {
			isPublic = true
		}
		
		// Create macro metadata
		macroMetadata := map[string]interface{}{
			"type":   "macro",
			"public": isPublic,
		}
		
		// Create macro chunk
		macroChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block as there's no specific macro type
			Name:      macroName,
			Path:      fmt.Sprintf("macro:%s", macroName),
			Content:   macroContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  macroMetadata,
		}
		macroChunk.ID = generateRustChunkID(macroChunk)
		chunks = append(chunks, macroChunk)
	}
	
	// TODO: Handle procedural macros if needed
	
	return chunks
}

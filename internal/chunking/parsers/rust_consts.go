package parsers

import (
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/internal/chunking"
)

// extractConsts extracts constant and static variable declarations from Rust code
func (p *RustParser) extractConsts(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all constant and static declarations
	constMatches := rustConstRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range constMatches {
		if len(match) < 6 {
			continue
		}
		
		// Get the constant name and type
		constName := code[match[2]:match[3]]
		constType := code[match[4]:match[5]]
		
		// Find the start of the declaration
		startPos := match[0]
		
		// Find the end of the declaration (at the semicolon)
		endPos := strings.Index(code[startPos:], ";")
		if endPos == -1 {
			// If no semicolon found, this shouldn't happen with valid Rust code
			continue
		}
		endPos += startPos
		
		// Get the full constant declaration
		constContent := code[startPos:endPos+1] // Include the semicolon
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Check if the declaration is a const or static
		isConst := strings.Contains(code[startPos:startPos+10], "const")
		
		// Check if the const/static is public
		isPublic := false
		if strings.HasPrefix(strings.TrimSpace(code[match[0]:]), "pub") {
			isPublic = true
		}
		
		// Try to extract the value
		value := ""
		equalPos := strings.Index(constContent, "=")
		if equalPos != -1 && equalPos+1 < len(constContent) {
			value = strings.TrimSpace(constContent[equalPos+1:len(constContent)-1]) // Remove semicolon
		}
		
		// Create const metadata
		constMetadata := map[string]interface{}{
			"type":      "const",
			"is_const":  isConst,
			"is_static": !isConst,
			"data_type": strings.TrimSpace(constType),
			"public":    isPublic,
		}
		
		if value != "" {
			constMetadata["value"] = value
		}
		
		// Create const chunk
		constChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock, // Using Block type for consts as there's no specific variable type
			Name:      constName,
			Path:      fmt.Sprintf("%s:%s", func() string {
				if isConst {
					return "const"
				} 
				return "static"
			}(), constName),
			Content:   constContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  constMetadata,
		}
		constChunk.ID = generateRustChunkID(constChunk)
		chunks = append(chunks, constChunk)
	}
	
	return chunks
}

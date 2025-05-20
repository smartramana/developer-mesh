package parsers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractImpls extracts implementation blocks from Rust code
func (p *RustParser) extractImpls(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// We need to match two kinds of impl blocks:
	// 1. Regular impl blocks: impl Foo { ... }
	// 2. Trait impl blocks: impl Trait for Foo { ... }
	
	// First, process regular impl blocks
	implMatches := rustImplRegex2.FindAllStringIndex(code, -1)
	
	for i, match := range implMatches {
		if len(match) < 2 {
			continue
		}
		
		// Find the impl content (including body)
		startPos := match[0]
		implContent, endPos := p.findBlockContent(code, startPos)
		
		// Get the impl declaration
		implDecl := implContent[:strings.Index(implContent, "{")]
		
		// Extract the type name - this is complex due to potential generics
		// Simplest approach is to look for the word after "impl"
		typeName := ""
		typeNameRegex := regexp.MustCompile(`impl(?:\s*<[^>]*>)?\s+([^\s<{]+)`)
		typeNameMatch := typeNameRegex.FindStringSubmatch(implDecl)
		if len(typeNameMatch) > 1 {
			typeName = typeNameMatch[1]
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Extract generics if present
		generics := ""
		if strings.Contains(implDecl, "<") && strings.Contains(implDecl, ">") {
			genericStart := strings.Index(implDecl, "<")
			genericEnd := strings.LastIndex(implDecl, ">")
			if genericStart != -1 && genericEnd != -1 && genericEnd > genericStart {
				generics = implDecl[genericStart:genericEnd+1]
			}
		}
		
		// Check for where clause
		whereClause := ""
		if strings.Contains(implDecl, " where ") {
			whereParts := strings.SplitN(implDecl, " where ", 2)
			if len(whereParts) > 1 {
				whereClause = "where " + strings.TrimSpace(whereParts[1])
			}
		}
		
		// Create impl metadata
		implMetadata := map[string]interface{}{
			"type": "implementation",
			"impl_type": "regular",
			"for_type": typeName,
		}
		
		if generics != "" {
			implMetadata["generics"] = generics
		}
		
		if whereClause != "" {
			implMetadata["where_clause"] = whereClause
		}
		
		// Create impl chunk
		implChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("impl-%d", i+1),
			Path:      fmt.Sprintf("impl:%s", typeName),
			Content:   implContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  implMetadata,
		}
		implChunk.ID = generateRustChunkID(implChunk)
		chunks = append(chunks, implChunk)
	}
	
	// Next, process trait implementation blocks
	traitImplMatches := rustImplRegex.FindAllStringIndex(code, -1)
	
	for i, match := range traitImplMatches {
		if len(match) < 2 {
			continue
		}
		
		// Find the impl content (including body)
		startPos := match[0]
		implContent, endPos := p.findBlockContent(code, startPos)
		
		// Get the impl declaration
		implDecl := implContent[:strings.Index(implContent, "{")]
		
		// Extract trait name and type name
		// This is complex due to potential generics and paths
		traitName := ""
		typeName := ""
		
		if strings.Contains(implDecl, " for ") {
			parts := strings.SplitN(implDecl, " for ", 2)
			if len(parts) > 1 {
				// Extract trait name from first part (after "impl")
				traitPart := parts[0]
				traitPart = strings.TrimPrefix(traitPart, "impl")
				traitPart = strings.TrimSpace(traitPart)
				
				// Handle generics in trait name
				if strings.Contains(traitPart, "<") {
					traitName = traitPart[:strings.Index(traitPart, "<")]
				} else {
					traitName = traitPart
				}
				
				// Extract type name from second part (before "{" or "where")
				typePart := parts[1]
				if strings.Contains(typePart, "where") {
					typePart = strings.SplitN(typePart, "where", 2)[0]
				}
				typePart = strings.TrimSpace(typePart)
				
				// Handle generics in type name
				if strings.Contains(typePart, "<") {
					typeName = typePart[:strings.Index(typePart, "<")]
				} else {
					typeName = typePart
				}
			}
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Extract generics if present
		generics := ""
		if strings.Contains(implDecl, "<") && strings.Contains(implDecl, ">") {
			genericStart := strings.Index(implDecl, "<")
			genericEnd := strings.LastIndex(implDecl, ">")
			if genericStart != -1 && genericEnd != -1 && genericEnd > genericStart {
				generics = implDecl[genericStart:genericEnd+1]
			}
		}
		
		// Check for where clause
		whereClause := ""
		if strings.Contains(implDecl, " where ") {
			whereParts := strings.SplitN(implDecl, " where ", 2)
			if len(whereParts) > 1 {
				whereClause = "where " + strings.TrimSpace(whereParts[1])
			}
		}
		
		// Create impl metadata
		implMetadata := map[string]interface{}{
			"type": "implementation",
			"impl_type": "trait",
			"trait": traitName,
			"for_type": typeName,
		}
		
		if generics != "" {
			implMetadata["generics"] = generics
		}
		
		if whereClause != "" {
			implMetadata["where_clause"] = whereClause
		}
		
		// Create impl chunk
		implChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("impl-trait-%d", i+1),
			Path:      fmt.Sprintf("impl:%s-for-%s", traitName, typeName),
			Content:   implContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  implMetadata,
		}
		implChunk.ID = generateRustChunkID(implChunk)
		chunks = append(chunks, implChunk)
		
		// Extract functions within the implementation block
		openBracePos := strings.Index(implContent, "{")
		if openBracePos != -1 && len(implContent) > openBracePos+1 {
			// Get the content within the braces
			implBody := implContent[openBracePos+1 : len(implContent)-1]
			
			// Extract functions
			funcChunks := p.extractFunctionsWithOffset(implBody, nil, implChunk.ID, false)
			chunks = append(chunks, funcChunks...)
		}
	}
	
	return chunks
}

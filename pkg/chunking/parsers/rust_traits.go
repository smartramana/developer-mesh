package parsers

import (
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractTraits extracts trait declarations from Rust code
func (p *RustParser) extractTraits(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all trait declarations
	traitMatches := rustTraitRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range traitMatches {
		if len(match) < 4 {
			continue
		}

		// Get the trait name
		traitName := code[match[2]:match[3]]

		// Find the trait content (including body)
		startPos := match[0]
		traitContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1

		// Extract generics if present
		generics := ""
		if strings.Contains(traitName, "<") {
			genericStart := strings.Index(traitName, "<")
			if genericStart != -1 {
				generics = traitName[genericStart:]
				traitName = traitName[:genericStart]
			}
		}

		// Check if the trait is public
		isPublic := false
		if strings.HasPrefix(strings.TrimSpace(code[match[0]:]), "pub") {
			isPublic = true
		}

		// Extract trait super-traits (traits this trait extends)
		superTraits := []string{}
		traitDeclLine := code[match[0] : match[0]+strings.Index(code[match[0]:], "{")]
		if strings.Contains(traitDeclLine, ":") {
			superTraitPart := strings.Split(traitDeclLine, ":")[1]
			superTraitPart = strings.TrimSpace(superTraitPart)
			// Split by + to get individual traits
			for _, st := range strings.Split(superTraitPart, "+") {
				st = strings.TrimSpace(st)
				if st != "" && st != "{" {
					// Remove any generic parameters from super trait
					if strings.Contains(st, "<") {
						st = st[:strings.Index(st, "<")]
					}
					superTraits = append(superTraits, st)
				}
			}
		}

		// Find all associated functions in the trait
		associatedFuncs := []string{}
		// Extract function signatures from trait body
		fnMatches := rustFunctionRegex.FindAllStringSubmatch(traitContent, -1)
		for _, fnMatch := range fnMatches {
			if len(fnMatch) > 1 {
				associatedFuncs = append(associatedFuncs, fnMatch[1])
			}
		}

		// Create trait metadata
		traitMetadata := map[string]interface{}{
			"type":   "trait",
			"public": isPublic,
		}

		if generics != "" {
			traitMetadata["generics"] = generics
		}

		if len(superTraits) > 0 {
			traitMetadata["super_traits"] = superTraits
		}

		if len(associatedFuncs) > 0 {
			traitMetadata["associated_functions"] = associatedFuncs
		}

		// Create trait chunk
		traitChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeInterface, // Using Interface type as it's the closest match for a trait
			Name:      traitName,
			Path:      fmt.Sprintf("trait:%s", traitName),
			Content:   traitContent,
			Language:  chunking.LanguageRust,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  traitMetadata,
		}
		traitChunk.ID = generateRustChunkID(traitChunk)
		chunks = append(chunks, traitChunk)
	}

	return chunks
}

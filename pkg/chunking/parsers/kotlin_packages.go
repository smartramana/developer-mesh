package parsers

import (
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractPackages extracts package declarations from Kotlin code
func (p *KotlinParser) extractPackages(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all package declarations (should be only one in most cases)
	packageMatches := kotlinPackageRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range packageMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the package name
		packageName := code[match[2]:match[3]]
		
		// Find the line numbers
		startPos := match[0]
		endPos := match[1]
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Get the full package declaration
		packageContent := code[startPos:endPos]
		
		// Create package metadata
		packageMetadata := map[string]interface{}{
			"type": "package",
			"name": packageName,
		}
		
		// Create package chunk
		packageChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      packageName,
			Path:      fmt.Sprintf("package:%s", packageName),
			Content:   packageContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  packageMetadata,
		}
		packageChunk.ID = generateKotlinChunkID(packageChunk)
		chunks = append(chunks, packageChunk)
	}
	
	return chunks
}

// extractImports extracts import statements from Kotlin code
func (p *KotlinParser) extractImports(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all import statements
	importMatches := kotlinImportRegex.FindAllStringSubmatchIndex(code, -1)
	
	for _, match := range importMatches {
		if len(match) < 4 {
			continue
		}
		
		// Get the import path
		importPath := code[match[2]:match[3]]
		
		// Find the line numbers
		startPos := match[0]
		endPos := match[1]
		startLine := getLineNumberFromPos(code, startPos) + 1
		endLine := getLineNumberFromPos(code, endPos) + 1
		
		// Get the full import statement
		importContent := code[startPos:endPos]
		
		// Determine if this is a wildcard import
		isWildcard := strings.HasSuffix(importPath, ".*")
		
		// Determine if this is an alias import (e.g. "import org.example.Foo as Bar")
		isAlias := strings.Contains(importPath, " as ")
		
		// Extract just the name part for the chunk name
		var importName string
		if isAlias {
			// For alias imports, use the alias as the name
			parts := strings.Split(importPath, " as ")
			if len(parts) > 1 {
				importName = strings.TrimSpace(parts[1])
			} else {
				importName = importPath
			}
		} else if isWildcard {
			// For wildcard imports, use the package name
			importName = importPath[:len(importPath)-2] // remove ".*"
			importName = importName[strings.LastIndex(importName, ".")+1:]
		} else {
			// For regular imports, use the class name
			importName = importPath[strings.LastIndex(importPath, ".")+1:]
		}
		
		// Create import metadata
		importMetadata := map[string]interface{}{
			"type":       "import",
			"path":       importPath,
			"is_wildcard": isWildcard,
			"is_alias":   isAlias,
		}
		
		// Create import chunk
		importChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeImport,
			Name:      importName,
			Path:      importPath,
			Content:   importContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  importMetadata,
		}
		importChunk.ID = generateKotlinChunkID(importChunk)
		chunks = append(chunks, importChunk)
	}
	
	return chunks
}

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

// HCL regular expressions for extracting Terraform code structure
var (
	// Match resource declarations
	hclResourceRegex = regexp.MustCompile(`(?m)^resource\s+"([^"]+)"\s+"([^"]+)"\s*{`)

	// Match data source declarations
	hclDataRegex = regexp.MustCompile(`(?m)^data\s+"([^"]+)"\s+"([^"]+)"\s*{`)

	// Match provider declarations
	hclProviderRegex = regexp.MustCompile(`(?m)^provider\s+"([^"]+)"\s*{`)

	// Match module declarations
	hclModuleRegex = regexp.MustCompile(`(?m)^module\s+"([^"]+)"\s*{`)

	// Match variable declarations
	hclVariableRegex = regexp.MustCompile(`(?m)^variable\s+"([^"]+)"\s*{`)

	// Match output declarations
	hclOutputRegex = regexp.MustCompile(`(?m)^output\s+"([^"]+)"\s*{`)

	// Match locals declarations
	hclLocalsRegex = regexp.MustCompile(`(?m)^locals\s*{`)

	// Match terraform block declarations
	hclTerraformRegex = regexp.MustCompile(`(?m)^terraform\s*{`)

	// Match block comments
	hclCommentRegex = regexp.MustCompile(`(?ms)/\*.*?\*/`)

	// Match line comments
	// TODO: Implement line comment extraction to capture inline documentation
	// hclLineCommentRegex = regexp.MustCompile(`(?m)^([^#\n]*?)#([^\n]*)`)
)

// HCLParser is a parser for HCL (Terraform) code
type HCLParser struct{}

// NewHCLParser creates a new HCLParser
func NewHCLParser() *HCLParser {
	return &HCLParser{}
}

// GetLanguage returns the language this parser handles
func (p *HCLParser) GetLanguage() chunking.Language {
	return chunking.LanguageHCL
}

// Parse parses HCL (Terraform) code and returns chunks
func (p *HCLParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	chunks := []*chunking.CodeChunk{}

	// Create a chunk for the entire file
	fileChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageHCL,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata:  map[string]interface{}{},
	}
	fileChunk.ID = generateHCLChunkID(fileChunk)
	chunks = append(chunks, fileChunk)

	// Extract comments
	commentChunks := p.extractComments(code, fileChunk.ID)
	chunks = append(chunks, commentChunks...)

	// Extract resources
	resourceChunks := p.extractResources(code, fileChunk.ID)
	chunks = append(chunks, resourceChunks...)

	// Extract data sources
	dataChunks := p.extractDataSources(code, fileChunk.ID)
	chunks = append(chunks, dataChunks...)

	// Extract providers
	providerChunks := p.extractProviders(code, fileChunk.ID)
	chunks = append(chunks, providerChunks...)

	// Extract modules
	moduleChunks := p.extractModules(code, fileChunk.ID)
	chunks = append(chunks, moduleChunks...)

	// Extract variables
	variableChunks := p.extractVariables(code, fileChunk.ID)
	chunks = append(chunks, variableChunks...)

	// Extract outputs
	outputChunks := p.extractOutputs(code, fileChunk.ID)
	chunks = append(chunks, outputChunks...)

	// Extract locals
	localsChunks := p.extractLocals(code, fileChunk.ID)
	chunks = append(chunks, localsChunks...)

	// Extract terraform blocks
	terraformChunks := p.extractTerraformBlocks(code, fileChunk.ID)
	chunks = append(chunks, terraformChunks...)

	// Process dependencies
	p.processDependencies(chunks)

	return chunks, nil
}

// extractComments extracts comments from HCL (Terraform) code
func (p *HCLParser) extractComments(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all block comments
	blockCommentMatches := hclCommentRegex.FindAllStringIndex(code, -1)

	for i, match := range blockCommentMatches {
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
			Name:      fmt.Sprintf("BlockComment_%d", i+1),
			Path:      fmt.Sprintf("comment:%d", startLine),
			Content:   commentText,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
		}
		commentChunk.ID = generateHCLChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}

	// We don't extract line comments as separate chunks since they're often inline with code
	// and would create too much noise. They'll be part of their parent blocks.

	return chunks
}

// findBlockContent finds the content of a block (including nested blocks)
func (p *HCLParser) findBlockContent(code string, startPos int) (string, int) {
	// Find the opening brace
	openBracePos := startPos
	for openBracePos < len(code) && code[openBracePos] != '{' {
		openBracePos++
	}

	if openBracePos >= len(code) {
		return "", startPos
	}

	// Count braces to find the matching closing brace
	braceCount := 1
	endPos := openBracePos + 1

	for endPos < len(code) && braceCount > 0 {
		switch code[endPos] {
		case '{':
			braceCount++
		case '}':
			braceCount--
		}
		endPos++
	}

	return code[startPos:endPos], endPos
}

// extractResources extracts resource blocks from HCL (Terraform) code
func (p *HCLParser) extractResources(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all resource declarations
	resourceMatches := hclResourceRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range resourceMatches {
		if len(match) < 6 {
			continue
		}

		// Get the resource type and name
		resourceType := code[match[2]:match[3]]
		resourceName := code[match[4]:match[5]]

		// Find the resource block content
		startPos := match[0]
		resourceContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create resource metadata
		resourceMetadata := map[string]interface{}{
			"resource_type": resourceType,
			"resource_name": resourceName,
		}

		// Create resource chunk
		resourceChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("resource.%s.%s", resourceType, resourceName),
			Path:      fmt.Sprintf("resource.%s.%s", resourceType, resourceName),
			Content:   resourceContent,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  resourceMetadata,
		}
		resourceChunk.ID = generateHCLChunkID(resourceChunk)
		chunks = append(chunks, resourceChunk)
	}

	return chunks
}

// extractDataSources extracts data source blocks from HCL (Terraform) code
func (p *HCLParser) extractDataSources(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all data source declarations
	dataMatches := hclDataRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range dataMatches {
		if len(match) < 6 {
			continue
		}

		// Get the data source type and name
		dataType := code[match[2]:match[3]]
		dataName := code[match[4]:match[5]]

		// Find the data source block content
		startPos := match[0]
		dataContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create data source metadata
		dataMetadata := map[string]interface{}{
			"data_type": dataType,
			"data_name": dataName,
		}

		// Create data source chunk
		dataChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("data.%s.%s", dataType, dataName),
			Path:      fmt.Sprintf("data.%s.%s", dataType, dataName),
			Content:   dataContent,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  dataMetadata,
		}
		dataChunk.ID = generateHCLChunkID(dataChunk)
		chunks = append(chunks, dataChunk)
	}

	return chunks
}

// extractProviders extracts provider blocks from HCL (Terraform) code
func (p *HCLParser) extractProviders(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all provider declarations
	providerMatches := hclProviderRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range providerMatches {
		if len(match) < 4 {
			continue
		}

		// Get the provider name
		providerName := code[match[2]:match[3]]

		// Find the provider block content
		startPos := match[0]
		providerContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create provider metadata
		providerMetadata := map[string]interface{}{
			"provider_name": providerName,
		}

		// Create provider chunk
		providerChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("provider.%s", providerName),
			Path:      fmt.Sprintf("provider.%s", providerName),
			Content:   providerContent,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  providerMetadata,
		}
		providerChunk.ID = generateHCLChunkID(providerChunk)
		chunks = append(chunks, providerChunk)
	}

	return chunks
}

// extractModules extracts module blocks from HCL (Terraform) code
func (p *HCLParser) extractModules(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all module declarations
	moduleMatches := hclModuleRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range moduleMatches {
		if len(match) < 4 {
			continue
		}

		// Get the module name
		moduleName := code[match[2]:match[3]]

		// Find the module block content
		startPos := match[0]
		moduleContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create module metadata
		moduleMetadata := map[string]interface{}{
			"module_name": moduleName,
		}

		// Create module chunk
		moduleChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("module.%s", moduleName),
			Path:      fmt.Sprintf("module.%s", moduleName),
			Content:   moduleContent,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  moduleMetadata,
		}
		moduleChunk.ID = generateHCLChunkID(moduleChunk)
		chunks = append(chunks, moduleChunk)
	}

	return chunks
}

// extractVariables extracts variable blocks from HCL (Terraform) code
func (p *HCLParser) extractVariables(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all variable declarations
	variableMatches := hclVariableRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range variableMatches {
		if len(match) < 4 {
			continue
		}

		// Get the variable name
		variableName := code[match[2]:match[3]]

		// Find the variable block content
		startPos := match[0]
		variableContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create variable metadata
		variableMetadata := map[string]interface{}{
			"variable_name": variableName,
		}

		// Create variable chunk
		variableChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("var.%s", variableName),
			Path:      fmt.Sprintf("var.%s", variableName),
			Content:   variableContent,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  variableMetadata,
		}
		variableChunk.ID = generateHCLChunkID(variableChunk)
		chunks = append(chunks, variableChunk)
	}

	return chunks
}

// extractOutputs extracts output blocks from HCL (Terraform) code
func (p *HCLParser) extractOutputs(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all output declarations
	outputMatches := hclOutputRegex.FindAllStringSubmatchIndex(code, -1)

	for _, match := range outputMatches {
		if len(match) < 4 {
			continue
		}

		// Get the output name
		outputName := code[match[2]:match[3]]

		// Find the output block content
		startPos := match[0]
		outputContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create output metadata
		outputMetadata := map[string]interface{}{
			"output_name": outputName,
		}

		// Create output chunk
		outputChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("output.%s", outputName),
			Path:      fmt.Sprintf("output.%s", outputName),
			Content:   outputContent,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  outputMetadata,
		}
		outputChunk.ID = generateHCLChunkID(outputChunk)
		chunks = append(chunks, outputChunk)
	}

	return chunks
}

// extractLocals extracts locals blocks from HCL (Terraform) code
func (p *HCLParser) extractLocals(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all locals declarations
	localsMatches := hclLocalsRegex.FindAllStringIndex(code, -1)

	for i, match := range localsMatches {
		if len(match) < 2 {
			continue
		}

		// Find the locals block content
		startPos := match[0]
		localsContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create locals chunk
		localsChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("locals_%d", i+1),
			Path:      fmt.Sprintf("locals_%d", i+1),
			Content:   localsContent,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata: map[string]interface{}{
				"block_type": "locals",
			},
		}
		localsChunk.ID = generateHCLChunkID(localsChunk)
		chunks = append(chunks, localsChunk)
	}

	return chunks
}

// extractTerraformBlocks extracts terraform configuration blocks from HCL code
func (p *HCLParser) extractTerraformBlocks(code string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}

	// Find all terraform block declarations
	terraformMatches := hclTerraformRegex.FindAllStringIndex(code, -1)

	for i, match := range terraformMatches {
		if len(match) < 2 {
			continue
		}

		// Find the terraform block content
		startPos := match[0]
		terraformContent, endPos := p.findBlockContent(code, startPos)

		// Find the line numbers
		startLine := countLinesUpTo(code, startPos) + 1
		endLine := countLinesUpTo(code, endPos) + 1

		// Create terraform block chunk
		terraformChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeBlock,
			Name:      fmt.Sprintf("terraform_%d", i+1),
			Path:      fmt.Sprintf("terraform_%d", i+1),
			Content:   terraformContent,
			Language:  chunking.LanguageHCL,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata: map[string]interface{}{
				"block_type": "terraform",
			},
		}
		terraformChunk.ID = generateHCLChunkID(terraformChunk)
		chunks = append(chunks, terraformChunk)
	}

	return chunks
}

// processDependencies analyzes chunks to identify dependencies between them
func (p *HCLParser) processDependencies(chunks []*chunking.CodeChunk) {
	// Map chunks by ID for quick lookup
	chunkMap := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}

	// Map chunks by name for dependency tracking
	chunksByName := make(map[string]*chunking.CodeChunk)
	chunksByPath := make(map[string]*chunking.CodeChunk)

	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeBlock {
			chunksByName[chunk.Name] = chunk

			// Add chunk to path map
			if resourceType, ok := chunk.Metadata["resource_type"].(string); ok {
				if resourceName, ok := chunk.Metadata["resource_name"].(string); ok {
					// Add Terraform reference paths: "${resource_type}.${resource_name}"
					refPath := fmt.Sprintf("%s.%s", resourceType, resourceName)
					chunksByPath[refPath] = chunk
				}
			}

			// Add other reference paths based on block type
			if strings.HasPrefix(chunk.Name, "var.") {
				varName := strings.TrimPrefix(chunk.Name, "var.")
				chunksByPath[fmt.Sprintf("var.%s", varName)] = chunk
			} else if strings.HasPrefix(chunk.Name, "data.") {
				parts := strings.Split(chunk.Name, ".")
				if len(parts) >= 3 {
					// Format: "data.TYPE.NAME"
					refPath := fmt.Sprintf("%s.%s", parts[1], parts[2])
					chunksByPath[refPath] = chunk
				}
			}
		}
	}

	// Regular expressions for finding Terraform references
	resourceRefRegex := regexp.MustCompile(`\$\{([a-z_]+\.[a-z_][a-z0-9_-]*\.[a-z_][a-z0-9_-]*(?:\.[a-z0-9_-]+)*)\}`)
	variableRefRegex := regexp.MustCompile(`var\.[a-z_][a-z0-9_-]*`)
	dataRefRegex := regexp.MustCompile(`data\.[a-z_][a-z0-9_-]*\.[a-z_][a-z0-9_-]*`)

	// Analyze dependencies
	for _, chunk := range chunks {
		// Skip file and comment chunks for dependency analysis
		if chunk.Type != chunking.ChunkTypeBlock {
			continue
		}

		// Find Terraform interpolation references: ${resource.name.attribute}
		resourceRefs := resourceRefRegex.FindAllStringSubmatch(chunk.Content, -1)
		for _, match := range resourceRefs {
			if len(match) > 1 {
				// Extract the resource reference
				refPath := match[1]
				// Find the first part of the reference (resource.name)
				parts := strings.Split(refPath, ".")
				if len(parts) >= 2 {
					resourcePath := strings.Join(parts[0:2], ".")

					// Check if there's a chunk with this reference
					if depChunk, ok := chunksByPath[resourcePath]; ok {
						if chunk.Dependencies == nil {
							chunk.Dependencies = []string{}
						}
						chunk.Dependencies = append(chunk.Dependencies, depChunk.ID)
					}
				}
			}
		}

		// Find variable references: var.name
		varRefs := variableRefRegex.FindAllString(chunk.Content, -1)
		for _, varRef := range varRefs {
			if depChunk, ok := chunksByName[varRef]; ok {
				if chunk.Dependencies == nil {
					chunk.Dependencies = []string{}
				}
				chunk.Dependencies = append(chunk.Dependencies, depChunk.ID)
			}
		}

		// Find data source references: data.type.name
		dataRefs := dataRefRegex.FindAllString(chunk.Content, -1)
		for _, dataRef := range dataRefs {
			if depChunk, ok := chunksByName[dataRef]; ok {
				if chunk.Dependencies == nil {
					chunk.Dependencies = []string{}
				}
				chunk.Dependencies = append(chunk.Dependencies, depChunk.ID)
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

// generateHCLChunkID generates a unique ID for an HCL chunk
func generateHCLChunkID(chunk *chunking.CodeChunk) string {
	// Combine type, name, path, and line numbers for a unique identifier
	idString := string(chunk.Type) + ":" + chunk.Path + ":" + strconv.Itoa(chunk.StartLine) + "-" + strconv.Itoa(chunk.EndLine)

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(idString))
	return hex.EncodeToString(hash[:])
}

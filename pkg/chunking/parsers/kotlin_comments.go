package parsers

import (
	"fmt"
	"strings"
	"regexp"

	"github.com/S-Corkum/devops-mcp/pkg/chunking"
)

// extractKDocs extracts KDoc comments from Kotlin code
func (p *KotlinParser) extractKDocs(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all KDoc comments
	kdocMatches := kotlinKDocRegex.FindAllStringIndex(code, -1)
	
	for i, match := range kdocMatches {
		if len(match) < 2 {
			continue
		}
		
		// Get the comment content
		commentContent := code[match[0]:match[1]]
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := getLineNumberFromPos(code, match[1]) + 1
		
		// Clean up the content to extract just the text, removing asterisks and markers
		commentText := commentContent
		commentText = strings.TrimPrefix(commentText, "/**")
		commentText = strings.TrimSuffix(commentText, "*/")
		
		// Clean up lines in block comments
		lines := strings.Split(commentText, "\n")
		for i, line := range lines {
			// Remove leading asterisks and whitespace
			lines[i] = regexp.MustCompile(`^\s*\*?\s*`).ReplaceAllString(line, "")
		}
		commentText = strings.Join(lines, "\n")
		commentText = strings.TrimSpace(commentText)
		
		// Parse KDoc tags if present
		tags := make(map[string][]string)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "@") {
				parts := strings.SplitN(line, " ", 2)
				tagName := parts[0][1:] // Remove @ symbol
				
				var tagValue string
				if len(parts) > 1 {
					tagValue = parts[1]
				}
				
				if existing, ok := tags[tagName]; ok {
					tags[tagName] = append(existing, tagValue)
				} else {
					tags[tagName] = []string{tagValue}
				}
			}
		}
		
		// Create comment metadata
		commentMetadata := map[string]interface{}{
			"type": "kdoc",
			"text": commentText,
		}
		
		// Add tags if they exist
		if len(tags) > 0 {
			commentMetadata["tags"] = tags
		}
		
		// Create KDoc comment chunk
		kdocChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("kdoc-%d", i+1),
			Path:      fmt.Sprintf("comment:%d", startLine),
			Content:   commentContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  commentMetadata,
		}
		kdocChunk.ID = generateKotlinChunkID(kdocChunk)
		chunks = append(chunks, kdocChunk)
	}
	
	return chunks
}

// extractComments extracts regular comments from Kotlin code
func (p *KotlinParser) extractComments(code string, lines []string, parentID string) []*chunking.CodeChunk {
	chunks := []*chunking.CodeChunk{}
	
	// Find all line comments
	lineCommentMatches := kotlinLineCommentRegex.FindAllStringIndex(code, -1)
	
	for i, match := range lineCommentMatches {
		if len(match) < 2 {
			continue
		}
		
		// Get the comment content
		commentContent := code[match[0]:match[1]]
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := getLineNumberFromPos(code, match[1]) + 1
		
		// Clean up the content
		commentText := strings.TrimPrefix(commentContent, "//")
		commentText = strings.TrimSpace(commentText)
		
		// Create comment metadata
		commentMetadata := map[string]interface{}{
			"type": "line_comment",
			"text": commentText,
		}
		
		// Create line comment chunk
		commentChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("comment-%d", i+1),
			Path:      fmt.Sprintf("comment:%d", startLine),
			Content:   commentContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  commentMetadata,
		}
		commentChunk.ID = generateKotlinChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}
	
	// Find all block comments that are not KDocs
	blockCommentMatches := kotlinBlockCommentRegex.FindAllStringIndex(code, -1)
	
	for i, match := range blockCommentMatches {
		if len(match) < 2 {
			continue
		}
		
		// Get the comment content
		commentContent := code[match[0]:match[1]]
		
		// Skip KDoc comments as they're handled separately
		if strings.HasPrefix(commentContent, "/**") {
			continue
		}
		
		// Find the line numbers
		startLine := getLineNumberFromPos(code, match[0]) + 1
		endLine := getLineNumberFromPos(code, match[1]) + 1
		
		// Clean up the content
		commentText := strings.TrimPrefix(commentContent, "/*")
		commentText = strings.TrimSuffix(commentText, "*/")
		commentText = strings.TrimSpace(commentText)
		
		// Create comment metadata
		commentMetadata := map[string]interface{}{
			"type": "block_comment",
			"text": commentText,
		}
		
		// Create block comment chunk
		commentChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      fmt.Sprintf("block-comment-%d", i+1),
			Path:      fmt.Sprintf("comment:%d", startLine),
			Content:   commentContent,
			Language:  chunking.LanguageKotlin,
			StartLine: startLine,
			EndLine:   endLine,
			ParentID:  parentID,
			Metadata:  commentMetadata,
		}
		commentChunk.ID = generateKotlinChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}
	
	return chunks
}
